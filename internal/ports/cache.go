package ports

import (
	"context"
	"time"

	"github.com/Im-Manav/ome/pkg/models"
)

// Cache defines Redis operations used across services
type Cache interface {
	// Order book snapshot — for fast GET /orderbook reads
	SetOrderBookSnapshot(ctx context.Context, symbol string, snap models.OrderBookSnapshot, ttl time.Duration) error
	GetOrderBookSnapshot(ctx context.Context, symbol string) (*models.OrderBookSnapshot, error)

	// Rate limiting - token bucket per user
	IncrWithExpiry(ctx context.Context, key string, expiry time.Duration) (int64, error)

	// Pub/Sub - for broadcasting trades to Websocket clients
	Publish(ctx context.Context, channel string, payload any) error
	Subscribe(ctx context.Context, channel string) (<-chan string, error)
	PublishOrderBookUpdate(ctx context.Context, snap models.OrderBookSnapshot) error

	// Session - JWT blocklist for logout
	SetWithExpiry(ctx context.Context, key, value string, expiry time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}
