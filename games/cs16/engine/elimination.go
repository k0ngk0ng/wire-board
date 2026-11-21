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
	goals := eliminationPatrolGoals(&state.World, start, [][]Spawn{opponent, own})
	if len(goals) == 0 {
		return Vec3{}, false
	}
	initial := (seat/2 + state.Round - 1) * 2 % len(goals)
	if player.BotDecision > 0 {
		for index, point := range goals {
			if Distance(point, player.BotGoal) < Unit {
				initial = (index + 1) % len(goals)
				break
			}
		}
	}
	for offset := range len(goals) {
		point := goals[(initial+offset)%len(goals)]
		if Distance(position, point) > 48*Unit {
			return point, true
		}
	}
	return Vec3{}, false
}

func eliminationPatrolGoals(world *Map, start int, groups [][]Spawn) []Vec3 {
	goals := []Vec3{}
	maximum := 0
	for _, spawns := range groups {
		maximum = max(maximum, len(spawns))
	}
	for index := range maximum {
		for _, spawns := range groups {
			if index >= len(spawns) {
				continue
			}
			offset := index/2 + index%2*((len(spawns)+1)/2)
			point, found := eliminationSpawnGoal(world, start, spawns[offset])
			if !found {
				continue
			}
			duplicate := false
			for _, goal := range goals {
				if goal == point {
					duplicate = true
					break
				}
			}
			if !duplicate {
				goals = append(goals, point)
			}
		}
	}
	return goals
}
