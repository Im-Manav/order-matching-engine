package engine

import (
	"errors"
	"time"

	"github.com/Im-Manav/order-matching-engine/internal/db"
	"github.com/Im-Manav/order-matching-engine/pkg/models"
	"github.com/google/uuid"
)

func Match(order *models.Order, ob *OrderBook, repo *db.Repo) ([]*models.Trade, error) {
	ob.AddOrder(order)
	order.Status = "OPEN"
	switch order.Side {
	case "BUY":
		return matchBuyOrder(order, ob, repo)
	case "SELL":
		return matchSellOrder(order, ob, repo)
	default:
		return nil, errors.New("invalid order side")
	}
}

func matchBuyOrder(order *models.Order, ob *OrderBook, repo *db.Repo) ([]*models.Trade, error) {
	originalQty := order.Quantity
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
			bestAsk.Status = "FILLED"
			repo.UpdateOrder(bestAsk)
			ob.RemoveOrder(bestAsk.ID, "SELL")
		} else {
			bestAsk.Status = "PARTIAL"
			repo.UpdateOrder(bestAsk)
		}
	}
	if order.Quantity == 0 {
		order.Status = "FILLED"
		ob.RemoveOrder(order.ID, "BUY")
	} else if order.Quantity < originalQty {
		order.Status = "PARTIAL"
	} else {
		order.Status = "OPEN"
	}
	return trades, nil
}

func matchSellOrder(order *models.Order, ob *OrderBook, repo *db.Repo) ([]*models.Trade, error) {
	originalQty := order.Quantity
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
			bestBid.Status = "FILLED"
			repo.UpdateOrder(bestBid)
			ob.RemoveOrder(bestBid.ID, "BUY")
		} else {
			repo.UpdateOrder(bestBid)
			bestBid.Status = "PARTIAL"
		}
	}
	if order.Quantity == 0 {
		order.Status = "FILLED"
		ob.RemoveOrder(order.ID, "SELL")
	} else if order.Quantity < originalQty {
		order.Status = "PARTIAL"
	} else {
		order.Status = "OPEN"
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
