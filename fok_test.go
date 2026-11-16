package matchingo

import (
	"fmt"
	"testing"
)

func TestIndependentFOKLiquidityBoundaries(t *testing.T) {
	for _, side := range []Side{Buy, Sell} {
		for _, scenario := range []string{"unrelated", "same-level-after-fill", "consumed-oco", "triggered-stop", "unrelated-stop", "insufficient", "outside-limit", "multiple-levels"} {
			t.Run(fmt.Sprint(side)+"/"+scenario, func(t *testing.T) {
				b := NewOrderBook()
				opposite := Sell
				distant := 200
				nextPrice := 101
				if side == Sell {
					opposite = Buy
					distant = 50
					nextPrice = 99
				}
				add := func(o *Order) {
					t.Helper()
					if _, err := b.Process(o); err != nil {
						t.Fatal(err)
					}
				}
				add(NewLimitOrder("background", opposite, FromInt(1), FromInt(distant), GTC, "late"))
				oco := ""
				if scenario == "consumed-oco" {
					oco = "background"
				}
				add(NewLimitOrder("maker", opposite, FromInt(1), FromInt(100), GTC, oco))
				if scenario == "same-level-after-fill" {
					add(NewLimitOrder("later", opposite, FromInt(1), FromInt(100), GTC, "background"))
				}
				if scenario == "triggered-stop" || scenario == "unrelated-stop" {
					trigger := 100
					if scenario == "unrelated-stop" {
						trigger = 80
					}
					add(NewStopLimitOrder("stop", side, FromInt(1), FromInt(90), FromInt(trigger), "maker"))
				}
				qty, limit := 1, 100
				if scenario == "insufficient" {
					qty = 2
				}
				if scenario == "outside-limit" {
					limit = 99
					if side == Sell {
						limit = 101
					}
				}
				if scenario == "multiple-levels" {
					add(NewLimitOrder("next", opposite, FromInt(1), FromInt(nextPrice), GTC, ""))
					qty = 2
					limit = nextPrice
				}
				order := NewLimitOrder("fok", side, FromInt(qty), FromInt(limit), FOK, "")
				want := scenario == "unrelated" || scenario == "same-level-after-fill" || scenario == "unrelated-stop" || scenario == "multiple-levels"
				if got := b.hasIndependentFOKLiquidity(order); got != want {
					t.Fatalf("independent=%v want %v", got, want)
				}
				if want && !b.simulateFOK(order) {
					t.Fatal("fast path disagrees with full simulation")
				}
				d, err := b.Process(order)
				if err != nil {
					t.Fatal(err)
				}
				if want && !d.Processed.Equal(FromInt(qty)) {
					t.Fatal("unexpected partial/rejected FOK", d)
				}
				checkBookInvariants(t, b)
			})
		}
	}
}
