package models

import (
	"time"

	"github.com/google/uuid"
)

type Side int8

const (
	Buy  Side = 0
	Sell Side = 1
)

func (s Side) String() string {
	switch s {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	default:
		return "UNKNOWN"
	}
}

type OrderType int8

const (
	Limit  OrderType = 0
	Market OrderType = 1
)

func (t OrderType) String() string {
	switch t {
	case Limit:
		return "LIMIT"
	case Market:
		return "MARKET"
	default:
		return "UNKNOWN"
	}
}

// OrderStatus — lifecycle of an order
type OrderStatus int8

const (
	StatusOpen      OrderStatus = 0
	StatusPartial   OrderStatus = 1
	StatusFilled    OrderStatus = 2
	StatusCancelled OrderStatus = 3
	StatusRejected  OrderStatus = 4
)

func (s OrderStatus) String() string {
	switch s {
	case StatusOpen:
		return "OPEN"
	case StatusPartial:
		return "PARTIAL"
	case StatusFilled:
		return "FILLED"
	case StatusCancelled:
		return "CANCELLED"
	case StatusRejected:
		return "REJECTED"
	default:
		return "UNKNOWN"
	}
}

// NOTE: For quantity float64 is used for simplicity. Production systems should use
// fixed-point arithmetic or a decimal library (shopspring/decimal)
// to avoid IEEE 754 rounding errors in price/quantity comparisons.

// Order is the core domain entity
type Order struct {
	ID           uuid.UUID   `json:"id" gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID   `json:"user_id"       gorm:"type:uuid;not null;index"`
	Symbol       string      `json:"symbol"        gorm:"not null;index"`
	Side         Side        `json:"side"          gorm:"not null"`
	Type         OrderType   `json:"type"          gorm:"not null"`
	Price        float64     `json:"price"         gorm:"not null"`  // 0 for market orders
	Quantity     float64     `json:"quantity"      gorm:"not null"`  // original quantity
	FilledQty    float64     `json:"filled_qty"    gorm:"default:0"` // how much has been matched
	RemainingQty float64     `json:"remaining_qty" gorm:"not null"`  // quantity left to fill
	Status       OrderStatus `json:"status"        gorm:"default:0;index"`
	CreatedAt    time.Time   `json:"created_at"    gorm:"index"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// PlaceOrderRequest is what the API receives from clients
type PlaceOrderRequest struct {
	Symbol   string    `json:"symbol" binding:"required"`
	Side     Side      `json:"side"     binding:"oneof=0 1"`
	Type     OrderType `json:"type"     binding:"oneof=0 1"`
	Price    float64   `json:"price"` // validated in service layer
	Quantity float64   `json:"quantity" binding:"required,gt=0"`
}

// PlaceOrderResponse is what the API returns after matching
type PlaceOrderResponse struct {
	Order  Order   `json:"order"`
	Trades []Trade `json:"trades"`
}

// CancelOrderResponse confirms a cancellation
type CancelOrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

// OrderBookLevel represents one price level in the order book depth
type OrderBookLevel struct {
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
	Orders   int     `json:"orders"` // number of orders at this level
}

// OrderBookSnapshot is what the GET /orderbook/:symbol endpoint returns
type OrderBookSnapshot struct {
	Symbol    string           `json:"symbol"`
	Bids      []OrderBookLevel `json:"bids"` // sorted high → low
	Asks      []OrderBookLevel `json:"asks"` // sorted low → high
	Timestamp time.Time        `json:"timestamp"`
}
