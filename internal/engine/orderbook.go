package engine

import (
	"errors"
	"fmt"
	"sort"

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

// The highest buy
func (ob *OrderBook) GetBestBid() *models.Order {
	priceList := createPriceList(ob.buyOrders)
	sort.Float64s(priceList)
	return ob.buyOrders[priceList[len(priceList)-1]][0]
}

// The lowest sell
func (ob *OrderBook) GetBestAsk() *models.Order {
	priceList := createPriceList(ob.sellOrders)
	sort.Float64s(priceList)
	return ob.sellOrders[priceList[0]][0]
}

// Helper for Remove Order
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

// Helper for GetBestBid() and GetBestAsk()
func createPriceList(m map[float64][]*models.Order) []float64 {
	if m == nil {
		return []float64{}
	}
	var priceList []float64
	for price := range m {
		priceList = append(priceList, price)
	}
	return priceList
}
