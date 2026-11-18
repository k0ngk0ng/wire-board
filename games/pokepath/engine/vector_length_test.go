package engine

import (
	"math"
	"testing"
)

func TestCoreEuclideanLengthHasPortableRoundingAndScale(t *testing.T) {
	for _, scenario := range []struct {
		x, y, expected float64
	}{
		{0, 0, 0},
		{math.Copysign(0, -1), 0, 0},
		{3, 4, 5},
		{-3, 4, 5},
		{4, -3, 5},
		{20, 24, 31.240998703626623},
		{-24, -20, 31.240998703626623},
		{300, 400, 500},
		{3e300, 4e300, 5e300},
		{3e-300, 4e-300, 5e-300},
		{math.Inf(1), math.NaN(), math.Inf(1)},
	} {
		actual := vectorLength(scenario.x, scenario.y)
		if actual != scenario.expected || actual == 0 && math.Signbit(actual) {
			t.Fatalf("vector (%g,%g): got %.17g, want %.17g", scenario.x, scenario.y, actual, scenario.expected)
		}
	}
	if !math.IsNaN(vectorLength(math.NaN(), 1)) || !math.IsNaN(vectorLength(1, math.NaN())) {
		t.Fatal("NaN coordinates did not propagate")
	}
	if !finiteNumber(vectorLength(1e308, 1e308)) || vectorLength(1e-320, 1e-320) <= 0 {
		t.Fatal("finite vector length overflowed or discarded a representable subnormal")
	}
}
