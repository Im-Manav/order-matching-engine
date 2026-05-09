package ports

import (
	"context"

	"github.com/Im-Manav/ome/pkg/models"
)

type EventPublisher interface {
	PublishOrder(ctx context.Context, order models.Order) error
	PublishTrade(ctx context.Context, trade models.Trade) error
	PublishTradeEvent(ctx context.Context, event models.TradeEvent) error
	Close() error
}
