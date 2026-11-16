package matchingo

import (
	"sync"

	"github.com/nikolaydubina/fpdecimal"
)

// ConcurrentOrderBook serializes all operations for one instrument. Orders are
// copied on entry and results are detached snapshots: callers cannot mutate
// resting orders through input, lookup, cancellation or Done.Order pointers.
// Do not mutate an input while Process is copying it. Configure decimal precision
// once, before starting any goroutines. Use one book per instrument for parallelism.
// The zero value is ready to use; do not copy this struct after first use.
type ConcurrentOrderBook struct {
	mu   sync.Mutex
	book *OrderBook
}

func NewConcurrentOrderBook() *ConcurrentOrderBook { return &ConcurrentOrderBook{book: NewOrderBook()} }
func (b *ConcurrentOrderBook) init() {
	if b.book == nil {
		b.book = NewOrderBook()
	}
}
func cloneOrder(o *Order) *Order {
	if o == nil {
		return nil
	}
	copy := *o
	return &copy
}

func (b *ConcurrentOrderBook) Process(order *Order) (*Done, error) {
	input := cloneOrder(order)
	b.mu.Lock()
	defer b.mu.Unlock()
	b.init()
	done, err := b.book.Process(input)
	if done != nil {
		done.Order = cloneOrder(done.Order)
	}
	return done, err
}
func (b *ConcurrentOrderBook) CancelOrder(id string) *Order {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.init()
	return cloneOrder(b.book.CancelOrder(id))
}
func (b *ConcurrentOrderBook) GetOrder(id string) *Order {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.init()
	return cloneOrder(b.book.GetOrder(id))
}
func (b *ConcurrentOrderBook) Depth() *Depth {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.init()
	return b.book.Depth()
}
func (b *ConcurrentOrderBook) CalculateMarketPrice(side Side, quantity fpdecimal.Decimal) (fpdecimal.Decimal, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.init()
	return b.book.CalculateMarketPrice(side, quantity)
}
