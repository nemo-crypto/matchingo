package matchingo

// OCO outside the consumed FIFO prefix cannot cancel liquidity during this
// fill. Any potentially activated stop forces the full simulation.
func (ob *OrderBook) canFillWithOCO(order *Order) bool {
	if ob.hasIndependentFOKLiquidity(order) {
		return true
	}
	return ob.simulateFOK(order)
}

func (ob *OrderBook) hasIndependentFOKLiquidity(order *Order) bool {
	side := ob.asks
	withinLimit := order.Price().GreaterThanOrEqual
	if order.Side() == Sell {
		side = ob.bids
		withinLimit = order.Price().LessThanOrEqual
	}
	remaining := order.Quantity()
	for queue := side.BestPriceQueue(); queue != nil && withinLimit(queue.Price()); queue = side.NextLevel(queue.Price()) {
		if ob.Stop.prices[queue.Price()] != nil {
			return false
		}
		for i := 0; i < queue.Len(); i++ {
			maker := queue.Orders.At(i)
			if maker.OCO() != "" {
				return false
			}
			if maker.Quantity().GreaterThanOrEqual(remaining) {
				return true
			}
			remaining = remaining.Sub(maker.Quantity())
		}
	}
	return false
}

// OCO legs are not independent liquidity. Simulate on an isolated copy.
// IOC gives the same matching path without recursively running a FOK check.
func (ob *OrderBook) simulateFOK(order *Order) bool {
	copyBook := NewOrderBook()
	copyBook.orders = make(map[string]*Order, len(ob.orders))
	copyBook.Stop.orders = make(map[string]*Order, len(ob.Stop.orders))
	copyBook.OCO = make(map[string]struct{}, len(ob.OCO))
	// One backing array keeps copied orders independent without one allocation
	// per order. Price insertion order is irrelevant; preserve FIFO within levels.
	copies := make([]Order, len(ob.orders))
	next := 0
	clone := func(order *Order) *Order {
		copies[next] = *order
		result := &copies[next]
		next++
		return result
	}
	for _, side := range []*OrderSide{ob.asks, ob.bids} {
		for _, queue := range side.prices {
			for i := 0; i < queue.Len(); i++ {
				copyBook.appendLimitOrder(clone(queue.Orders.At(i)))
			}
		}
	}
	// Preserve FIFO order within each stop level too.
	for _, queue := range ob.Stop.prices {
		for i := 0; i < queue.Len(); i++ {
			copyOrder := clone(queue.Orders.At(i))
			copyBook.Stop.Append(copyOrder)
			copyBook.volume = copyBook.volume.Add(copyOrder.Quantity())
			copyBook.orders[copyOrder.ID()] = copyOrder
			if copyOrder.oco != "" {
				copyBook.ocoOrders++
			}
		}
	}
	for id := range ob.OCO {
		copyBook.OCO[id] = struct{}{}
	}
	taker := *order
	taker.tif = IOC
	done, _ := copyBook.processLimitOrder(&taker)
	return done.Processed.Equal(order.Quantity())
}
