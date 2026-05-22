package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Im-Manav/ome/pkg/logger"
	"github.com/Im-Manav/ome/pkg/models"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Producer wraps kafka-go writers — one writer per topic.
// Each writer is goroutine-safe; kafka-go handles batching internally.
type Producer struct {
	orders      *kafkago.Writer
	trades      *kafkago.Writer
	orderEvents *kafkago.Writer
	marketData  *kafkago.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		orders:      newWriter(brokers, TopicOrders),
		trades:      newWriter(brokers, TopicTrades),
		orderEvents: newWriter(brokers, TopicOrderEvents),
		marketData:  newWriter(brokers, TopicMarketData),
	}
}

// newWriter creates a kafka-go writer with sane production defaults.
func newWriter(brokers []string, topic string) *kafkago.Writer {
	return &kafkago.Writer{
		Addr:  kafkago.TCP(brokers...),
		Topic: topic,

		Balancer: &kafkago.Hash{},

		BatchSize:    100,
		BatchTimeout: 5 * time.Millisecond,

		MaxAttempts: 5,

		RequiredAcks: kafkago.RequireAll,

		AllowAutoTopicCreation: true,
	}
}

// PublishOrder publishes an incoming order to the orders topic.
// Key = symbol so all BTC-USD orders go to the same partition.
func (p *Producer) PublishOrder(ctx context.Context, order models.Order) error {
	return p.publish(ctx, p.orders, order.Symbol, order)
}

// PublishTrade publishes a matched trade to the trades topic.
func (p *Producer) PublishTrade(ctx context.Context, trade models.Trade) error {
	return p.publish(ctx, p.trades, trade.Symbol, trade)
}

// PublishTradeEvent publishes a trade event (with fill metadata) to trades topic.
func (p *Producer) PublishTradeEvent(ctx context.Context, event models.TradeEvent) error {
	return p.publish(ctx, p.trades, event.Symbol, event)
}

// PublishOrderEvent publishes an order status update (filled, cancelled, partial).
func (p *Producer) PublishOrderEvent(ctx context.Context, order models.Order) error {
	return p.publish(ctx, p.orderEvents, order.Symbol, order)
}

// publish is the shared internal writer — marshals to JSON and writes.
func (p *Producer) publish(
	ctx context.Context,
	writer *kafkago.Writer,
	key string,
	payload any,
) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kafka publish marshal: %w", err)
	}

	msg := kafkago.Message{
		Key:   []byte(key),
		Value: data,
		Time:  time.Now().UTC(),
	}

	if err := writer.WriteMessages(ctx, msg); err != nil {
		logger.Error("kafka publish failed",
			zap.String("topic", writer.Topic),
			zap.String("key", key),
			logger.Err(err),
		)
		return fmt.Errorf("kafka publish to %s: %w", writer.Topic, err)
	}

	logger.Info("kafka published",
		zap.String("topic", writer.Topic),
		zap.String("key", key),
	)
	return nil
}

// Close flushes and closes all writers. Call on shutdown.
func (p *Producer) Close() error {
	var errs []error
	for _, w := range []*kafkago.Writer{
		p.orders, p.trades, p.orderEvents, p.marketData,
	} {
		if err := w.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("kafka producer close errors: %v", errs)
	}
	return nil
}
