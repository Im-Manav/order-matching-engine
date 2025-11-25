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

	sell := &models.Order{
		ID:       "s1",
		Side:     "SELL",
		Price:    100,
		Quantity: 6,
	}
	ob.AddOrder(sell)

	buy := &models.Order{
		ID:       "b1",
		Side:     "BUY",
		Price:    100,
		Quantity: 10,
	}

	trades, err := Match(buy, ob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Quantity != 6 {
		t.Errorf("expected qty 6, got %d", trades[0].Quantity)
	}

	if buy.Status != "PARTIAL" {
		t.Errorf("expected buy to be PARTIAL, got %s", buy.Status)
	}
	if sell.Status != "FILLED" {
		t.Errorf("expected sell to be FILLED, got %s", sell.Status)
	}

	// BUY should remain in orderbook with qty 4
	bestBid := ob.GetBestBid()
	if bestBid == nil || bestBid.Quantity != 4 {
		t.Errorf("expected remaining BUY qty 4, got %+v", bestBid)
	}
}

func TestPartialFill_SellLarger(t *testing.T) {
	ob := NewOrderBook()

	buy := &models.Order{
		ID:       "b1",
		Side:     "BUY",
		Price:    100,
		Quantity: 7,
	}
	ob.AddOrder(buy)

	sell := &models.Order{
		ID:       "s1",
		Side:     "SELL",
		Price:    100,
		Quantity: 12,
	}

	trades, err := Match(sell, ob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Quantity != 7 {
		t.Errorf("expected qty 7, got %d", trades[0].Quantity)
	}

	if sell.Status != "PARTIAL" {
		t.Errorf("expected sell to be PARTIAL, got %s", sell.Status)
	}
	if buy.Status != "FILLED" {
		t.Errorf("expected buy to be FILLED, got %s", buy.Status)
	}

	// SELL should remain with 5 qty
	bestAsk := ob.GetBestAsk()
	if bestAsk == nil || bestAsk.Quantity != 5 {
		t.Errorf("expected remaining SELL qty 5, got %+v", bestAsk)
	}
}

func TestPartialFill_MultiLevel(t *testing.T) {
	ob := NewOrderBook()

	ob.AddOrder(&models.Order{ID: "s1", Side: "SELL", Price: 100, Quantity: 3})
	ob.AddOrder(&models.Order{ID: "s2", Side: "SELL", Price: 101, Quantity: 4})

	buy := &models.Order{
		ID:       "b1",
		Side:     "BUY",
		Price:    105,
		Quantity: 10,
	}

	trades, err := Match(buy, ob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(trades))
	}

	if buy.Status != "PARTIAL" {
		t.Errorf("expected buy to be PARTIAL, got %s", buy.Status)
	}

	if buy.Quantity != 3 {
		t.Errorf("expected remaining qty 3, got %d", buy.Quantity)
	}

	if ob.GetBestAsk() != nil {
		t.Errorf("expected no remaining asks")
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

func TestNoMatch_BuyTooLow(t *testing.T) {
	ob := NewOrderBook()

	ob.AddOrder(&models.Order{
		ID: "s1", Side: "SELL", Price: 100, Quantity: 5,
	})

	buy := &models.Order{
		ID: "b1", Side: "BUY", Price: 95, Quantity: 5,
	}

	trades, err := Match(buy, ob)
	if err != nil {
		t.Fatalf("unexpected error")
	}

	if len(trades) != 0 {
		t.Fatalf("expected no trades")
	}

	if buy.Status != "OPEN" {
		t.Errorf("expected OPEN, got %s", buy.Status)
	}

	// should have been added to orderbook
	if ob.GetBestBid().ID != "b1" {
		t.Errorf("order not added to book")
	}
}

func TestNoMatch_EmptyBook(t *testing.T) {
	ob := NewOrderBook()

	buy := &models.Order{
		ID: "b1", Side: "BUY", Price: 100, Quantity: 10,
	}

	trades, err := Match(buy, ob)
	if err != nil {
		t.Fatalf("unexpected error")
	}

	if len(trades) != 0 {
		t.Fatalf("expected no trades")
	}

	if ob.GetBestBid().ID != "b1" {
		t.Errorf("order not added to empty book")
	}
}

func TestExactMultiMatch(t *testing.T) {
	ob := NewOrderBook()

	ob.AddOrder(&models.Order{ID: "s1", Side: "SELL", Price: 100, Quantity: 3, Status: "OPEN"})
	ob.AddOrder(&models.Order{ID: "s2", Side: "SELL", Price: 100, Quantity: 6, Status: "OPEN"})

	buy := &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 9, Status: "OPEN"}

	trades, _ := Match(buy, ob)

	if len(trades) != 2 {
		t.Fatalf("expected 2 trades")
	}

	if ob.GetBestAsk() != nil {
		t.Errorf("orderbook should be empty on sell side")
	}
}

func TestSkipEmptyPriceLevel(t *testing.T) {
	ob := NewOrderBook()

	ob.buyOrders[100] = []*models.Order{} // empty slice

	if ob.GetBestBid() != nil {
		t.Errorf("empty price level should not be best bid")
	}
}

func TestInvalidSide(t *testing.T) {
	ob := NewOrderBook()

	order := &models.Order{
		ID: "x1", Side: "HOLD", Price: 100, Quantity: 5,
	}
	_, err := Match(order, ob)
	if err == nil {
		t.Fatalf("expected error for invalid side")
	}
}

// Table Driven Scenarios

func TestMatching_Scenarios(t *testing.T) {
	tests := []struct {
		name       string
		initial    []*models.Order
		incoming   *models.Order
		wantTrades int
	}{
		{
			name: "No match",
			initial: []*models.Order{
				{ID: "s1", Side: "SELL", Price: 100, Quantity: 5},
			},
			incoming:   &models.Order{ID: "b1", Side: "BUY", Price: 95, Quantity: 5},
			wantTrades: 0,
		},
		{
			name: "Perfect match",
			initial: []*models.Order{
				{ID: "s1", Side: "SELL", Price: 100, Quantity: 10},
			},
			incoming:   &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 10},
			wantTrades: 1,
		},
		{
			name: "Partial fill",
			initial: []*models.Order{
				{ID: "s1", Side: "SELL", Price: 100, Quantity: 5},
			},
			incoming:   &models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 10},
			wantTrades: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ob := NewOrderBook()

			for _, o := range tc.initial {
				ob.AddOrder(o)
			}

			trades, _ := Match(tc.incoming, ob)

			if len(trades) != tc.wantTrades {
				t.Errorf("expected %d trades, got %d", tc.wantTrades, len(trades))
			}
		})
	}
}

func TestPrintBook(t *testing.T) {
	ob := NewOrderBook()

	ob.AddOrder(&models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 5})
	ob.AddOrder(&models.Order{ID: "b2", Side: "BUY", Price: 101, Quantity: 10})
	ob.AddOrder(&models.Order{ID: "s1", Side: "SELL", Price: 102, Quantity: 3})
	ob.AddOrder(&models.Order{ID: "s2", Side: "SELL", Price: 103, Quantity: 6})

	// Just call it, depth > number of prices
	ob.Printbook(5)

	// depth < number of prices
	ob.Printbook(1)
}

func TestTotalQuantity(t *testing.T) {
	orders := []*models.Order{
		{ID: "o1", Quantity: 3},
		{ID: "o2", Quantity: 7},
	}
	if totalQuantity(orders) != 10 {
		t.Errorf("expected total 10")
	}

	// empty slice
	if totalQuantity([]*models.Order{}) != 0 {
		t.Errorf("expected total 0 for empty slice")
	}
}

func TestRemoveOrder(t *testing.T) {
	ob := NewOrderBook()
	ob.AddOrder(&models.Order{ID: "b1", Side: "BUY", Price: 100, Quantity: 5})

	// remove existing order
	ok, err := removeFromMap(ob.buyOrders, "b1")
	if !ok || err != nil {
		t.Errorf("expected removal success")
	}

	// remove again (non-existent)
	ok, err = removeFromMap(ob.buyOrders, "b1")
	if ok || err == nil {
		t.Errorf("expected removal failure")
	}
}

func TestGetBestAsk_EmptyOrNil(t *testing.T) {
	ob := NewOrderBook()

	// empty sellOrders
	if ob.GetBestAsk() != nil {
		t.Errorf("expected nil")
	}

	// with empty slice
	ob.sellOrders[100] = []*models.Order{}
	if ob.GetBestAsk() != nil {
		t.Errorf("expected nil")
	}
}

func TestCreatePriceList_EmptySlices(t *testing.T) {
	m := map[float64][]*models.Order{
		100: {},
		101: {{ID: "o1"}},
	}
	prices := createPriceList(m)
	if len(prices) != 1 || prices[0] != 101 {
		t.Errorf("expected only price 101")
	}
}
