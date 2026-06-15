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
	"github.com/Im-Manav/ome/internal/kafka"
	"github.com/Im-Manav/ome/pkg/logger"
	"go.uber.org/zap"
)

// candleInterval is the bucket size for OHLCV candles.
// 1 minute is a good default — fine enough for a live chart,
// coarse enough to keep TimescaleDB rows manageable.
const candleInterval = time.Minute

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

	// Builder accumulates trades into in-memory candles per symbol
	// and flushes them to TimescaleDB + Redis pub/sub.
	builder := NewCandleBuilder(repo, redisClient)

	// Consume the trades topic
	consumer := kafka.NewTradeConsumer(cfg.KafkaBrokers, kafka.GroupMarketData)
	consumer.AddHandler(builder.HandleTrade)
	defer consumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := consumer.Start(ctx); err != nil {
			logger.Error("trade consumer stopped", logger.Err(err))
		}
	}()

	// Periodically flush in-progress candles even if no new trades arrive —
	// otherwise a quiet market would never close out the current candle.
	go builder.StartFlushLoop(ctx, candleInterval)

	logger.Info("market data service started")

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received")
	cancel()
	time.Sleep(500 * time.Millisecond)
	logger.Info("market data service stopped")

	_ = zap.L()
}
