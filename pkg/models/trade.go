package models

import (
	"time"

	"github.com/google/uuid"
)

type Trade struct {
	ID          uuid.UUID `json:"id"            gorm:"type:uuid;primaryKey"`
	Symbol      string    `json:"symbol"        gorm:"not null;index"`
	BuyOrderID  uuid.UUID `json:"buy_order_id"  gorm:"type:uuid;not null;index"`
	SellOrderID uuid.UUID `json:"sell_order_id" gorm:"type:uuid;not null;index"`
	BuyUserID   uuid.UUID `json:"buy_user_id"   gorm:"type:uuid;not null"`
	SellUserID  uuid.UUID `json:"sell_user_id"  gorm:"type:uuid;not null"`
	Price       float64   `json:"price"         gorm:"not null"` // price at which trade executed
	Quantity    float64   `json:"quantity"      gorm:"not null"` // quantity that traded
	ExecutedAt  time.Time `json:"executed_at"   gorm:"index"`
}

// OHLCV is a candlestick bar — stored in TimescaleDB hypertable
type OHLCV struct {
	Time   time.Time `json:"time" gorm:"primaryKey;index"`
	Symbol string    `json:"symbol" gorm:"primaryKey;not null"`
	Open   float64   `json:"open"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Close  float64   `json:"close"`
	Volume float64   `json:"volume"`
}

// TradeEvent is published to Kafka and broadcast over WebSocket
// It's a superset of Trade with extra fields for clients
type TradeEvent struct {
	Trade
	BuyerFilled  bool `json:"buyer_filled"`  // was the buy order fully filled?
	SellerFilled bool `json:"seller_filled"` // was the sell order fully filled?
}
