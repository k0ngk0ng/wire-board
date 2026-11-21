package engine

var recoveryOffsets = makeRecoveryOffsets()

func makeRecoveryOffsets() []Vec3 {
	result := make([]Vec3, 0, 54)
	for _, z := range []int{-Unit / 8, 0, Unit / 8} {
		result = append(result, Vec3{Z: z})
	}
	for _, y := range []int{-Unit / 8, 0, Unit / 8} {
		result = append(result, Vec3{Y: y})
	}
	for _, x := range []int{-Unit / 8, 0, Unit / 8} {
		result = append(result, Vec3{X: x})
	}
	for _, x := range []int{-Unit / 8, Unit / 8} {
		for _, y := range []int{-Unit / 8, Unit / 8} {
			for _, z := range []int{-Unit / 8, Unit / 8} {
				result = append(result, Vec3{x, y, z})
			}
		}
	}
	for _, z := range []int{0, Unit, 6 * Unit} {
		result = append(result, Vec3{Z: z})
	}
	for _, y := range []int{-2 * Unit, 0, 2 * Unit} {
		result = append(result, Vec3{Y: y})
	}
	for _, x := range []int{-2 * Unit, 0, 2 * Unit} {
		result = append(result, Vec3{X: x})
	}
	for _, z := range []int{0, Unit, 6 * Unit} {
		for _, x := range []int{-2 * Unit, 0, 2 * Unit} {
			for _, y := range []int{-2 * Unit, 0, 2 * Unit} {
				result = append(result, Vec3{x, y, z})
			}
		}
	}
	return result
}

func recoverPosition(state *State, player *Player) {
	hull := 0
	if player.Crouch {
		hull = 1
	}
	origin := Position(*player)
	if !traceMovement(state, player.Seat, origin, origin, hull).StartSolid {
		return
	}
	for _, offset := range recoveryOffsets {
		candidate := Add(origin, offset)
		if !traceMovement(state, player.Seat, candidate, candidate, hull).StartSolid {
			player.X, player.Y, player.Z = candidate.X, candidate.Y, candidate.Z
			return
		}
	}
}
