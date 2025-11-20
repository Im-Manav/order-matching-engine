package engine

import (
	"testing"

	"github.com/Im-Manav/order-matching-engine/pkg/models"
)

func TestPerfectMatch(t *testing.T) {
	ob := NewOrderBook()

	buy := &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 10}
	sell := &models.Order{ID: "s1", Side: "SELL", Price: 100, Quantity: 10}

	ob.AddOrder(sell)
	trades, _ := Match(buy, ob)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Quantity != 10 {
		t.Errorf("expected qty 10, got %d", trades[0].Quantity)
	}
}
func TestPartialFill_BuyLarger(t *testing.T) {
	ob := NewOrderBook()

	sell := &models.Order{ID: "s1", Side: "SELL", Price: 100, Quantity: 5}
	buy := &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 10}

	ob.AddOrder(sell)
	trades, _ := Match(buy, ob)

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade")
	}
	if trades[0].Quantity != 5 {
		t.Errorf("expected qty 5")
	}

	remaining := ob.buyOrders[100][0]
	if remaining.Quantity != 5 {
		t.Errorf("expected remaining buy qty 5, got %d", remaining.Quantity)
	}
}

func TestPartialFill_SellLarger(t *testing.T) {
	ob := NewOrderBook()

	buy := &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 5}
	sell := &models.Order{ID: "s1", Side: "SELL", Price: 100, Quantity: 10}

	ob.AddOrder(buy)
	trades, _ := Match(sell, ob)

	if trades[0].Quantity != 5 {
		t.Errorf("expected qty 5")
	}

	// sell should remain with 5 qty
	remaining := ob.sellOrders[100][0]
	if remaining.Quantity != 5 {
		t.Errorf("expected remaining sell qty 5")
	}
}

func TestNoMatch(t *testing.T) {
	ob := NewOrderBook()

	ob.AddOrder(&models.Order{ID: "s1", Side: "SELL", Price: 105, Quantity: 10})
	buy := &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 10}

	trades, _ := Match(buy, ob)

	if len(trades) != 0 {
		t.Errorf("expected no trades")
	}

	// buy must be placed in book
	if ob.buyOrders[100][0].ID != "b1" {
		t.Errorf("buy order should rest in book")
	}
}

func TestMultiLevelMatch(t *testing.T) {
	ob := NewOrderBook()

	ob.AddOrder(&models.Order{ID: "s1", Side: "SELL", Price: 95, Quantity: 3})
	ob.AddOrder(&models.Order{ID: "s2", Side: "SELL", Price: 100, Quantity: 4})

	buy := &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 10}

	trades, _ := Match(buy, ob)

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades")
	}

	if trades[0].Quantity != 3 || trades[1].Quantity != 4 {
		t.Errorf("quantities incorrect")
	}

	// leftover buy qty = 10 - 3 - 4 = 3
	if ob.buyOrders[100][0].Quantity != 3 {
		t.Errorf("expected remaining 3")
	}
}

func TestChoosesBestAsk(t *testing.T) {
	ob := NewOrderBook()

	ob.AddOrder(&models.Order{ID: "s2", Side: "SELL", Price: 102, Quantity: 5})
	ob.AddOrder(&models.Order{ID: "s1", Side: "SELL", Price: 100, Quantity: 5}) // best ask

	buy := &models.Order{ID: "b1", Side: "BUY", Price: 105, Quantity: 3}

	trades, _ := Match(buy, ob)

	if trades[0].Price != 100 {
		t.Errorf("trade should happen at best ask 100")
	}
}

func TestFIFO(t *testing.T) {
	ob := NewOrderBook()

	ob.AddOrder(&models.Order{ID: "s1", Side: "SELL", Price: 100, Quantity: 2})
	ob.AddOrder(&models.Order{ID: "s2", Side: "SELL", Price: 100, Quantity: 3})

	buy := &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 4}

	trades, _ := Match(buy, ob)

	if trades[0].SellOrderID != "s1" {
		t.Errorf("expected s1 first due to FIFO")
	}
	if trades[1].SellOrderID != "s2" {
		t.Errorf("expected s2 second due to FIFO")
	}
}
