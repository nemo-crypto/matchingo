package matchingo

import (
	"math"
	"math/big"
	"testing"
)

func FuzzMulDiv(f *testing.F) {
	for _, s := range [][3]uint64{{1, 1, 1}, {math.MaxInt64, math.MaxInt64, 1000}, {100000000000, 100000000, 1000}, {0, 0, 0}, {math.MaxUint64, math.MaxUint64, math.MaxUint64}} {
		f.Add(s[0], s[1], s[2], true)
		f.Add(s[0], s[1], s[2], false)
	}
	f.Fuzz(func(t *testing.T, a, b, d uint64, ceil bool) {
		got, ok := mulDiv(a, b, d, ceil)
		if d == 0 {
			if ok {
				t.Fatal("zero divisor")
			}
			return
		}
		product := new(big.Int).Mul(new(big.Int).SetUint64(a), new(big.Int).SetUint64(b))
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(product, new(big.Int).SetUint64(d), r)
		if ceil && r.Sign() != 0 {
			q.Add(q, big.NewInt(1))
		}
		wantOK := q.IsInt64()
		if ok != wantOK || (ok && got != q.Int64()) || (!ok && got != math.MaxInt64) {
			t.Fatalf("%d * %d / %d ceil=%v: got %d,%v want %s,%v", a, b, d, ceil, got, ok, q, wantOK)
		}
	})
}
