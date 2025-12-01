package models

type Trade struct {
	ID          string  `json:"id" gorm:"primaryKey;type:uuid"`
	BuyOrderID  string  `json:"buy_order_id"`
	SellOrderID string  `json:"sell_order_id"`
	Symbol      string  `json:"symbol"`
	Quantity    int64   `json:"quantity"`
	Price       float64 `json:"price"`
	Timestamp   int64   `json:"timestamp"`
	BuyUserID   string  `json:"buy_user_id"`
	SellUserID  string  `json:"sell_user_id"`
}
