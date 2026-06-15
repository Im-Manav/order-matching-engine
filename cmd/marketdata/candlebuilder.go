package main

import (
	"context"
	"sync"
	"time"

	"github.com/Im-Manav/ome/internal/db"
	"github.com/Im-Manav/ome/pkg/logger"
	"github.com/Im-Manav/ome/pkg/models"
	"go.uber.org/zap"
)

// CandleBuilder maintains one in-progress OHLCV candle per symbol.
// Each incoming trade updates the current candle. When the candle's
// time bucket closes, it's flushed to TimescaleDB and published to Redis.
//
// This is the classic "tumbling window" pattern from stream processing —
// implemented here with a plain map + mutex since throughput for a
// portfolio project doesn't need a dedicated stream processor.
type CandleBuilder struct {
	repo    *db.Repository
	pub     candlePublisher
	mu      sync.Mutex
	candles map[string]*models.OHLCV
}

type candlePublisher interface {
	Publish(ctx context.Context, channel string, payload any) error
}

func NewCandleBuilder(repo *db.Repository, pub candlePublisher) *CandleBuilder {
	return &CandleBuilder{
		repo:    repo,
		pub:     pub,
		candles: make(map[string]*models.OHLCV),
	}
}

// HandleTrade is called for every trade consumed from Kafka.
// It updates the in-progress candle for the trade's symbol.
func (b *CandleBuilder) HandleTrade(ctx context.Context, event models.TradeEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	bucketTime := event.ExecutedAt.Truncate(time.Minute)

	candle, exists := b.candles[event.Symbol]

	// New candle needed if: none exists yet, OR the trade belongs to a
	// new time bucket (previous candle should already have been flushed
	// by the flush loop, but handle it here too for safety).
	if !exists || !candle.Time.Equal(bucketTime) {
		if exists {
			b.flush(ctx, candle)
		}
		candle = &models.OHLCV{
			Time:   bucketTime,
			Symbol: event.Symbol,
			Open:   event.Price,
			High:   event.Price,
			Low:    event.Price,
			Close:  event.Price,
			Volume: 0,
		}
		b.candles[event.Symbol] = candle
	}

	if event.Price > candle.High {
		candle.High = event.Price
	}
	if event.Price < candle.Low {
		candle.Low = event.Price
	}
	candle.Close = event.Price
	candle.Volume += event.Quantity

	b.publishCandle(ctx, *candle)

	logger.Info("candle updated",
		zap.String("symbol", event.Symbol),
		zap.Time("bucket", bucketTime),
		zap.Float64("close", candle.Close),
		zap.Float64("volume", candle.Volume),
	)

	return nil
}

// StartFlushLoop periodically checks whether any in-progress candle's
// time bucket has closed and flushes it to TimescaleDB.
//
// Without this, a candle for a quiet symbol would sit in memory forever
// if no new trade ever arrives to trigger the "new bucket" check in HandleTrade.
func (b *CandleBuilder) StartFlushLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval / 4)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.flushExpired(ctx, interval)
		case <-ctx.Done():
			return
		}
	}
}

func (b *CandleBuilder) flushExpired(ctx context.Context, interval time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now().UTC()
	currentBucket := now.Truncate(interval)

	for symbol, candle := range b.candles {
		if candle.Time.Before(currentBucket) {
			b.flush(ctx, candle)
			delete(b.candles, symbol)
		}
	}
}

// flush persists a closed candle to TimescaleDB.
// Uses UpsertOHLCV so re-flushing the same candle (e.g. on restart) is safe.
func (b *CandleBuilder) flush(ctx context.Context, candle *models.OHLCV) {
	c := *candle
	if err := b.repo.UpsertOHLCV(&c); err != nil {
		logger.Error("failed to flush candle",
			logger.Err(err),
			zap.String("symbol", c.Symbol),
		)
		return
	}
	logger.Info("candle flushed to db",
		zap.String("symbol", c.Symbol),
		zap.Time("bucket", c.Time),
	)
}

// publishCandle sends the current candle state to Redis pub/sub
// so the WebSocket hub (subscribing separately) can forward it
// to dashboard clients for live chart updates.
func (b *CandleBuilder) publishCandle(ctx context.Context, candle models.OHLCV) {
	channel := "candles:" + candle.Symbol
	if err := b.pub.Publish(ctx, channel, candle); err != nil {
		logger.Error("failed to publish candle", logger.Err(err))
	}
}
