package engine

import (
	"testing"
	"time"

	"github.com/Im-Manav/ome/pkg/models"
	"github.com/google/uuid"
)

func newOrder(side models.Side, orderType models.OrderType, price, qty float64) *models.Order {
	return &models.Order{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		Symbol:       "BTC-USD",
		Side:         side,
		Type:         orderType,
		Price:        price,
		Quantity:     qty,
		RemainingQty: qty,
		Status:       models.StatusOpen,
		CreatedAt:    time.Now().UTC(),
	}
}

func TestFullMatch(t *testing.T) {
	m := NewMatcher()

	sell := newOrder(models.Sell, models.Limit, 100.0, 10.0)
	m.Match(sell) // rests in book — no buyers yet

	buy := newOrder(models.Buy, models.Limit, 100.0, 10.0)
	trades := m.Match(buy)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Quantity != 10.0 {
		t.Errorf("expected qty 10, got %f", trades[0].Quantity)
	}
	if buy.Status != models.StatusFilled {
		t.Errorf("expected buy to be filled, got %s", buy.Status)
	}
	if sell.Status != models.StatusFilled {
		t.Errorf("expected sell to be filled, got %s", sell.Status)
	}
}

func TestPartialMatch(t *testing.T) {
	m := NewMatcher()

	// Sell 10, buy only 6 → sell should be partially filled
	sell := newOrder(models.Sell, models.Limit, 100.0, 10.0)
	m.Match(sell)

	buy := newOrder(models.Buy, models.Limit, 100.0, 6.0)
	trades := m.Match(buy)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if sell.Status != models.StatusPartial {
		t.Errorf("expected sell to be partial, got %s", sell.Status)
	}
	if sell.RemainingQty != 4.0 {
		t.Errorf("expected sell remaining 4, got %f", sell.RemainingQty)
	}
	if buy.Status != models.StatusFilled {
		t.Errorf("expected buy to be filled, got %s", buy.Status)
	}
}

func TestNoMatchPriceMismatch(t *testing.T) {
	m := NewMatcher()

	// Seller wants $110, buyer only willing to pay $100 → no match
	sell := newOrder(models.Sell, models.Limit, 110.0, 5.0)
	m.Match(sell)

	buy := newOrder(models.Buy, models.Limit, 100.0, 5.0)
	trades := m.Match(buy)

	if len(trades) != 0 {
		t.Errorf("expected 0 trades, got %d", len(trades))
	}
	if buy.Status != models.StatusOpen {
		t.Errorf("expected buy to be open (resting), got %s", buy.Status)
	}
}

func TestPriceTimePriority(t *testing.T) {
	m := NewMatcher()

	// Two sells at the same price — earlier one should match first
	early := newOrder(models.Sell, models.Limit, 100.0, 5.0)
	early.CreatedAt = time.Now().Add(-time.Second)
	m.Match(early)

	late := newOrder(models.Sell, models.Limit, 100.0, 5.0)
	m.Match(late)

	buy := newOrder(models.Buy, models.Limit, 100.0, 5.0)
	trades := m.Match(buy)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].SellOrderID != early.ID {
		t.Errorf("expected early sell to match first")
	}
}

func TestMultipleTradesOneOrder(t *testing.T) {
	m := NewMatcher()

	// Three small sells, one big buy that sweeps all of them
	s1 := newOrder(models.Sell, models.Limit, 100.0, 3.0)
	s2 := newOrder(models.Sell, models.Limit, 101.0, 3.0)
	s3 := newOrder(models.Sell, models.Limit, 102.0, 3.0)
	m.Match(s1)
	m.Match(s2)
	m.Match(s3)

	buy := newOrder(models.Buy, models.Limit, 105.0, 9.0)
	trades := m.Match(buy)

	if len(trades) != 3 {
		t.Fatalf("expected 3 trades, got %d", len(trades))
	}
	if buy.Status != models.StatusFilled {
		t.Errorf("expected buy fully filled, got %s", buy.Status)
	}
}

func TestMarketOrderNotResting(t *testing.T) {
	m := NewMatcher()

	// Market buy with no sells — should be cancelled, not resting
	buy := newOrder(models.Buy, models.Market, 0, 5.0)
	trades := m.Match(buy)

	if len(trades) != 0 {
		t.Errorf("expected 0 trades, got %d", len(trades))
	}
	if buy.Status != models.StatusCancelled {
		t.Errorf("expected market order to be cancelled when unmatched, got %s", buy.Status)
	}
}

func TestCancelOrder(t *testing.T) {
	m := NewMatcher()
	book := m.getOrCreateBook("BTC-USD")

	sell := newOrder(models.Sell, models.Limit, 100.0, 10.0)
	m.Match(sell) // rests in book

	found, side := book.Cancel(sell.ID)
	if !found {
		t.Error("expected order to be found for cancellation")
	}
	if side != models.Sell {
		t.Errorf("expected sell side, got %v", side)
	}

	// Now a matching buy should produce no trades
	buy := newOrder(models.Buy, models.Limit, 100.0, 10.0)
	trades := m.Match(buy)

	if len(trades) != 0 {
		t.Errorf("expected 0 trades after cancel, got %d", len(trades))
	}
}
