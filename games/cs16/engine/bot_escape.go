package engine

func stepBotProgress(state *State, seat int, memory *BotMemory) {
	player := &state.Players[seat]
	position := Position(*player)
	if player.Input.MoveX == 0 && player.Input.MoveY == 0 || botHorizontalDistance(position, memory.ProgressPoint) >= 24*Unit || Abs(position.Z-memory.ProgressPoint.Z) >= 48*Unit {
		memory.ProgressPoint, memory.ProgressAt = position, state.Frame
	}
	if memory.EscapeUntil > 0 && (memory.EscapeUntil <= state.Frame || botHorizontalDistance(position, memory.EscapePoint) <= 16*Unit) {
		memory.EscapePoint, memory.EscapeUntil = Vec3{}, 0
		memory.ProgressPoint, memory.ProgressAt = position, state.Frame
	}
	if memory.EscapeUntil > state.Frame || state.Frame-memory.ProgressAt < 100 {
		return
	}
	point, found := botEscapeDestination(state, seat, memory.EscapeCount)
	memory.ProgressPoint, memory.ProgressAt = position, state.Frame
	memory.EscapeCount++
	if found {
		memory.EscapePoint, memory.EscapeUntil = point, state.Frame+100
		player.BotPath, player.BotDecision = nil, 0
	}
}

func botEscapeDestination(state *State, seat, attempt int) (Vec3, bool) {
	player := &state.Players[seat]
	position := Position(*player)
	hull := 0
	if player.Crouch {
		hull = 1
	}
	offsets := [...]int{16384, -16384, 32768, 8192, -8192, 24576, -24576, 0}
	for offset := range offsets {
		yaw := player.Yaw + offsets[(offset+attempt)%len(offsets)]
		point := combatAddRay(position, Direction(yaw, 0), 96*Unit)
		trace := TraceWorld(state, position, point, hull)
		if trace.StartSolid || trace.Fraction < NormalUnit {
			continue
		}
		floor := TraceWorld(state, point, Add(point, Vec3{Z: -64 * Unit}), hull)
		if floor.StartSolid || floor.Fraction >= NormalUnit || floor.Normal.Z < NormalUnit*7/10 {
			continue
		}
		return floor.End, true
	}
	return Vec3{}, false
}

func botEscapeWaypoint(state *State, seat int) (Vec3, bool) {
	memory := botMemory(state, seat)
	if memory == nil || memory.EscapeUntil <= state.Frame {
		return Vec3{}, false
	}
	return memory.EscapePoint, true
}
