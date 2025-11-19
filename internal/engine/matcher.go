package engine

import (
	"errors"
	"time"

	"github.com/Im-Manav/order-matching-engine/pkg/models"
	"github.com/google/uuid"
)

func Match(order *models.Order, ob *OrderBook) ([]*models.Trade, error) {
	switch order.Side {
	case "BUY":
		return matchBuyOrder(order, ob)
	case "SELL":
		return matchSellOrder(order, ob)
	default:
		return nil, errors.New("invalid order side")
	}
}

func matchBuyOrder(order *models.Order, ob *OrderBook) ([]*models.Trade, error) {
	trades := []*models.Trade{}
	for order.Quantity > 0 {
		bestAsk := ob.GetBestAsk()
		if bestAsk == nil || order.Price < bestAsk.Price {
			break
		}
		tradeQty := min(order.Quantity, bestAsk.Quantity)
		trade := createTrade(order, bestAsk, tradeQty)
		trades = append(trades, trade)

		order.Quantity -= tradeQty
		bestAsk.Quantity -= tradeQty

		if bestAsk.Quantity == 0 {
			ob.RemoveOrder(bestAsk.ID, "SELL")
		}
	}
	if order.Quantity > 0 {
		ob.AddOrder(order)
	}
	return trades, nil
}

func matchSellOrder(order *models.Order, ob *OrderBook) ([]*models.Trade, error) {
	trades := []*models.Trade{}
	for order.Quantity > 0 {
		bestBid := ob.GetBestBid()
		if bestBid == nil || order.Price > bestBid.Price {
			break
		}
		tradeQty := min(order.Quantity, bestBid.Quantity)
		trade := createTrade(bestBid, order, tradeQty)
		trades = append(trades, trade)

		order.Quantity -= tradeQty
		bestBid.Quantity -= tradeQty

		if bestBid.Quantity == 0 {
			ob.RemoveOrder(bestBid.ID, "BUY")
		}
	}
	if order.Quantity > 0 {
		ob.AddOrder(order)
	}
	return trades, nil
}

func createTrade(buy *models.Order, sell *models.Order, quantity int64) *models.Trade {
	return &models.Trade{
		ID:          generateTradeID(),
		BuyOrderID:  buy.ID,
		SellOrderID: sell.ID,
		Quantity:    quantity,
		Price:       sell.Price,
		Timestamp:   time.Now().UnixNano(),
	}
}

func generateTradeID() string {
	return uuid.New().String()
}
