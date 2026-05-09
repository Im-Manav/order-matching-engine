package ports

import (
	"github.com/Im-Manav/ome/pkg/models"
	"github.com/google/uuid"
)

// OrderBook is the in-memory data structure for one symbol.
// Implemented by engine.OrderBook using a heap.
type OrderBook interface {
	// Add inserts a new order into the book
	Add(order *models.Order)

	// Cancel removes an order by ID, returns whether it existed and which side
	Cancel(orderID uuid.UUID) (found bool, side models.Side)

	// BestBid returns the highest buy order without removing it
	BestBid() *models.Order

	// BestAsk returns the lowest sell order without removing it
	BestAsk() *models.Order

	// PopBestBid removes and returns the highest buy order
	PopBestBid() *models.Order

	// PopBestAsk removes and returns the lowest sell order
	PopBestAsk() *models.Order

	// Depth returns the top N price levels for bids and asks
	Depth(levels int) (bids, asks []models.OrderBookLevel)

	// Size returns total number of resting orders
	Size() int
}
