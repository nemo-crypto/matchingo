package matchingo

import (
	"encoding/json"

	"github.com/nikolaydubina/fpdecimal"
)

// Level contains Price and Volume in depth
type Level struct {
	Price  fpdecimal.Decimal
	Volume fpdecimal.Decimal
}

type Depth struct {
	Ask map[string]string `json:"ask"`
	Bid map[string]string `json:"bid"`
}

// Depth returns Price levels and volume at Price level
func (ob *OrderBook) Depth() *Depth {
	depth := &Depth{
		Ask: make(map[string]string, len(ob.asks.prices)),
		Bid: make(map[string]string, len(ob.bids.prices)),
	}
	// Depth is an unordered snapshot; avoid a tree search for each level.
	for _, level := range ob.asks.prices {
		depth.Ask[level.Price().String()] = level.Volume().String()
	}
	for _, level := range ob.bids.prices {
		depth.Bid[level.Price().String()] = level.Volume().String()
	}
	return depth
}

// DepthJSON returns ask/bid depth as JSON
func (ob *OrderBook) DepthJSON() string {
	depth := ob.Depth()

	j, _ := json.Marshal(depth)

	return string(j)
}
