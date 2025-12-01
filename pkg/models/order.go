package models

import "time"

type Order struct {
	ID           string    `json:"id" gorm:"primaryKey;type:uuid"`
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"` // BUY or SELL
	Quantity     int64     `json:"quantity"`
	Price        float64   `json:"price"`
	Type         string    `json:"type"`   // MARKET or LIMIT
	Status       string    `json:"status"` // OPEN, FILLED, CANCELLED
	FilledQty    int64     `json:"filled_qty"`
	RemainingQty int64     `json:"remaining_qty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	UserID       string    `json:"user_id"`
}
