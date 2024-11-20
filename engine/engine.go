package engine

import "log"

func (book *OrderBook) Process(order Order) []Trade {
	if order.Side.String() == "buy" {
		return book.processLimitBuyOrder(order)
	}
	return book.processLimitSellOrder(order)
}

func (book *OrderBook) processLimitBuyOrder(order Order) []Trade {
	log.Println("Processing LIMIT BUY ORDER")
	trades := make([]Trade, 0, 1)
	n := len(book.Asks)
	if n == 0 {
		book.addBuyOrder(order)
		return trades
	}

	if n != 0 || book.Asks[n-1].Price.LessThanOrEqual(order.Price) {
		for i := n - 1; i >= 0; i-- {
			sellOrder := book.Asks[i]

			if sellOrder.Price.GreaterThan(order.Price) {
				break
			}

			if sellOrder.Quantity.GreaterThan(order.Quantity) {
				trades = append(
					trades,
					Trade{
						TakerOrderID: order.ID,
						MakerOrderID: sellOrder.ID,
						Quantity:     order.Quantity.BigInt().Uint64(),
						Price:        sellOrder.Price.BigInt().Uint64(),
						Timestamp:    order.Timestamp,
					},
				)
			}
			if sellOrder.Quantity.LessThan(order.Quantity) {
				trades = append(
					trades,
					Trade{
						TakerOrderID: order.ID,
						MakerOrderID: sellOrder.ID,
						Quantity:     sellOrder.Quantity.BigInt().Uint64(),
						Price:        sellOrder.Price.BigInt().Uint64(),
						Timestamp:    order.Timestamp,
					},
				)
				order.Quantity = order.Quantity.Sub(sellOrder.Quantity)
				book.removeSellOrder(i)
				continue
			}
		}
	}
	book.addBuyOrder(order)
	return trades
}

func (book *OrderBook) processLimitSellOrder(order Order) []Trade {
	log.Println("Processing LIMIT SELL ORDER ")

	trades := make([]Trade, 0, 1)
	n := len(book.Bids)

	if n == 0 {
		book.addSellOrder(order)
		return trades
	}

	// Proceed only if the sell Price is Greather than user highest buy Price
	if n != 0 || book.Bids[n-1].Price.GreaterThanOrEqual(order.Price) {
		// travers all bids that match
		for i := n - 1; i >= 0; i-- {
			buyOrder := book.Bids[i]

			if buyOrder.Price.LessThan(order.Price) {
				break // exit
			}

			// fill the entire order of buy order is gte
			if buyOrder.Price.GreaterThanOrEqual(order.Price) {
				trades = append(
					trades,
					Trade{
						TakerOrderID: order.ID,
						MakerOrderID: buyOrder.ID,
						Quantity:     order.Quantity.BigInt().Uint64(),
						Price:        buyOrder.Price.BigInt().Uint64(),
						Timestamp:    order.Timestamp,
					},
				)
				buyOrder.Quantity = buyOrder.Quantity.Sub(order.Quantity)

				// if buyOrder.Quantity = 0
				if buyOrder.Quantity.IsZero() {
					book.removeBuyOrder(i)
				}
				return trades
			}

			// fill a partial order and continue
			if buyOrder.Quantity.LessThan(order.Quantity) {
				trades = append(
					trades,
					Trade{
						TakerOrderID: order.ID,
						MakerOrderID: buyOrder.ID,
						Quantity:     buyOrder.Quantity.BigInt().Uint64(),
						Price:        buyOrder.Price.BigInt().Uint64(),
						Timestamp:    order.Timestamp,
					},
				)
				order.Quantity = order.Quantity.Sub(buyOrder.Quantity)
				book.removeBuyOrder(i)
				continue
			}
		}
	}
	book.addSellOrder(order)
	return trades
}
