package engine

import "math"

func polygonContains(point Point, vertices []Point) bool {
	inside := false
	previous := vertices[len(vertices)-1]
	for _, current := range vertices {
		cross := (point.X-previous.X)*(current.Y-previous.Y) - (point.Y-previous.Y)*(current.X-previous.X)
		if math.Abs(cross) < 1e-9 && point.X >= min(previous.X, current.X) && point.X <= max(previous.X, current.X) && point.Y >= min(previous.Y, current.Y) && point.Y <= max(previous.Y, current.Y) {
			return true
		}
		if (previous.Y > point.Y) != (current.Y > point.Y) {
			x := previous.X + (point.Y-previous.Y)*(current.X-previous.X)/(current.Y-previous.Y)
			if point.X < x {
				inside = !inside
			}
		}
		previous = current
	}
	return inside
}

func combatRangeType(species Species, pokemon Pokemon, profile AttackProfile) string {
	shape := species.RangeType
	if pokemon.Item == "starPiece" && (shape == "cross" || shape == "xShape") {
		shape = "star"
	}
	if profile.RangeShape != "" {
		shape = profile.RangeShape
	}
	return shape
}

func combatRangeHalfWidth(state *State, pokemon Pokemon, shape string) float64 {
	width := 24.0
	if pokemon.Item == "wideLens" {
		width *= 2
	}
	width *= WorldCrossWidth(state)
	if shape == "xShape" && pokemon.Item == "wideLens" {
		width += 24
	}
	return width
}
