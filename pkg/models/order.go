package models

type Order struct {
	ID           string  `json:"id"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"` // BUY or SELL
	Quantity     int64   `json:"quantity"`
	Price        float64 `json:"price"`
	Type         string  `json:"type"`   // MARKET or LIMIT
	Status       string  `json:"status"` // OPEN, FILLED, CANCELLED
	FilledQty    int64   `json:"filled_qty"`
	RemainingQty int64   `json:"remaining_qty"`
	CreatedAt    int64   `json:"created_at"`
	UpdatedAt    int64   `json:"updated_at"`
	UserID       string  `json:"user_id"`
}
