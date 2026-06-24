package engine

import (
	"container/heap"
	"sync"

	"github.com/Im-Manav/ome/pkg/models"
	"github.com/google/uuid"
)

// Buy Heap (max-heap: highest price first, earliest time as tiebreaker)
type buyHeap []*models.Order

func (h buyHeap) Len() int      { return len(h) }
func (h buyHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h buyHeap) Less(i, j int) bool {
	if h[i].Price != h[j].Price {
		return h[i].Price > h[j].Price // Higher Price Wins
	}
	return h[i].CreatedAt.Before(h[j].CreatedAt) // Earlier time wins
}

func (h *buyHeap) Push(x any) {
	*h = append(*h, x.(*models.Order))
}

func (h *buyHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// ─── Sell heap (min-heap: lowest price first, earliest time as tiebreaker) ───

type sellHeap []*models.Order

func (h sellHeap) Len() int      { return len(h) }
func (h sellHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h sellHeap) Less(i, j int) bool {
	if h[i].Price != h[j].Price {
		return h[i].Price < h[j].Price // lower price wins
	}
	return h[i].CreatedAt.Before(h[j].CreatedAt) // earlier time wins
}
func (h *sellHeap) Push(x any) {
	*h = append(*h, x.(*models.Order))
}
func (h *sellHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// ─── OrderBook ───────────────────────────────────────────────────────────────

// OrderBook holds all resting orders for a single symbol.
// It is safe for concurrent reads but matching must happen
// on a single goroutine per symbol — enforced by the engine.

type OrderBook struct {
	symbol string
	buys   *buyHeap                    // max-heap of resting buy orders
	sells  *sellHeap                   // min-heap of resting sell orders
	index  map[uuid.UUID]*models.Order // O(1) lookup for cancellation
	mu     sync.RWMutex
}

func NewOrderBook(symbol string) *OrderBook {
	bh := &buyHeap{}
	sh := &sellHeap{}
	heap.Init(bh)
	heap.Init(sh)
	return &OrderBook{
		symbol: symbol,
		buys:   bh,
		sells:  sh,
		index:  make(map[uuid.UUID]*models.Order),
	}
}

// Add pushes a resting order into the appropriate heap.
// Call this only for orders that didn't fully match.
func (ob *OrderBook) Add(order *models.Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.index[order.ID] = order
	switch order.Side {
	case models.Buy:
		heap.Push(ob.buys, order)
	case models.Sell:
		heap.Push(ob.sells, order)
	}
}

// Cancel marks an order as cancelled and removes it from the index.
// The order stays in the heap until it surfaces at the top —
// the matcher checks status and skips cancelled orders (lazy deletion).
func (ob *OrderBook) Cancel(orderID uuid.UUID) (found bool, side models.Side) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	order, ok := ob.index[orderID]
	if !ok {
		return false, 0
	}
	order.Status = models.StatusCancelled
	delete(ob.index, orderID)
	return true, order.Side
}

// BestBid returns the highest resting buy order without removing it.
// Returns nil if no buy orders exist.
func (ob *OrderBook) BestBid() *models.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	// skip cancelled orders that are still in the heap
	for ob.buys.Len() > 0 {
		top := (*ob.buys)[0]
		if top.Status == models.StatusCancelled {
			ob.mu.RUnlock()
			ob.mu.Lock()
			heap.Pop(ob.buys)
			ob.mu.Unlock()
			ob.mu.RLock()
			continue
		}
		return top
	}
	return nil
}

// BestAsk returns the lowest resting sell order without removing it.
func (ob *OrderBook) BestAsk() *models.Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	for ob.sells.Len() > 0 {
		top := (*ob.sells)[0]
		if top.Status == models.StatusCancelled {
			ob.mu.RUnlock()
			ob.mu.Lock()
			heap.Pop(ob.sells)
			ob.mu.Unlock()
			ob.mu.RLock()
			continue
		}
		return top
	}
	return nil
}

// PopBestBid removes and returns the highest resting buy order.
func (ob *OrderBook) PopBestBid() *models.Order {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	for ob.buys.Len() > 0 {
		order := heap.Pop(ob.buys).(*models.Order)
		if order.Status == models.StatusCancelled {
			continue
		}
		delete(ob.index, order.ID)
		return order
	}
	return nil
}

// PopBestAsk removes and returns the lowest resting sell order.
func (ob *OrderBook) PopBestAsk() *models.Order {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	for ob.sells.Len() > 0 {
		order := heap.Pop(ob.sells).(*models.Order)
		if order.Status == models.StatusCancelled {
			continue
		}
		delete(ob.index, order.ID)
		return order
	}
	return nil
}

// Depth returns the top N price levels aggregated for display.
// Bids are sorted high→low, asks low→high.
func (ob *OrderBook) Depth(levels int) (bids, asks []models.OrderBookLevel) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	bids = aggregateLevels(ob.buys, levels, true)
	asks = aggregateLevels(ob.sells, levels, false)
	return
}

// Size returns the total number of active resting orders.
func (ob *OrderBook) Size() int {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return len(ob.index)
}

// aggregateLevels groups orders by price into OrderBookLevel entries.
// This is what the GET /orderbook endpoint returns.
func aggregateLevels(h any, levels int, _ bool) []models.OrderBookLevel {
	// We snapshot the heap slice - don't mutate it
	var orders []*models.Order
	switch v := any(h).(type) {
	case *buyHeap:
		orders = []*models.Order(*v)
	case *sellHeap:
		orders = []*models.Order(*v)
	default:
		return []models.OrderBookLevel{}
	}

	levelMap := make(map[float64]*models.OrderBookLevel)
	var levelOrder []float64

	for _, o := range orders {
		if o.Status == models.StatusCancelled {
			continue
		}
		if _, exists := levelMap[o.Price]; !exists {
			levelMap[o.Price] = &models.OrderBookLevel{Price: o.Price}
			levelOrder = append(levelOrder, o.Price)
		}
		levelMap[o.Price].Quantity += o.RemainingQty
		levelMap[o.Price].Orders++
	}

	// build result up to `levels` entries
	result := make([]models.OrderBookLevel, 0, levels)
	for i, price := range levelOrder {
		if i >= levels {
			break
		}
		result = append(result, *levelMap[price])
	}
	return result
}
