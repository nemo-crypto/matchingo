package matchingo

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestMixedOrderInvariants(t *testing.T) {
	for seed := int64(0); seed < 8; seed++ {
		rng := rand.New(rand.NewSource(seed))
		book := NewOrderBook()
		for step := 0; step < 3000; step++ {
			id := fmt.Sprintf("%d", step)
			side := Buy
			if rng.Intn(2) == 0 {
				side = Sell
			}
			qty, price := FromInt(rng.Intn(5)+1), FromInt(rng.Intn(5)+98)
			oco := ""
			if step > 0 && rng.Intn(4) == 0 {
				oco = fmt.Sprint(rng.Intn(step))
			}
			var order *Order
			switch rng.Intn(7) {
			case 0:
				book.CancelOrder(fmt.Sprint(rng.Intn(step + 1)))
			case 1:
				order = NewMarketOrder(id, side, qty)
			case 2:
				order = NewMarketQuoteOrder(id, side, FromInt(rng.Intn(500)+1))
			case 3:
				order = NewStopLimitOrder(id, side, qty, price, FromInt(rng.Intn(5)+98), oco)
			default:
				tif := []TIF{GTC, IOC, FOK}[rng.Intn(3)]
				order = NewLimitOrder(id, side, qty, price, tif, oco)
			}
			if order != nil {
				if order.TIF() == FOK && book.ocoOrders > 0 && book.hasIndependentFOKLiquidity(order) && !book.simulateFOK(order) {
					t.Fatalf("independent FOK disagrees with simulation seed=%d step=%d", seed, step)
				}
				done, err := book.Process(order)
				if err != nil {
					t.Fatalf("seed=%d step=%d: %v", seed, step, err)
				}
				if order.TIF() == FOK && done.Processed.Scaled() != 0 && !done.Processed.Equal(done.Quantity) {
					t.Fatalf("partial FOK seed=%d step=%d", seed, step)
				}
			}
			checkBookInvariants(t, book)
		}
	}
}

func checkBookInvariants(t *testing.T, b *OrderBook) {
	t.Helper()
	seen := map[string]bool{}
	var total int64
	oco := 0
	inspect := func(q *OrderQueue) {
		var volume int64
		for i := 0; i < q.Len(); i++ {
			o := q.Orders.At(i)
			if seen[o.ID()] || b.orders[o.ID()] != o || o.IsCanceled() || o.Quantity().Scaled() <= 0 {
				t.Fatalf("invalid indexed order %s", o.ID())
			}
			seen[o.ID()] = true
			volume += o.Quantity().Scaled()
			if o.OCO() != "" {
				oco++
			}
		}
		if volume != q.Volume().Scaled() {
			t.Fatalf("queue volume %d != %d", volume, q.Volume().Scaled())
		}
		total += volume
	}
	for _, s := range []*OrderSide{b.asks, b.bids} {
		count := 0
		for p, q := range s.prices {
			if !p.Equal(q.Price()) || q.Len() == 0 {
				t.Fatal("price index")
			}
			inspect(q)
			count += q.Len()
		}
		if s.Len() != count || s.Depth() != len(s.prices) || len(s.Prices()) != s.Depth() {
			t.Fatal("side index")
		}
	}
	stopCount := 0
	for price, q := range b.Stop.prices {
		inspect(q)
		for i := 0; i < q.Len(); i++ {
			o := q.Orders.At(i)
			if !o.IsStopOrder() || !price.Equal(o.StopPrice()) || b.Stop.orders[o.ID()] != o {
				t.Fatal("stop index")
			}
		}
		stopCount += q.Len()
	}
	if stopCount != b.Stop.Len() || stopCount != len(b.Stop.orders) {
		t.Fatal("stop count")
	}
	if len(seen) != len(b.orders) || total != b.volume.Scaled() || oco != b.ocoOrders {
		t.Fatalf("book totals orders=%d/%d volume=%d/%d OCO=%d/%d", len(seen), len(b.orders), total, b.volume.Scaled(), oco, b.ocoOrders)
	}
	snapshot := b.Depth()
	for _, s := range []*OrderSide{b.asks, b.bids} {
		for p, q := range s.prices {
			levels := snapshot.Ask
			if s == b.bids {
				levels = snapshot.Bid
			}
			if levels[p.String()] != q.Volume().String() {
				t.Fatal("depth snapshot")
			}
		}
	}
}

// A failed FOK may trigger stops and cancel OCO siblings in its dry run.
// None of those simulated changes may escape into the live book.
func TestFOKSimulationPreservesStopAndOCOState(t *testing.T) {
	b := NewOrderBook()
	orders := []*Order{
		NewLimitOrder("a", Sell, FromInt(1), FromInt(100), GTC, "b"),
		NewLimitOrder("b", Sell, FromInt(1), FromInt(100), GTC, "a"),
		NewStopLimitOrder("stop", Buy, FromInt(1), FromInt(90), FromInt(100), "late"),
	}
	for _, o := range orders {
		if _, err := b.Process(o); err != nil {
			t.Fatal(err)
		}
	}
	b.OCO["pending"] = struct{}{}
	for i := 0; i < 3; i++ {
		d, err := b.Process(NewLimitOrder("fok", Buy, FromInt(2), FromInt(100), FOK, ""))
		if err != nil || !d.Order.IsCanceled() || len(d.Trades) != 0 || len(d.Activated) != 0 {
			t.Fatal(d, err)
		}
		for _, o := range orders {
			if b.GetOrder(o.ID()) != o || o.IsCanceled() || !o.Quantity().Equal(FromInt(1)) {
				t.Fatal("simulation changed live order")
			}
		}
		if !orders[2].IsStopOrder() || b.Stop.Len() != 1 || len(b.OCO) != 1 {
			t.Fatal("simulation changed stop/OCO state")
		}
		if _, ok := b.OCO["pending"]; !ok {
			t.Fatal("simulation consumed marker")
		}
		checkBookInvariants(t, b)
	}
}
