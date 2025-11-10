package engine

import (
	"errors"
	"fmt"

	"github.com/Im-Manav/order-matching-engine/pkg/models"
)

type OrderBook struct {
	buyOrders  map[float64][]*models.Order
	sellOrders map[float64][]*models.Order
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		buyOrders:  make(map[float64][]*models.Order),
		sellOrders: make(map[float64][]*models.Order),
	}
}

func (ob *OrderBook) AddOrder(order *models.Order) {
	if order.Side == "BUY" {
		ob.buyOrders[order.Price] = append(ob.buyOrders[order.Price], order)
	}
	if order.Side == "SELL" {
		ob.sellOrders[order.Price] = append(ob.sellOrders[order.Price], order)
	}
}

func (ob *OrderBook) RemoveOrder(orderID string, side string) {
	switch side {
	case "BUY":
		removeFromMap(ob.buyOrders, orderID)
	case "SELL":
		removeFromMap(ob.sellOrders, orderID)
	default:
		fmt.Println("Invalid Side")
	}
}

func removeFromMap(m map[float64][]*models.Order, orderID string) (bool, error) {
	for price, orders := range m {
		for i, order := range orders {
			if order.ID == orderID {
				m[price] = append(m[price][:i], m[price][i+1:]...)
				if len(m[price]) == 0 {
					delete(m, price)
				}
				return true, nil
			}
		}
	}
	return false, errors.New("key not found")
}
