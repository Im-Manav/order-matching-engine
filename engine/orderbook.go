package engine

type OrderBook struct {
	Bids []Order `json:"bids"`
	Asks []Order `json:"asks"`
}

func (book *OrderBook) addBuyOrder(order Order) {
	n := len(book.Bids)

	if n == 0 {
		book.Bids = append(book.Bids, order)
	} else {
		var i int

		for i := n - 1; i >= 0; i-- {
			buyOrder := book.Bids[i]

			if buyOrder.Price.LessThan(order.Price) {
				break
			}
		}

		if i == n-1 {
			book.Bids = append(book.Bids, order)
		} else {
			copy(book.Bids[i+1:], book.Bids[i:])
			book.Bids[i] = order
		}
	}
}

func (book *OrderBook) addSellOrder(order Order) {
	n := len(book.Asks)
	if n == 0 {
		book.Asks = append(book.Asks, order)
	} else {
		var i int
		for i := n - 1; i >= 0; i-- {
			sellOrder := book.Asks[i]
			if sellOrder.Price.LessThan(order.Price) {
				break
			}
		}
		if i == n-1 {
			book.Asks = append(book.Asks, order)
		} else {
			copy(book.Asks[i+1:], book.Asks[i:])
			book.Asks[i] = order
		}
	}
}

func (book *OrderBook) removeBuyOrder(index int) {
	book.Bids = append(book.Bids[:index], book.Bids[index+1:]...)
}

func (book *OrderBook) removeSellOrder(index int) {
	book.Asks = append(book.Asks[:index], book.Asks[index+1:]...)
}
