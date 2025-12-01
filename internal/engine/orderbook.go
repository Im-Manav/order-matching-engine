package engine

// This file is only needed to implement in-memory orderbook operations

import (
	"errors"
	"fmt"
	"sort"
	"time"

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
	order.CreatedAt = time.Now()
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

	for _, price := range priceList {
		for _, order := range ob.buyOrders[price] {
			if order.Quantity > 0 {
				return order
			}
		}
	}
	return nil
}

// The lowest sell
func (ob *OrderBook) GetBestAsk() *models.Order {
	priceList := createPriceList(ob.sellOrders)
	sort.Float64s(priceList)

	for _, price := range priceList {
		for _, o := range ob.sellOrders[price] {
			if o.Quantity > 0 {
				return o
			}
		}
	}
	return nil
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

func (ob *OrderBook) FindOrder(id string) *models.Order {
	for _, orders := range ob.buyOrders {
		for _, o := range orders {
			if o.ID == id {
				return o
			}
		}
	}
	for _, orders := range ob.sellOrders {
		for _, o := range orders {
			if o.ID == id {
				return o
			}
		}
	}
	return nil
}

func (ob *OrderBook) Cancel(id string) (bool, string) {
	if ok, _ := removeFromMap(ob.buyOrders, id); ok {
		return true, "BUY"
	}
	if ok, _ := removeFromMap(ob.sellOrders, id); ok {
		return true, "SELL"
	}
	return false, ""
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
