package tests

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	m "github.com/gonevo/matchingo"
)

// Each iteration is a complete, bounded lifecycle, not an ever-growing book.
// ns/op and allocations are per lifecycle; orders/s counts Process calls only.
func BenchmarkEngine(b *testing.B) {
	for _, scenario := range []string{"match", "cancel", "quote", "stop", "fok", "sweep100"} {
		b.Run(scenario, func(b *testing.B) {
			book := m.NewOrderBook()
			one, price := m.FromInt(1), m.FromInt(100)
			calls := 2
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				process := func(o *m.Order) *m.Done {
					d, err := book.Process(o)
					if err != nil {
						b.Fatal(err)
					}
					return d
				}
				switch scenario {
				case "match":
					process(m.NewLimitOrder("maker", m.Sell, one, price, m.GTC, ""))
					if d := process(m.NewMarketOrder("taker", m.Buy, one)); !d.Left.Equal(m.FromInt(0)) {
						b.Fatal("unfilled")
					}
				case "cancel":
					calls = 1
					process(m.NewLimitOrder("maker", m.Sell, one, price, m.GTC, ""))
					if book.CancelOrder("maker") == nil {
						b.Fatal("cancel")
					}
				case "quote":
					process(m.NewLimitOrder("maker", m.Sell, one, price, m.GTC, ""))
					process(m.NewMarketQuoteOrder("taker", m.Buy, price))
				case "stop":
					calls = 3
					process(m.NewStopLimitOrder("stop", m.Buy, one, m.FromInt(99), price, ""))
					process(m.NewLimitOrder("maker", m.Sell, one, price, m.GTC, ""))
					process(m.NewMarketOrder("taker", m.Buy, one))
					if book.CancelOrder("stop") == nil {
						b.Fatal("stop")
					}
				case "fok":
					process(m.NewLimitOrder("maker", m.Sell, one, price, m.GTC, ""))
					process(m.NewLimitOrder("taker", m.Buy, one, price, m.FOK, ""))
				case "sweep100":
					calls = 101
					for j := 0; j < 100; j++ {
						process(m.NewLimitOrder(strconv.Itoa(j), m.Sell, one, m.FromInt(100+j), m.GTC, ""))
					}
					d := process(m.NewMarketOrder("taker", m.Buy, m.FromInt(100)))
					if len(d.Trades) != 101 {
						b.Fatal("missing fills")
					}
				}
			}
			b.ReportMetric(float64(b.N*calls)/b.Elapsed().Seconds(), "orders/s")
		})
	}
}

func BenchmarkStopLookup(b *testing.B) {
	for _, n := range []int{0, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			book := m.NewStopBook()
			for i := 0; i < n; i++ {
				book.Append(m.NewStopLimitOrder(strconv.Itoa(i), m.Buy, m.FromInt(1), m.FromInt(1), m.FromInt(i+1), ""))
			}
			price := m.FromInt(n + 2)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if got := book.Activate(price); len(got) != 0 {
					b.Fatal(got)
				}
			}
		})
	}
}

func BenchmarkDepthSnapshot(b *testing.B) {
	book := m.NewOrderBook()
	for i := 0; i < 1000; i++ {
		if _, err := book.Process(m.NewLimitOrder(strconv.Itoa(i), m.Sell, m.FromInt(1), m.FromInt(100+i), m.GTC, "")); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if len(book.Depth().Ask) != 1000 {
			b.Fatal("depth")
		}
	}
}

// OCO FOK must account for cancellations rather than summing both legs.
func BenchmarkFOKWithOCO(b *testing.B) {
	for _, size := range []int{10, 1000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			book := m.NewOrderBook()
			for i := 0; i < size; i++ {
				oco := ""
				if i == 0 {
					oco = "1"
				}
				if i == 1 {
					oco = "0"
				}
				if _, err := book.Process(m.NewLimitOrder(strconv.Itoa(i), m.Sell, m.FromInt(1), m.FromInt(100), m.GTC, oco)); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				d, err := book.Process(m.NewLimitOrder("fok", m.Buy, m.FromInt(size), m.FromInt(100), m.FOK, ""))
				if err != nil || len(d.Trades) != 0 || len(d.Canceled) != 1 {
					b.Fatal("FOK must cancel", err)
				}
			}
		})
	}
}

func BenchmarkDenseLevelCancel(b *testing.B) {
	book := m.NewOrderBook()
	ids := make([]string, 10000)
	rng := rand.New(rand.NewSource(42))
	for i := range ids {
		ids[i] = strconv.Itoa(i)
		if _, err := book.Process(m.NewLimitOrder(ids[i], m.Buy, m.FromInt(1), m.FromInt(100), m.GTC, "")); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[rng.Intn(len(ids))]
		if book.CancelOrder(id) == nil {
			b.Fatal("missing cancellation")
		}
		if _, err := book.Process(m.NewLimitOrder(id, m.Buy, m.FromInt(1), m.FromInt(100), m.GTC, "")); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.N*2)/b.Elapsed().Seconds(), "operations/s")
}

func BenchmarkFOKUnrelatedOCO(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			book := m.NewOrderBook()
			for i := 0; i < size; i++ {
				sibling := ""
				if i < 2 {
					sibling = strconv.Itoa(1 - i)
				}
				if _, err := book.Process(m.NewLimitOrder(strconv.Itoa(i), m.Sell, m.FromInt(1), m.FromInt(200+i), m.GTC, sibling)); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := book.Process(m.NewLimitOrder("maker", m.Sell, m.FromInt(1), m.FromInt(100), m.GTC, "")); err != nil {
					b.Fatal(err)
				}
				done, err := book.Process(m.NewLimitOrder("taker", m.Buy, m.FromInt(1), m.FromInt(100), m.FOK, ""))
				if err != nil || !done.Processed.Equal(m.FromInt(1)) || len(done.Trades) != 2 {
					b.Fatal(done, err)
				}
			}
		})
	}
}

func BenchmarkMarketEstimate(b *testing.B) {
	book := m.NewOrderBook()
	for level := 1; level <= 1000; level++ {
		order := m.NewLimitOrder(strconv.Itoa(level), m.Sell, m.FromInt(1), m.FromInt(level), m.GTC, "")
		if _, err := book.Process(order); err != nil {
			b.Fatal(err)
		}
	}

	for _, levels := range []int{1, 100, 1000} {
		b.Run(strconv.Itoa(levels), func(b *testing.B) {
			quantity := m.FromInt(levels)
			want := m.FromInt(levels * (levels + 1) / 2)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				price, err := book.CalculateMarketPrice(m.Buy, quantity)
				if err != nil || price != want {
					b.Fatalf("estimate = %v, %v; want %v", price, err, want)
				}
			}
		})
	}
}
