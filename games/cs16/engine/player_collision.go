package engine

func traceMovement(state *State, seat int, start, end Vec3, hullIndex int) TraceResult {
	result := TraceWorld(state, start, end, hullIndex)
	if hullIndex < 0 {
		return result
	}
	height := 36 * Unit
	if hullIndex == 1 {
		height = 18 * Unit
	}
	for _, other := range state.Players {
		if other.Seat == seat || !other.Alive {
			continue
		}
		otherHeight := 36 * Unit
		if other.Crouch {
			otherHeight = 18 * Unit
		}
		bounds := Bounds{Min: Vec3{other.X - 32*Unit, other.Y - 32*Unit, other.Z - otherHeight - height}, Max: Vec3{other.X + 32*Unit, other.Y + 32*Unit, other.Z + otherHeight + height}}
		trace := TraceBounds(bounds, start, end)
		if trace.Fraction < result.Fraction {
			result = trace
		}
	}
	return result
}
