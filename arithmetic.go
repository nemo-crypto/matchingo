package matchingo

import (
	"math"
	"math/bits"

	"github.com/nikolaydubina/fpdecimal"
)

var decimalScales = [...]uint64{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000, 100000000, 1000000000, 10000000000}

// mulDiv uses a 128-bit intermediate. All engine operands are non-negative.
// Saturation is used only for buying capacity, which is bounded by actual liquidity.
func mulDiv(a, b, divisor uint64, ceil bool) (int64, bool) {
	hi, lo := bits.Mul64(a, b)
	if divisor == 0 || hi >= divisor {
		return math.MaxInt64, false
	}
	q, r := bits.Div64(hi, lo, divisor)
	if q > math.MaxInt64 || (ceil && r != 0 && q == math.MaxInt64) {
		return math.MaxInt64, false
	}
	if ceil && r != 0 {
		q++
	}
	return int64(q), true
}

func quoteCost(quantity, price fpdecimal.Decimal, ceil bool) (fpdecimal.Decimal, bool) {
	n, ok := mulDiv(uint64(quantity.Scaled()), uint64(price.Scaled()), decimalScales[fpdecimal.FractionDigits], ceil)
	return fpdecimal.FromIntScaled(n), ok
}
