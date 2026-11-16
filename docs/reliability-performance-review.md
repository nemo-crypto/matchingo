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
