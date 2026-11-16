package tests

import (
	"github.com/gonevo/matchingo"
	"github.com/nikolaydubina/fpdecimal"
	"testing"
)

func TestMarketEstimateRejectsInvalidRequests(t *testing.T) {
	book := matchingo.NewOrderBook()
	for _, side := range []matchingo.Side{matchingo.Buy, matchingo.Sell} {
		_, err := book.Process(matchingo.NewLimitOrder(side.String(), side, fpdecimal.FromInt(3), fpdecimal.FromInt(10+int(side)*-1), matchingo.GTC, ""))
		if err != nil {
			t.Fatal(err)
		}
	}
	before := book.DepthJSON()
	for _, tc := range []struct {
		side matchingo.Side
		qty  int
		want error
	}{
		{matchingo.Side(-1), 1, matchingo.ErrInvalidSide},
		{matchingo.Side(2), 1, matchingo.ErrInvalidSide},
		{matchingo.Buy, -1, matchingo.ErrInvalidQuantity},
		{matchingo.Sell, -1, matchingo.ErrInvalidQuantity},
	} {
		price, err := book.CalculateMarketPrice(tc.side, fpdecimal.FromInt(tc.qty))
		if err != tc.want || !price.Equal(fpdecimal.Zero) {
			t.Fatalf("price=%s error=%v, want %v", price, err, tc.want)
		}
	}
	for _, side := range []matchingo.Side{matchingo.Buy, matchingo.Sell} {
		price, err := book.CalculateMarketPrice(side, fpdecimal.Zero)
		if err != nil || !price.Equal(fpdecimal.Zero) {
			t.Fatal("zero estimate changed")
		}
	}
	if book.DepthJSON() != before {
		t.Fatal("estimate mutated book")
	}
}
