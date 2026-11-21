package engine

func eliminationSpawnGoal(world *Map, start int, spawn Spawn) (Vec3, bool) {
	point := Vec3{spawn.X, spawn.Y, spawn.Z}
	if len(world.Nav) == 0 {
		return point, true
	}
	node := botNearestNode(world, point)
	if node < 0 || start < 0 {
		return Vec3{}, false
	}
	costs := botCachedCosts(world, node, false, false)
	if len(costs) != len(world.Nav) {
		costs = botGoalCosts(world, node, false, false, botReverseEdges(world))
	}
	if costs[start] == int(^uint(0)>>1) {
		return Vec3{}, false
	}
	return world.Nav[node], true
}

func eliminationPatrolDestination(state *State, seat int) (Vec3, bool) {
	player := &state.Players[seat]
	position := Position(*player)
	if player.BotDecision > 0 && len(player.BotPath) > 0 && Distance(position, player.BotGoal) > 48*Unit {
		return player.BotGoal, true
	}
	own, opponent := state.World.SpawnsT, state.World.SpawnsCT
	if player.Team == "ct" {
		own, opponent = opponent, own
	}
	start := botNearestNode(&state.World, position)
	useOwn := false
	if player.BotDecision > 0 {
		for _, spawn := range opponent {
			if point, found := eliminationSpawnGoal(&state.World, start, spawn); found && Distance(point, player.BotGoal) < Unit {
				useOwn = true
				break
			}
		}
	}
	groups := [][]Spawn{opponent, own}
	if useOwn {
		groups[0], groups[1] = groups[1], groups[0]
	}
	for _, spawns := range groups {
		if len(spawns) == 0 {
			continue
		}
		initial := (seat/2 + state.Round - 1) % len(spawns)
		for offset := range len(spawns) {
			if point, found := eliminationSpawnGoal(&state.World, start, spawns[(initial+offset)%len(spawns)]); found && Distance(position, point) > 48*Unit {
				return point, true
			}
		}
	}
	return Vec3{}, false
}
