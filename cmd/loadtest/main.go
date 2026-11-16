// Command loadtest measures library operations, not network/service QPS.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	m "github.com/gonevo/matchingo"
)

func main() {
	rate := flag.Int("rate", 1000, "target library operations/sec (two operations per lifecycle)")
	duration := flag.Duration("duration", 30*time.Second, "offered-load duration")
	workers := flag.Int("workers", 8, "bounded concurrent workers on one book")
	depthLevels := flag.Int("depth", 0, "preloaded untouched ask levels, 0..10000")
	withOCO := flag.Bool("oco", false, "link two preloaded orders, exercising FOK simulation")
	flag.Parse()
	if *depthLevels < 0 || *depthLevels > 10000 || (*withOCO && *depthLevels < 2) || *rate < 2 || *rate > 2000000 || *duration < time.Second || *duration > time.Hour || *workers < 1 || *workers > 1024 {
		fmt.Fprintln(os.Stderr, "require rate 2..2000000, duration 1s..1h, workers 1..1024, depth 0..10000 (OCO requires depth >=2)")
		os.Exit(2)
	}
	total := int64(float64(*rate) * duration.Seconds() / 2)
	// Bounded memory: a microsecond histogram up to one second plus overflow.
	histogram := make([]atomic.Int64, 1000002)
	var completed, failures, maxLatency atomic.Int64
	book := m.NewConcurrentOrderBook()
	for i := 0; i < *depthLevels; i++ {
		oco := ""
		if *withOCO && i < 2 {
			oco = fmt.Sprintf("seed-%d", 1-i)
		}
		if _, err := book.Process(m.NewLimitOrder(fmt.Sprintf("seed-%d", i), m.Sell, m.FromInt(1), m.FromInt(200+i), m.GTC, oco)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	type job struct {
		id  int64
		due time.Time
	}
	jobs := make(chan job, *workers*2)
	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				id := fmt.Sprint(j.id)
				var err error
				var done *m.Done
				switch j.id % 4 {
				case 0:
					_, err = book.Process(m.NewLimitOrder(id, m.Buy, m.FromInt(1), m.FromInt(1), m.GTC, ""))
					if err == nil && book.CancelOrder(id) == nil {
						err = fmt.Errorf("missing cancellation")
					}
				default:
					_, err = book.Process(m.NewLimitOrder(id, m.Sell, m.FromInt(1), m.FromInt(100), m.GTC, ""))
					if err == nil {
						switch j.id % 4 {
						case 1:
							done, err = book.Process(m.NewMarketOrder("t"+id, m.Buy, m.FromInt(1)))
						case 2:
							done, err = book.Process(m.NewMarketQuoteOrder("t"+id, m.Buy, m.FromInt(100)))
						case 3:
							done, err = book.Process(m.NewLimitOrder("t"+id, m.Buy, m.FromInt(1), m.FromInt(100), m.FOK, ""))
						}
						if err == nil && (done == nil || len(done.Trades) == 0 || done.Left.GreaterThan(m.FromInt(0))) {
							err = fmt.Errorf("incomplete fill")
						}
					}
				}
				if err != nil {
					failures.Add(1)
				} else {
					completed.Add(2)
				}
				latency := time.Since(j.due).Microseconds()
				for old := maxLatency.Load(); latency > old; old = maxLatency.Load() {
					if maxLatency.CompareAndSwap(old, latency) {
						break
					}
				}
				bucket := latency
				if bucket < 0 {
					bucket = 0
				}
				if bucket > 1000000 {
					bucket = 1000001
				}
				histogram[bucket].Add(1)
			}
		}()
	}
	start := time.Now()
	// Absolute deadlines include producer lag and queueing in latency. Blocking
	// on a full bounded queue does not drop offered work or reset its deadline.
	for i := int64(0); i < total; i++ {
		due := start.Add(time.Duration(float64(i) * 2 / float64(*rate) * float64(time.Second)))
		if wait := time.Until(due); wait > 0 {
			time.Sleep(wait)
		}
		jobs <- job{i, due}
	}
	close(jobs)
	wg.Wait()
	if remaining := time.Until(start.Add(*duration)); remaining > 0 {
		time.Sleep(remaining)
	}
	elapsed := time.Since(start)
	percentiles := []float64{.5, .95, .99}
	sort.Float64s(percentiles)
	fmt.Printf("workload=25%% append/cancel,25%% market,25%% quote,25%% FOK; one shared book; seed_levels=%d OCO=%v\n", *depthLevels, *withOCO)
	fmt.Printf("target_ops_s=%d offered_ops=%d completed_ops=%d errors=%d elapsed=%s achieved_ops_s=%.2f\n", *rate, total*2, completed.Load(), failures.Load(), elapsed, float64(completed.Load())/elapsed.Seconds())
	for _, p := range percentiles {
		target := int64(float64(total) * p)
		if target < 1 {
			target = 1
		}
		var sum int64
		for us := range histogram {
			sum += histogram[us].Load()
			if sum >= target {
				if us == 1000001 {
					fmt.Printf("cycle_p%.0f=>1s ", p*100)
				} else {
					fmt.Printf("cycle_p%.0f=%dus ", p*100, us)
				}
				break
			}
		}
	}
	fmt.Printf("cycle_max=%dus (includes scheduling and queueing)\n", maxLatency.Load())
	for i := 0; i < *depthLevels; i++ {
		o := book.CancelOrder(fmt.Sprintf("seed-%d", i))
		if o == nil || !o.Quantity().Equal(m.FromInt(1)) {
			failures.Add(1)
		}
	}
	depth := book.Depth()
	if failures.Load() != 0 || len(depth.Ask) != 0 || len(depth.Bid) != 0 || float64(completed.Load())/elapsed.Seconds() < float64(*rate)*.99 {
		fmt.Fprintln(os.Stderr, "FAIL: errors, residual liquidity or throughput below 99% target")
		os.Exit(1)
	}
	fmt.Println("PASS")
}
