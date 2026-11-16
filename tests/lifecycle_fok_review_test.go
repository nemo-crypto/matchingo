package tests

import (
	"fmt"
	"testing"

	"github.com/gonevo/matchingo"
	"github.com/nikolaydubina/fpdecimal"
)

func TestLateOCOSiblingIsCanceled(t *testing.T) {
	for _, side := range []matchingo.Side{matchingo.Buy, matchingo.Sell} {
		for _, primaryRole := range []string{"maker", "taker"} {
			for _, kind := range []string{"limit", "stop"} {
				t.Run(side.String()+"/"+primaryRole+"/"+kind, func(t *testing.T) {
					book := matchingo.NewOrderBook()
					opposite := matchingo.Buy
					if side == matchingo.Buy {
						opposite = matchingo.Sell
					}
					process := func(order *matchingo.Order) *matchingo.Done {
						t.Helper()
						done, err := book.Process(order)
						if err != nil {
							t.Fatal(err)
						}
						return done
					}
					primary := matchingo.NewLimitOrder("primary", side, fpdecimal.FromInt(1), fpdecimal.FromInt(10), matchingo.GTC, "late")
					if primaryRole == "taker" {
						process(matchingo.NewLimitOrder("liquidity", opposite, fpdecimal.FromInt(1), fpdecimal.FromInt(10), matchingo.GTC, ""))
						process(primary)
					} else {
						process(primary)
						process(matchingo.NewMarketOrder("fill", opposite, fpdecimal.FromInt(1)))
					}
					if _, ok := book.OCO["late"]; !ok {
						t.Fatal("missing pending cancellation")
					}
					late := matchingo.NewLimitOrder("late", side, fpdecimal.FromInt(2), fpdecimal.FromInt(10), matchingo.GTC, "primary")
					if kind == "stop" {
						late = matchingo.NewStopLimitOrder("late", side, fpdecimal.FromInt(2), fpdecimal.FromInt(10), fpdecimal.FromInt(11), "primary")
					}
					done := process(late)
					if !late.IsCanceled() || len(done.Canceled) != 1 || done.Canceled[0] != "late" {
						t.Fatalf("missing cancellation: %s", done)
					}
					if done.Stored || len(done.Trades) != 0 || book.GetOrder("late") != nil || book.Stop.Len() != 0 || len(book.OCO) != 0 {
						t.Fatal("late sibling changed book or retained marker")
					}
					if len(book.Depth().Ask) != 0 || len(book.Depth().Bid) != 0 {
						t.Fatal("late sibling left liquidity")
					}
				})
			}
		}
	}
}

func TestStopRemovalUsesStoredOrderAndIsIdempotent(t *testing.T) {
	for _, kind := range []string{"nil", "same-price-missing", "other-price-missing", "same-id-copy"} {
		t.Run(kind, func(t *testing.T) {
			book := matchingo.NewStopBook()
			original := matchingo.NewStopLimitOrder("live", matchingo.Buy, fpdecimal.FromInt(3), fpdecimal.FromInt(10), fpdecimal.FromInt(11), "")
			book.Append(original)
			var candidate *matchingo.Order
			switch kind {
			case "same-price-missing":
				candidate = matchingo.NewStopLimitOrder("missing", matchingo.Buy, fpdecimal.FromInt(9), fpdecimal.FromInt(10), fpdecimal.FromInt(11), "")
			case "other-price-missing":
				candidate = matchingo.NewStopLimitOrder("missing", matchingo.Buy, fpdecimal.FromInt(9), fpdecimal.FromInt(10), fpdecimal.FromInt(12), "")
			case "same-id-copy":
				candidate = matchingo.NewStopLimitOrder("live", matchingo.Buy, fpdecimal.FromInt(9), fpdecimal.FromInt(10), fpdecimal.FromInt(12), "")
			}
			if kind == "same-id-copy" {
				if book.Remove(candidate) != original || book.Len() != 0 {
					t.Fatal("removal did not use indexed order")
				}
			} else {
				if book.Remove(candidate) != nil || book.Len() != 1 {
					t.Fatal("missing removal mutated stop book")
				}
				if book.Remove(original) != original {
					t.Fatal("live removal failed")
				}
			}
			if book.Remove(original) != nil || book.RemoveByID("live") != nil || book.Len() != 0 {
				t.Fatal("repeat removal changed state")
			}
			if len(book.Activate(fpdecimal.FromInt(11))) != 0 {
				t.Fatal("removed order still activates")
			}
		})
	}
}

func TestFOKPriceAndVolumeBoundaries(t *testing.T) {
	for _, takerSide := range []matchingo.Side{matchingo.Buy, matchingo.Sell} {
		side := matchingo.NewOrderSideAsk()
		makerSide := matchingo.Sell
		if takerSide == matchingo.Sell {
			side = matchingo.NewOrderSideBid()
			makerSide = matchingo.Buy
		}
		for i, price := range []int{10, 20, 30} {
			side.Append(matchingo.NewLimitOrder(fmt.Sprint(i), makerSide, fpdecimal.FromInt(2), fpdecimal.FromInt(price), matchingo.GTC, ""))
		}
		for _, price := range []int{5, 10, 15, 20, 25, 30, 35} {
			available := 0
			for _, makerPrice := range []int{10, 20, 30} {
				if takerSide == matchingo.Buy && makerPrice <= price || takerSide == matchingo.Sell && makerPrice >= price {
					available += 2
				}
			}
			for _, qty := range []int{1, 2, 3, 4, 5, 6, 7} {
				got := side.CanOrderBeFilled(takerSide, fpdecimal.FromInt(price), fpdecimal.FromInt(qty))
				if got != (available >= qty) {
					t.Fatalf("side=%s price=%d qty=%d available=%d got=%v", takerSide, price, qty, available, got)
				}
			}
		}
		if side.Len() != 3 || side.Depth() != 3 {
			t.Fatal("FOK check changed book")
		}
	}
}

func BenchmarkFOKDepthReview(b *testing.B) {
	for _, depth := range []int{100, 10000} {
		for _, takerSide := range []matchingo.Side{matchingo.Buy, matchingo.Sell} {
			side := matchingo.NewOrderSideAsk()
			makerSide := matchingo.Sell
			limit := depth
			if takerSide == matchingo.Sell {
				side = matchingo.NewOrderSideBid()
				makerSide = matchingo.Buy
				limit = 1
			}
			for i := 1; i <= depth; i++ {
				side.Append(matchingo.NewLimitOrder(fmt.Sprint(i), makerSide, fpdecimal.FromInt(1), fpdecimal.FromInt(i), matchingo.GTC, ""))
			}
			for _, qty := range []int{1, depth} {
				b.Run(fmt.Sprintf("%s/depth%d/qty%d", takerSide, depth, qty), func(b *testing.B) {
					price, quantity := fpdecimal.FromInt(limit), fpdecimal.FromInt(qty)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if !side.CanOrderBeFilled(takerSide, price, quantity) {
							b.Fatal("unexpected unfillable order")
						}
					}
				})
			}
		}
	}
}
