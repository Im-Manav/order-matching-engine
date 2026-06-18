package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Im-Manav/ome/internal/ai"
	"github.com/Im-Manav/ome/internal/cache"
	"github.com/Im-Manav/ome/internal/config"
	"github.com/Im-Manav/ome/internal/db"
	"github.com/Im-Manav/ome/pkg/logger"
	"go.uber.org/zap"
)

// symbols to generate predictions for — matches the symbols
// the market simulator will trade.
var symbols = []string{"BTC-USD", "ETH-USD", "AAPL", "TSLA"}

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
	repo := db.NewRepository(database)

	redisClient, err := cache.NewClient(cfg)
	if err != nil {
		logger.Fatal("redis connection failed", logger.Err(err))
	}
	defer redisClient.Close()

	aiClient := ai.NewClient(cfg.AIBaseURL, cfg.AIModel, cfg.AIAPIKey)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	interval := time.Duration(cfg.PredictionInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("predictor started",
		zap.Strings("symbols", symbols),
		zap.Duration("interval", interval),
		zap.String("provider", cfg.AIProvider),
	)

	// Run once immediately on startup, then on each tick
	runPredictions(ctx, repo, redisClient, aiClient)

	go func() {
		for {
			select {
			case <-ticker.C:
				runPredictions(ctx, repo, redisClient, aiClient)
			case <-ctx.Done():
				return
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received")
	cancel()
	time.Sleep(500 * time.Millisecond)
	logger.Info("predictor stopped")
}

// runPredictions generates and broadcasts a prediction for each symbol.
// Failures for one symbol don't block the others — each is independent.
func runPredictions(
	ctx context.Context,
	repo *db.Repository,
	redisClient *cache.Client,
	aiClient *ai.Client,
) {
	for _, symbol := range symbols {
		predictForSymbol(ctx, symbol, repo, redisClient, aiClient)
	}
}

func predictForSymbol(
	ctx context.Context,
	symbol string,
	repo *db.Repository,
	redisClient *cache.Client,
	aiClient *ai.Client,
) {
	candles, err := repo.GetOHLCV(symbol, "1 minute", 20)
	if err != nil {
		logger.Error("failed to fetch candles for prediction",
			logger.Err(err), zap.String("symbol", symbol))
		return
	}

	if len(candles) < 3 {
		// Not enough data yet — common at startup before the
		// market simulator has been running for a few minutes
		logger.Info("skipping prediction, insufficient data",
			zap.String("symbol", symbol), zap.Int("candles", len(candles)))
		return
	}

	// Get current order book snapshot for bid/ask context
	snap, err := redisClient.GetOrderBookSnapshot(ctx, symbol)
	var bestBid, bestAsk float64
	if snap != nil {
		if len(snap.Bids) > 0 {
			bestBid = snap.Bids[0].Price
		}
		if len(snap.Asks) > 0 {
			bestAsk = snap.Asks[0].Price
		}
	}

	prediction, err := aiClient.PredictPrice(ctx, symbol, candles, bestBid, bestAsk)
	if err != nil {
		logger.Error("prediction failed",
			logger.Err(err), zap.String("symbol", symbol))
		return
	}

	// Broadcast via Redis pub/sub — the gateway's WebSocket hub
	// will need a small addition (Step 9/10) to forward this channel
	channel := "predictions:" + symbol
	if err := redisClient.Publish(ctx, channel, prediction); err != nil {
		logger.Error("failed to publish prediction", logger.Err(err))
	}
}
