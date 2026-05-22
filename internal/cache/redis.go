package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Im-Manav/ome/internal/config"
	"github.com/Im-Manav/ome/pkg/models"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(cfg *config.Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.RedisPassword,
		DB:       0,

		// Connection pool — same discipline as Postgres
		PoolSize:     20,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Verify connection at startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

// ─── Key helpers ─────────────────────────────────────────────────────────────
// All Redis keys go through these helpers — never hardcode key strings
// anywhere else in the codebase. One place to change, one place to read.

func keyOrderBookSnapshot(symbol string) string {
	return fmt.Sprintf("orderbook:snapshot:%s", symbol)
}

func keyRateLimit(userID string) string {
	return fmt.Sprintf("ratelimit:%s", userID)
}

func keyJWTBlocklist(tokenID string) string {
	return fmt.Sprintf("jwt:blocklist:%s", tokenID)
}

func channelTrades(symbol string) string {
	return fmt.Sprintf("trades:%s", symbol)
}

func channelOrderBook(symbol string) string {
	return fmt.Sprintf("orderbook:%s", symbol)
}

// ─── Order book snapshot ──────────────────────────────────────────────────────
// The matching engine writes a fresh snapshot after every match.
// The GET /orderbook/:symbol handler reads from here instead of
// rebuilding the snapshot from the heap on every request.

func (c *Client) SetOrderBookSnapshot(
	ctx context.Context,
	symbol string,
	snap models.OrderBookSnapshot,
	ttl time.Duration,
) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("SetOrderBookSnapshot marshal: %w", err)
	}
	return c.rdb.Set(ctx, keyOrderBookSnapshot(symbol), data, ttl).Err()
}

func (c *Client) GetOrderBookSnapshot(
	ctx context.Context,
	symbol string,
) (*models.OrderBookSnapshot, error) {
	data, err := c.rdb.Get(ctx, keyOrderBookSnapshot(symbol)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetOrderBookSnapshot: %w", err)
	}

	var snap models.OrderBookSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("GetOrderBookSnapshot unmarshal: %w", err)
	}
	return &snap, nil
}

// ─── Rate limiting ────────────────────────────────────────────────────────────
// Fixed window counter — increment a key, set expiry on first increment.
// The API gateway calls this before processing any order request.
//
// Usage: if count > limit, reject with 429.

func (c *Client) IncrWithExpiry(
	ctx context.Context,
	key string,
	expiry time.Duration,
) (int64, error) {
	// Pipeline both commands - one round trip instead of two
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, keyRateLimit(key))
	pipe.Expire(ctx, keyRateLimit(key), expiry)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("IncrWithExpiry: %w", err)
	}
	return incr.Val(), nil
}

// ─── Pub/Sub ──────────────────────────────────────────────────────────────────
// Publish sends a trade or order book event to all subscribers on a channel.
// The market data service and WebSocket hub both subscribe to these channels.

func (c *Client) Publish(ctx context.Context, channel string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Publish marshal: %w", err)
	}
	return c.rdb.Publish(ctx, channel, data).Err()
}

// PublishTrade publishes a trade event to the symbol-specific trades channel.
func (c *Client) PublishTrade(ctx context.Context, event models.TradeEvent) error {
	return c.Publish(ctx, channelTrades(event.Symbol), event)
}

// PublishOrderBookUpdate publishes an order book snapshot to subscribers.
func (c *Client) PublishOrderBookUpdate(
	ctx context.Context,
	snap models.OrderBookSnapshot,
) error {
	return c.Publish(ctx, channelOrderBook(snap.Symbol), snap)
}

// Subscribe returns a channel that streams raw JSON strings from a Redis
// pub/sub channel. The caller is responsible for unmarshalling.
// The goroutine exits cleanly when ctx is cancelled.
func (c *Client) Subscribe(
	ctx context.Context,
	channel string,
) (<-chan string, error) {
	sub := c.rdb.Subscribe(ctx, channel)

	// Verify the subscription was accepted
	if _, err := sub.Receive(ctx); err != nil {
		return nil, fmt.Errorf("Subscribe %s: %w", channel, err)
	}

	out := make(chan string, 64)

	go func() {
		defer close(out)
		defer sub.Close()

		ch := sub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				select {
				case out <- msg.Payload:
				default:
					// Subscriber is too slow — drop the message rather than
					// block the Redis reader. In production a Prometheus counter needs to be increment here
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

// SubscribeTrades subscribes to the trades channel for a symbol.
// Returns a channel of TradeEvent — already unmarshalled.
func (c *Client) SubscribeTrades(
	ctx context.Context,
	symbol string,
) (<-chan models.TradeEvent, error) {
	raw, err := c.Subscribe(ctx, channelTrades(symbol))
	if err != nil {
		return nil, err
	}

	out := make(chan models.TradeEvent, 64)

	go func() {
		defer close(out)
		for payload := range raw {
			var event models.TradeEvent
			if err := json.Unmarshal([]byte(payload), &event); err != nil {
				continue
			}
			out <- event
		}
	}()
	return out, nil
}

func (c *Client) SubscribeOrderBook(
	ctx context.Context,
	symbol string,
) (<-chan models.OrderBookSnapshot, error) {
	raw, err := c.Subscribe(ctx, channelOrderBook(symbol))
	if err != nil {
		return nil, err
	}

	out := make(chan models.OrderBookSnapshot, 64)

	go func() {
		defer close(out)
		for payload := range raw {
			var snap models.OrderBookSnapshot
			if err := json.Unmarshal([]byte(payload), &snap); err != nil {
				continue
			}
			out <- snap
		}
	}()

	return out, nil
}

// ─── JWT blocklist ────────────────────────────────────────────────────────────
// On logout, the token's JTI (JWT ID) is added here with TTL = token expiry.
// The auth middleware checks this before accepting any request.

func (c *Client) SetWithExpiry(
	ctx context.Context,
	key, value string,
	expiry time.Duration,
) error {
	return c.rdb.Set(ctx, keyJWTBlocklist(key), value, expiry).Err()
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, keyJWTBlocklist(key)).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (c *Client) Delete(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, keyJWTBlocklist(key)).Err()
}
