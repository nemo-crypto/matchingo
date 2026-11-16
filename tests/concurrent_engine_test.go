package tests

import (
	"fmt"
	"sync"
	"testing"

	m "github.com/gonevo/matchingo"
)

func TestConcurrentBookSnapshotIsolation(t *testing.T) {
	var book m.ConcurrentOrderBook
	input := m.NewLimitOrder("maker", m.Sell, m.FromInt(2), m.FromInt(10), m.GTC, "")
	done, err := book.Process(input)
	if err != nil {
		t.Fatal(err)
	}
	input.SetQuantity(m.FromInt(90))
	done.Order.Cancel()
	found := book.GetOrder("maker")
	found.SetQuantity(m.FromInt(80))
	depth := book.Depth()
	depth.Ask["10.000"] = "broken"
	if got := book.GetOrder("maker"); got.IsCanceled() || !got.Quantity().Equal(m.FromInt(2)) {
		t.Fatal("input or result escaped")
	}
	fill, err := book.Process(m.NewMarketOrder("taker", m.Buy, m.FromInt(1)))
	if err != nil || !fill.Processed.Equal(m.FromInt(1)) {
		t.Fatal(fill, err)
	}
	if !found.Quantity().Equal(m.FromInt(80)) {
		t.Fatal("snapshot changed")
	}
	canceled := book.CancelOrder("maker")
	if canceled == nil || !canceled.Quantity().Equal(m.FromInt(1)) || !canceled.IsCanceled() {
		t.Fatal("cancel")
	}
	if len(book.Depth().Ask) != 0 {
		t.Fatal("residual depth")
	}
}

func TestConcurrentBookContention(t *testing.T) {
	book := m.NewConcurrentOrderBook()
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				id := fmt.Sprintf("%d/%d", w, i)
				_, err := book.Process(m.NewLimitOrder(id, m.Buy, m.FromInt(1), m.FromInt(10), m.GTC, ""))
				if err != nil {
					t.Error(err)
					return
				}
				if book.GetOrder(id) == nil {
					t.Error("lost order")
					return
				}
				if i%20 == 0 {
					book.Depth()
					book.CalculateMarketPrice(m.Sell, m.FromInt(1))
				}
				if book.CancelOrder(id) == nil {
					t.Error("lost cancellation")
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	if len(book.Depth().Bid) != 0 {
		t.Fatal("leaked orders")
	}
}

func BenchmarkConcurrentBook(b *testing.B) {
	book := m.NewConcurrentOrderBook()
	var seq int
	var mu sync.Mutex
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		mu.Lock()
		worker := seq
		seq++
		mu.Unlock()
		id := fmt.Sprint(worker)
		for pb.Next() {
			if _, err := book.Process(m.NewLimitOrder(id, m.Buy, m.FromInt(1), m.FromInt(100), m.GTC, "")); err != nil {
				b.Error(err)
			}
			if book.CancelOrder(id) == nil {
				b.Error("cancel")
			}
		}
	})
	b.ReportMetric(float64(b.N*2)/b.Elapsed().Seconds(), "operations/s")
}
