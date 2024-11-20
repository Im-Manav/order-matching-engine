package engine

import (
	"encoding/json"

	"github.com/shopspring/decimal"
)

type Order struct {
	ID        string          `json:"id"`
	Side      Side            `json:"side"`
	Quantity  decimal.Decimal `json:"quantity"`
	Price     decimal.Decimal `json:"price"`
	Timestamp int64           `json:"timestamp"`
}

func (order *Order) FromJSON(msg []byte) error {
	return json.Unmarshal(msg, order)
}

func (order *Order) toJSON() []byte {
	str, _ := json.Marshal(order)
	return str
}
