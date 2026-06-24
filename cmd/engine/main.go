package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Im-Manav/ome/internal/cache"
	"github.com/Im-Manav/ome/internal/config"
	"github.com/Im-Manav/ome/internal/db"
	"github.com/Im-Manav/ome/internal/engine"
	"github.com/Im-Manav/ome/internal/kafka"
	"github.com/Im-Manav/ome/internal/service"
	"github.com/Im-Manav/ome/pkg/logger"
	"github.com/Im-Manav/ome/pkg/models"
	"go.uber.org/zap"
)

type noopBroadcaster struct{}

func (noopBroadcaster) BroadcastTrade(event models.TradeEvent)                 {}
func (noopBroadcaster) BroadcastOrderBookUpdate(snap models.OrderBookSnapshot) {}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic("failed to load config: " + err.Error())
	}

	if err := logger.Init(cfg.Env); err != nil {
		panic("failed to init logger: " + err.Error())
	}

	defer logger.Sync()

	database, err := db.NewConnection(cfg)
	if err != nil {
		logger.Fatal("postgres connection failed", logger.Err(err))
	}
	logger.Info("postgres connected")

	redisClient, err := cache.NewClient(cfg)
	if err != nil {
		logger.Fatal("redis connection failed", logger.Err(err))
	}
	defer redisClient.Close()
	logger.Info("redis connected")

	repo := db.NewRepository(database)

	producer := kafka.NewProducer(cfg.KafkaBrokers)
	defer producer.Close()

	// The matching engine itself — pure, in-memory, one instance
	// holding the order book heaps for every symbol.
	matcher := engine.NewMatcher()

	// OrderService gives us the PostMatchHandler — persists trades,
	// updates order status, broadcasts. We pass nil for Broadcaster
	// here since this binary doesn't own WebSocket clients — the
	// gateway does. Trade broadcast to WebSocket happens via the
	// gateway subscribing to Redis pub/sub (already published below).
	orderSvc := service.NewOrderService(repo, repo, producer, redisClient, noopBroadcaster{})

	consumer := kafka.NewOrderConsumer(
		cfg.KafkaBrokers,
		kafka.GroupEngine,
		matcher,
		producer,
	)
	defer consumer.Close()

	// Register the post-match handler: persist trades, update order,
	// then publish to Redis so the gateway's WebSocket hub can forward it.
	consumer.AddHandler(orderSvc.PostMatchHandler)

	consumer.AddHandler(func(ctx context.Context, order models.Order, trades []models.Trade) error {
		book := matcher.BookFor(order.Symbol)
		bids, asks := book.Depth(20)
		logger.Info("snapshot buil")
		snap := models.OrderBookSnapshot{
			Symbol:    order.Symbol,
			Bids:      bids,
			Asks:      asks,
			Timestamp: time.Now().UTC(),
		}
		if err := redisClient.SetOrderBookSnapshot(ctx, order.Symbol, snap, 30*time.Second); err != nil {
			logger.Error("failed to cache orderbook snapshot", logger.Err(err))
		}
		return redisClient.PublishOrderBookUpdate(ctx, snap)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := consumer.Start(ctx); err != nil {
			logger.Error("order consumer stopped", logger.Err(err))
		}
	}()

	logger.Info("matching engine service started", zap.String("group", kafka.GroupEngine))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received")
	cancel()
	time.Sleep(500 * time.Millisecond)
	logger.Info("matching engine service stopped")

}
