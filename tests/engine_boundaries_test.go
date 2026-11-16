package tests

import (
	"errors"
	"math"
	"testing"

	m "github.com/gonevo/matchingo"
	"github.com/nikolaydubina/fpdecimal"
)

func TestFOKRespectsOCOCancellation(t *testing.T) {
	for _, side := range []m.Side{m.Buy, m.Sell} {
		book := m.NewOrderBook()
		opposite := m.Buy
		if side == m.Buy {
			opposite = m.Sell
		}
		for _, pair := range [][2]string{{"a", "b"}, {"b", "a"}} {
			_, err := book.Process(m.NewLimitOrder(pair[0], opposite, m.FromInt(1), m.FromInt(100), m.GTC, pair[1]))
			if err != nil {
				t.Fatal(err)
			}
		}
		d, err := book.Process(m.NewLimitOrder("fok", side, m.FromInt(2), m.FromInt(100), m.FOK, ""))
		if err != nil {
			t.Fatal(err)
		}
		if len(d.Trades) != 0 || d.Stored || !d.Order.IsCanceled() || book.GetOrder("a") == nil || book.GetOrder("b") == nil {
			t.Fatalf("FOK partially mutated book: %+v", d)
		}
	}
}

func TestLargeQuoteArithmeticDoesNotOverflow(t *testing.T) {
	book := m.NewOrderBook()
	_, err := book.Process(m.NewLimitOrder("maker", m.Sell, m.FromInt(100000000), m.FromInt(100000), m.GTC, ""))
	if err != nil {
		t.Fatal(err)
	}
	expected := m.FromInt(10000000000000)
	estimate, err := book.CalculateMarketPrice(m.Buy, m.FromInt(100000000))
	if err != nil || !estimate.Equal(expected) {
		t.Fatalf("estimate=%s err=%v", estimate, err)
	}
	d, err := book.Process(m.NewMarketQuoteOrder("taker", m.Buy, expected))
	if err != nil || !d.Processed.Equal(expected) || !d.Left.Equal(fpdecimal.Zero) || book.GetOrder("maker") != nil {
		t.Fatalf("large quote: %+v %v", d, err)
	}
}

func TestMarketEstimateOverflowIsReported(t *testing.T) {
	book := m.NewOrderBook()
	_, err := book.Process(m.NewLimitOrder("maker", m.Sell, fpdecimal.FromIntScaled(math.MaxInt64), m.FromInt(2), m.GTC, ""))
	if err != nil {
		t.Fatal(err)
	}
	_, err = book.CalculateMarketPrice(m.Buy, fpdecimal.FromIntScaled(math.MaxInt64))
	if !errors.Is(err, m.ErrArithmeticOverflow) {
		t.Fatal("unrepresentable estimate did not report overflow", err)
	}
	if book.GetOrder("maker") == nil {
		t.Fatal("estimate modified book")
	}
}

func TestNoFreeQuoteFill(t *testing.T) {
	book := m.NewOrderBook()
	unit := fpdecimal.FromIntScaled(1)
	_, err := book.Process(m.NewLimitOrder("maker", m.Sell, unit, unit, m.GTC, ""))
	if err != nil {
		t.Fatal(err)
	}
	d, err := book.Process(m.NewMarketQuoteOrder("taker", m.Buy, unit))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Trades) > 0 && !d.Processed.GreaterThan(fpdecimal.Zero) {
		t.Fatal("maker filled without spending quote")
	}
}

func TestEmptyQueueFirst(t *testing.T) {
	if m.NewOrderQueue(m.FromInt(1)).First() != nil {
		t.Fatal("empty queue")
	}
}

func TestInvalidPrecisionDoesNotChangeGlobal(t *testing.T) {
	old := fpdecimal.FractionDigits
	defer func() { fpdecimal.FractionDigits = old }()
	for _, v := range []int{-1, 11, 256} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("accepted precision %d", v)
				}
			}()
			m.SetDecimalFraction(v)
		}()
		if fpdecimal.FractionDigits != old {
			t.Fatalf("invalid precision changed state to %d", fpdecimal.FractionDigits)
		}
	}
}

func TestInvalidIdentityRejected(t *testing.T) {
	for _, o := range []*m.Order{m.NewLimitOrder("", m.Buy, m.FromInt(1), m.FromInt(1), m.GTC, ""), m.NewLimitOrder("self", m.Buy, m.FromInt(1), m.FromInt(1), m.GTC, "self")} {
		_, err := m.NewOrderBook().Process(o)
		if !errors.Is(err, m.ErrInvalidOrder) {
			t.Fatalf("invalid identity: %v", err)
		}
	}
}

func TestBookCapacityRejectedBeforeMutation(t *testing.T) {
	for _, stop := range []bool{false, true} {
		book := m.NewOrderBook()
		qty := fpdecimal.FromIntScaled(math.MaxInt64 - 1)
		var order *m.Order
		if stop {
			order = m.NewStopLimitOrder("huge", m.Sell, qty, m.FromInt(2), m.FromInt(1), "")
		} else {
			order = m.NewLimitOrder("huge", m.Sell, qty, m.FromInt(2), m.GTC, "")
		}
		if _, err := book.Process(order); err != nil {
			t.Fatal(err)
		}
		if _, err := book.Process(m.NewLimitOrder("overflow", m.Buy, m.FromInt(1), m.FromInt(2), m.GTC, "")); !errors.Is(err, m.ErrBookCapacity) {
			t.Fatalf("capacity: %v", err)
		}
		if !book.GetOrder("huge").Quantity().Equal(qty) {
			t.Fatal("rejected request traded")
		}
		book.CancelOrder("huge")
		if _, err := book.Process(m.NewLimitOrder("ok", m.Buy, m.FromInt(1), m.FromInt(2), m.GTC, "")); err != nil {
			t.Fatal("capacity leaked", err)
		}
	}
}

func TestFOKWithOCOCanSucceedAndPreservesFIFO(t *testing.T) {
	book := m.NewOrderBook()
	for _, o := range []*m.Order{
		m.NewLimitOrder("a", m.Sell, m.FromInt(1), m.FromInt(100), m.GTC, "b"),
		m.NewLimitOrder("b", m.Sell, m.FromInt(1), m.FromInt(100), m.GTC, "a"),
		m.NewLimitOrder("c", m.Sell, m.FromInt(1), m.FromInt(100), m.GTC, ""),
	} {
		if _, err := book.Process(o); err != nil {
			t.Fatal(err)
		}
	}
	d, err := book.Process(m.NewLimitOrder("fok", m.Buy, m.FromInt(2), m.FromInt(100), m.FOK, ""))
	if err != nil || !d.Processed.Equal(m.FromInt(2)) || d.Stored || len(d.Trades) != 3 || d.Trades[1].OrderID != "a" || d.Trades[2].OrderID != "c" {
		t.Fatal(d, err)
	}
	if len(book.Depth().Ask) != 0 || len(book.OCO) != 0 {
		t.Fatal("OCO state leaked")
	}
}

func TestQuoteBudgetAcrossPrecisions(t *testing.T) {
	old := fpdecimal.FractionDigits
	defer func() { fpdecimal.FractionDigits = old }()
	for precision := 0; precision <= 10; precision++ {
		m.SetDecimalFraction(precision)
		for _, side := range []m.Side{m.Buy, m.Sell} {
			opposite := m.Sell
			if side == m.Sell {
				opposite = m.Buy
			}
			book := m.NewOrderBook()
			unit := fpdecimal.FromIntScaled(1)
			if _, err := book.Process(m.NewLimitOrder("maker", opposite, unit, unit, m.GTC, "")); err != nil {
				t.Fatal(err)
			}
			done, err := book.Process(m.NewMarketQuoteOrder("taker", side, unit))
			if err != nil || !done.Processed.Equal(unit) || !done.Left.Equal(fpdecimal.Zero) || len(done.Trades) != 2 || book.GetOrder("maker") != nil {
				t.Fatalf("precision %d side %s: %v %v", precision, side, done, err)
			}
		}
	}
}
