package game

import (
	"math"
	"slices"
)

// Scenario display coordinates can differ across CPU architectures. Quantize
// to millionths of a map unit; topology IDs already use a coarser 0.001 grid.
// The persisted coordinates are untouched. Equality only compares integers.
func catanCoordinateKey(value float64) (int64, bool) {
	rounded := math.Round(value * 1_000_000)
	if math.IsNaN(rounded) || math.IsInf(rounded, 0) {
		return 0, false
	}
	// Restrict conversion to exactly representable integers, comfortably within
	// int64. Compare the integer exponent instead of float limits.
	_, exponent := math.Frexp(rounded)
	if exponent > 53 {
		return 0, false
	}
	return int64(rounded), true
}

func sameCatanCoordinate(a, b float64) bool {
	left, leftOK := catanCoordinateKey(a)
	right, rightOK := catanCoordinateKey(b)
	return leftOK && rightOK && left == right
}

func sameCatanTile(a, b CatanTile) bool {
	return sameCatanCoordinate(a.X, b.X) && sameCatanCoordinate(a.Y, b.Y) &&
		a.ID == b.ID && a.Resource == b.Resource && a.Number == b.Number &&
		(a.Vertices == nil) == (b.Vertices == nil) && slices.Equal(a.Vertices, b.Vertices)
}

func sameCatanTiles(a, b []CatanTile) bool {
	if len(a) != len(b) || (a == nil) != (b == nil) {
		return false
	}
	for i := range a {
		if !sameCatanTile(a[i], b[i]) {
			return false
		}
	}
	return true
}

func sameCatanVertex(a, b CatanVertex) bool {
	return sameCatanCoordinate(a.X, b.X) && sameCatanCoordinate(a.Y, b.Y) &&
		a.ID == b.ID && a.Harbor == b.Harbor && a.Owner == b.Owner && a.Level == b.Level
}
