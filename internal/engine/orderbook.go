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
	if len(ob.buyOrders) == 0 {
		return nil
	}
	priceList := createPriceList(ob.buyOrders)
	if len(priceList) == 0 {
		return nil
	}
	sort.Float64s(priceList)
	return ob.buyOrders[priceList[len(priceList)-1]][0]
}

// The lowest sell
func (ob *OrderBook) GetBestAsk() *models.Order {
	if len(ob.sellOrders) == 0 {
		return nil
	}
	priceList := createPriceList(ob.sellOrders)
	if len(priceList) == 0 {
		return nil
	}
	sort.Float64s(priceList)
	return ob.sellOrders[priceList[0]][0]
}

// To print the entire book
func (ob *OrderBook) Printbook(depth int) {
	fmt.Println("----Order Book----")
	sellPrices := createPriceList(ob.sellOrders)
	sort.Float64s(sellPrices)
	for i, price := range sellPrices {
		if i >= depth {
			break
		}
		fmt.Printf("SELL PRICE: %.2f | QTY: %d\n", price, totalQuantity(ob.sellOrders[price]))
	}

	buyPrices := createPriceList(ob.buyOrders)
	sort.Float64s(buyPrices)
	for i := len(buyPrices) - 1; i >= 0; i-- {
		price := buyPrices[i]
		if len(buyPrices)-i >= depth {
			break
		}
		fmt.Printf("BUY PRICE: %.2f | QTY: %d", price, totalQuantity(ob.buyOrders[price]))
	}
}

// <-- Helper Functions -->

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
	for price, orders := range m {
		if len(orders) > 0 {
			priceList = append(priceList, price)
		}
	}
	return priceList
}

// Helper to calculate total quantity at a price
func totalQuantity(o []*models.Order) int {
	qty := 0
	for _, order := range o {
		qty += int(order.Quantity)
	}
	return qty
}
