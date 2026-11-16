# Reliability and performance review — 2026-11-16

This fork branch combines the three upstream stop/OCO fixes with a focused review of public input handling, quote-budget accounting, output behavior, and allocation cost. It does not change the module's Go 1.20 language version or add dependencies.

## Changes

- Reject nil/unknown orders, invalid sides, canceled orders, non-positive remaining quantities, invalid prices, and duplicate live IDs before mutating either book. Duplicate validation now covers market and stop orders too. Constructors retain their documented panic behavior; `Process` reports errors.
- Preserve the unspent quote-currency remainder after fixed-point division. Stop when the remainder cannot buy a base unit instead of treating truncation as expenditure. A 300 quote-unit buy across prices 100 and 110 now reports 299.990 processed and 0.010 canceled remainder, rather than claiming all 300 were spent.
- Report completely unfilled market orders as canceled with their full remaining quantity. The established non-matching limit-order result convention is preserved.
- Remove unsolicited `Depth` stdout output. Order-book snapshots should not print internal order structures or interfere with a service's output protocol.
- Preallocate queue-drain output while preserving FIFO order and the existing nil result for an empty queue.
- Incorporate stop-index cleanup, distinct activation pointers, and single-book OCO cancellation from upstream PRs #3, #4 and #5.

## Validation and local measurements

Go 1.25.11, macOS arm64, Apple M5. `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, formatting and whitespace checks pass. Regression tests demonstrate the original failures before the new fixes.

`go test ./tests -run '^$' -bench BenchmarkQueueSliceReview -benchmem -count=3` constructs and drains 1,000 queued entries per iteration:

| Metric | Before preallocation | After preallocation |
| --- | ---: | ---: |
| Time, median | 39,066 ns/op | 17,388 ns/op |
| Bytes | 518,093 B/op | 167,257 B/op |
| Allocations | 27 | 16 |

This is a local microbenchmark, not exchange throughput or tail latency. Re-run on deployment hardware before sizing capacity.

## Remaining boundaries

The book is single-threaded mutable state; callers must serialize access and must not mutate resting `Order` objects returned by lookup. Race-test success does not make concurrent access supported. This review does not establish correctness for fixed-point integer overflow, configurable decimal precision, sub-quote-unit settlement/rounding policies, persistence/replay, pre-trade balances, self-trade prevention or financial settlement. No production exchange or live funds were used.

## Follow-up: safe removal boundaries

`OrderSide.Remove` now returns nil for absent or nil orders without changing order counts, depth, or volume. `OrderQueue.Remove` rejects nil and `RemoveIndex` returns false for any out-of-range index instead of panicking. Successful removals retain their existing behavior. Regression tests cover bid/ask sides, absent price levels, repeated removal, nil inputs, invalid indexes, and successful cleanup. The tests failed on the prior version (incorrect removal result and index panic). Full race tests and `go vet ./...` passed. This does not make the mutable book thread-safe.

## Follow-up: late OCO siblings, stop removal, and FOK traversal

When an OCO primary fills before its sibling is submitted, the pending cancellation now marks the incoming sibling canceled and reports its ID. Stop-limit submission also checks that pending marker before storage. Both buy/sell directions and primary maker/taker fills are covered, with limit and stop siblings. Existing no-trade result quantity conventions are preserved.

StopBook.Remove now delegates to the stored ID lookup and returns the indexed order. Nil, unknown IDs, repeated removals, and removal through a different object carrying the same ID no longer dereference missing price queues or corrupt counts. This does not permit callers to mutate resting orders.

FOK prechecks traverse the sorted price tree directly, stopping as soon as the required quantity is found or the limit price is exceeded. They no longer allocate a snapshot of every price. Boundary tests verify buy/sell price limits and available volume; existing full matching tests remain in place.

Validation: full `go test ./... -count=1`, `go test -race ./... -count=1`, and `go vet ./...` passed on Go 1.25.11 / Apple M5. The new lifecycle regressions failed on the previous version (eight missing OCO cancellations and a nil-stop removal panic).

Reproduce the FOK-only benchmark with `go test ./tests -run '^$' -bench BenchmarkFOKDepthReview -benchmem -benchtime=100ms -count=3`. Setup is excluded from the timer. Median of three local repetitions:

| 10,000 price levels | Before ns/op | After ns/op | Before B/op | After B/op |
| --- | ---: | ---: | ---: | ---: |
| Buy, quantity 1 | 38,966 | 19.10 | 81,920 | 0 |
| Buy, quantity 10,000 | 80,873 | 73,142 | 81,920 | 0 |
| Sell, quantity 1 | 37,043 | 34.18 | 81,920 | 0 |
| Sell, quantity 10,000 | 78,014 | 72,950 | 81,920 | 0 |

All measured cases went from one allocation per check to zero. These results isolate FOK prechecks, not order processing, concurrency, exchange throughput, or latency under load. Unsubmitted OCO siblings still leave pending cancellation markers until consumed; this change does not introduce marker expiry, ID generations, or a new stop-trigger policy.

## Follow-up: submitted versus constructor quantity

Done now snapshots Order.Quantity at submission instead of OriginalQty. Using SetQuantity before submitting a fresh order previously made fill totals refer to the constructor value, and an unmatched limit could be restored to that old value. OriginalQty remains unchanged for historical inspection. Regression tests cover reduced/increased quantities, limit/base-market/quote-market orders, and empty/partial/full liquidity; all 18 cases failed before the fix. Full tests, race tests, and vet pass. This does not authorize mutating orders while they rest in the book.
