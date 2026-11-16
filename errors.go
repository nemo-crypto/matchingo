package matchingo

import "errors"

// OrderBook errors
var (
	ErrBookCapacity         = errors.New("orderbook: aggregate resting quantity exceeds decimal capacity")
	ErrArithmeticOverflow   = errors.New("orderbook: arithmetic overflow")
	ErrInvalidPrecision     = errors.New("orderbook: precision must be between 0 and 10")
	ErrInvalidOrder         = errors.New("orderbook: invalid order")
	ErrInvalidSide          = errors.New("orderbook: invalid order side")
	ErrCanceledOrder        = errors.New("orderbook: order is canceled")
	ErrInvalidQuantity      = errors.New("orderbook: invalid GetOrder Quantity")
	ErrInvalidPrice         = errors.New("orderbook: invalid GetOrder Price")
	ErrInvalidTif           = errors.New("orderbook: invalid GetOrder time in force")
	ErrOrderExists          = errors.New("orderbook: GetOrder already exists")
	ErrInsufficientQuantity = errors.New("orderbook: insufficient Volume to calculate Price")
)
