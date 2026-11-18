package engine

import "math"

func vectorLength(x, y float64) float64 {
	x, y = math.Abs(x), math.Abs(y)
	if math.IsInf(x, 1) || math.IsInf(y, 1) {
		return math.Inf(1)
	}
	if math.IsNaN(x) || math.IsNaN(y) {
		return math.NaN()
	}
	if x < y {
		x, y = y, x
	}
	if x == 0 {
		return 0
	}
	ratio := y / x
	square := float64(ratio * ratio)
	return x * math.Sqrt(1+square)
}
