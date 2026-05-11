package engine

import (
	"time"

	"github.com/Im-Manav/ome/pkg/models"
	"github.com/google/uuid"
)

// Matcher implements the price-time priority matching algorithm.
// It holds one OrderBook per symbol and is the only writer to them.
// IMPORTANT: Match() is not thread-safe by design — the engine runs
// one goroutine per symbol, so there is never concurrent access.
type Matcher struct {
	books map[string]*OrderBook // symbol -> order book
}

func NewMatcher() *Matcher {
	return &Matcher{
		books: make(map[string]*OrderBook),
	}
}

// getOrCreateBook returns the order book for a symbol,
// creating one if it doesn't exist yet.
func (m *Matcher) getOrCreateBook(symbol string) *OrderBook {
	if _, ok := m.books[symbol]; !ok {
		m.books[symbol] = NewOrderBook(symbol)
	}
	return m.books[symbol]
}

// Match is the core algorithm. It takes an incoming order and tries to
// match it against resting orders. Returns all trades produced.
//
// Flow:
//  1. If incoming is a BUY:  match against best asks (lowest sell first)
//     A match fires when ask.Price <= buy.Price
//  2. If incoming is a SELL: match against best bids (highest buy first)
//     A match fires when bid.Price >= sell.Price
//  3. Each match produces one Trade and reduces RemainingQty on both sides
//  4. Fully filled resting orders are removed from the book
//  5. Partially filled resting orders stay in the book with updated qty
//  6. Whatever remains of the incoming order rests in the book
func (m *Matcher) Match(order *models.Order) []models.Trade {
	book := m.getOrCreateBook(order.Symbol)
	var trades []models.Trade

	switch order.Side {
	case models.Buy:
		trades = m.matchBuy(order, book)
	case models.Sell:
		trades = m.matchSell(order, book)
	}

	// If the incoming order is still not fully filled, it rests in the book
	if order.RemainingQty > 0 && order.Status != models.StatusCancelled {
		// Market orders that can't fully fill are rejected — never rest
		if order.Type == models.Market {
			order.Status = models.StatusCancelled
		} else {
			book.Add(order)
		}
	}
	return trades
}

// matchBuy matches an incoming buy order against resting sell orders.
func (m *Matcher) matchBuy(buy *models.Order, book *OrderBook) []models.Trade {
	var trades []models.Trade

	for buy.RemainingQty > 0 {
		ask := book.BestAsk()

		if ask == nil || ask.Price > buy.Price {
			break
		}

		// Pop the ask off the book — we're going to fill it (fully or partially)
		ask = book.PopBestAsk()
		if ask == nil {
			break
		}

		trade := executeTrade(buy, ask)
		trades = append(trades, trade)

		if ask.RemainingQty > 0 {
			book.Add(ask)
		}
	}
	return trades
}

// matchSell matches an incoming sell order against resting buy orders.
func (m *Matcher) matchSell(sell *models.Order, book *OrderBook) []models.Trade {
	var trades []models.Trade

	for sell.RemainingQty > 0 {
		bid := book.BestBid()

		if bid == nil || bid.Price < sell.Price {
			break
		}

		bid = book.PopBestBid()
		if bid == nil {
			break
		}

		trade := executeTrade(bid, sell)
		trades = append(trades, trade)

		if bid.RemainingQty > 0 {
			book.Add(bid)
		}
	}
	return trades
}

// executeTrade creates a trade between a buy and sell order.
// It mutates both orders' FilledQty, RemainingQty, and Status.
// Trade price is always the resting order's price (maker price).
func executeTrade(buy, sell *models.Order) models.Trade {
	qty := min(buy.RemainingQty, sell.RemainingQty)
	tradePrice := sell.Price
	if buy.CreatedAt.Before(sell.CreatedAt) {
		tradePrice = buy.Price
	}

	applyFill(buy, qty)
	applyFill(sell, qty)

	return models.Trade{
		ID:          uuid.New(),
		Symbol:      buy.Symbol,
		BuyOrderID:  buy.ID,
		SellOrderID: sell.ID,
		BuyUserID:   buy.UserID,
		SellUserID:  sell.UserID,
		Price:       tradePrice,
		Quantity:    qty,
		ExecutedAt:  time.Now().UTC(),
	}
}

func applyFill(order *models.Order, qty float64) {
	order.FilledQty += qty
	order.RemainingQty -= qty

	switch {
	case order.RemainingQty <= 0:
		order.RemainingQty = 0
		order.Status = models.StatusFilled
	case order.RemainingQty > 0:
		order.Status = models.StatusPartial
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
