package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Im-Manav/ome/internal/engine"
	"github.com/Im-Manav/ome/pkg/logger"
	"github.com/Im-Manav/ome/pkg/models"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type OrderConsumer struct {
	reader   *kafkago.Reader
	matcher  *engine.Matcher
	producer *Producer
	handlers []PostMatchHandler
}

type PostMatchHandler func(ctx context.Context, order models.Order, trades []models.Trade) error

func NewOrderConsumer(
	brokers []string,
	groupID string,
	matcher *engine.Matcher,
	producer *Producer,
) *OrderConsumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        brokers,
		Topic:          TopicOrders,
		GroupID:        groupID,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		MaxWait:        100 * time.Millisecond,
		StartOffset:    kafkago.LastOffset,
		CommitInterval: 0,
	})

	return &OrderConsumer{
		reader:   reader,
		matcher:  matcher,
		producer: producer,
	}
}

func (c *OrderConsumer) AddHandler(h PostMatchHandler) {
	c.handlers = append(c.handlers, h)
}

func (c *OrderConsumer) Start(ctx context.Context) error {
	logger.Info("kafka order consumer started",
		zap.String("topic", TopicOrders),
	)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				logger.Info("kafka consumer shutting down")
				return nil
			}
			logger.Error("kafka fetch message failed", logger.Err(err))
			continue
		}

		if err := c.processMessage(ctx, msg); err != nil {
			logger.Error("kafka process message failed",
				logger.Err(err),
				zap.String("key", string(msg.Key)),
			)
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			logger.Error("kafka commit failed", logger.Err(err))
		}
	}
}

func (c *OrderConsumer) processMessage(ctx context.Context, msg kafkago.Message) error {
	var order models.Order
	if err := json.Unmarshal(msg.Value, &order); err != nil {
		return fmt.Errorf("unmarshal order: %w", err)
	}

	logger.Info("processing order",
		zap.String("order_id", order.ID.String()),
		zap.String("symbol", order.Symbol),
		zap.String("side", order.Side.String()),
		zap.Float64("price", order.Price),
		zap.Float64("quantity", order.Quantity),
	)

	trades := c.matcher.Match(&order)

	logger.Info("match complete",
		zap.String("order_id", order.ID.String()),
		zap.Int("trades_produced", len(trades)),
		zap.String("order_status", order.Status.String()),
	)

	for _, trade := range trades {
		event := models.TradeEvent{
			Trade:        trade,
			BuyerFilled:  isFilled(order, trade, models.Buy),
			SellerFilled: isFilled(order, trade, models.Sell),
		}
		if err := c.producer.PublishTradeEvent(ctx, event); err != nil {
			logger.Error("failed to publish trade event",
				logger.Err(err),
				zap.String("trade_id", trade.ID.String()),
			)
		}
	}

	if err := c.producer.PublishOrderEvent(ctx, order); err != nil {
		logger.Error("failed to publish order event", logger.Err(err))
	}

	for _, handler := range c.handlers {
		if err := handler(ctx, order, trades); err != nil {
			logger.Error("post-match handler failed", logger.Err(err))
		}
	}
	return nil
}

func isFilled(order models.Order, trade models.Trade, side models.Side) bool {
	if order.Side == side {
		return order.Status == models.StatusFilled
	}
	return false
}

func (c *OrderConsumer) Close() error {
	return c.reader.Close()
}

// ─── Trade Consumer ───────────────────────────────────────────────────────────

// TradeConsumer reads from the trades topic.
// Used by the market data service to build OHLCV candles.
type TradeConsumer struct {
	reader   *kafkago.Reader
	handlers []TradeHandler
}

type TradeHandler func(ctx context.Context, event models.TradeEvent) error

func NewTradeConsumer(brokers []string, groupID string) *TradeConsumer {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        brokers,
		Topic:          TopicTrades,
		GroupID:        groupID,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		MaxWait:        100 * time.Millisecond,
		StartOffset:    kafkago.LastOffset,
		CommitInterval: 0,
	})

	return &TradeConsumer{reader: reader}
}

func (c *TradeConsumer) AddHandler(h TradeHandler) {
	c.handlers = append(c.handlers, h)
}

func (c *TradeConsumer) Start(ctx context.Context) error {
	logger.Info("kafka trade consumer started",
		zap.String("topic", TopicTrades),
	)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			logger.Error("trade consumer fetch failed", logger.Err(err))
			continue
		}

		var event models.TradeEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			logger.Error("trade unmarshal failed", logger.Err(err))
			_ = c.reader.CommitMessages(ctx, msg)
			continue
		}

		for _, handler := range c.handlers {
			if err := handler(ctx, event); err != nil {
				logger.Error("trade handler failed", logger.Err(err))
			}
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			logger.Error("trade commit failed", logger.Err(err))
		}
	}
}

func (c *TradeConsumer) Close() error {
	return c.reader.Close()
}
