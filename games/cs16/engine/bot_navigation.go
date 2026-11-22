package engine

func botPointVisible(state *State, player *Player, point Vec3) bool {
	eye := Eye(*player)
	trace := TraceWorld(state, eye, point, -1)
	return !trace.StartSolid && trace.Fraction >= NormalUnit && !SmokeBlocks(state, eye, point)
}

func botBombDestination(state *State, seat int) (Vec3, bool) {
	player := &state.Players[seat]
	point := bombPosition(state.Bomb)
	if player.Team == "t" {
		return point, true
	}
	visible := botPointVisible(state, player, Vec3{point.X, point.Y, point.Z + 8*Unit})
	if visible {
		return point, true
	}
	if player.BotBombKnown {
		return player.BotBombPoint, true
	}
	if len(state.World.Bombsites) > 0 {
		index := player.BotSearchSite % len(state.World.Bombsites)
		return botPositionBounds(state.World.Bombsites[index]), false
	}
	return Position(*player), false
}

func botHostageDestination(state *State, player *Player) (Vec3, int) {
	position := Position(*player)
	goal := position
	best := int(^uint(0) >> 1)
	for index, hostage := range state.Hostages {
		if hostage.Rescued || player.Team == "ct" && (hostage.Health <= 0 || hostage.Following >= 0 && hostage.Following != player.Seat) {
			continue
		}
		point := Vec3{hostage.X, hostage.Y, hostage.Z}
		if player.Team == "t" && !botPointVisible(state, player, Vec3{point.X, point.Y, point.Z + 40*Unit}) {
			if index >= len(state.World.Hostages) {
				continue
			}
			spawn := state.World.Hostages[index]
			point = Vec3{spawn.X, spawn.Y, spawn.Z}
		} else if hostage.Health <= 0 {
			continue
		}
		distance := Distance(position, point)
		if player.BotDecision > 0 && Distance(point, player.BotGoal) < Unit {
			return point, distance
		}
		if distance < best {
			goal, best = point, distance
		}
	}
	return goal, best
}

func botDestination(state *State, seat int) (Vec3, int, bool) {
	player := &state.Players[seat]
	if state.World.Mode == "elimination" {
		if point, found := MapWeaponGoal(state, seat); found {
			return point, 0, true
		}
		point, found := eliminationPatrolDestination(state, seat)
		return point, 0, found
	}
	if state.World.Mode == "bomb" {
		if state.Bomb.Planted {
			point, _ := botBombDestination(state, seat)
			return point, 0, true
		}
		if state.Bomb.Dropped && player.Team == "t" {
			return bombPosition(state.Bomb), 0, true
		}
		if len(state.World.Bombsites) == 0 {
			return Vec3{}, 0, false
		}
		index := (seat/2 + state.Round - 1) % len(state.World.Bombsites)
		if state.Bomb.Carrier == seat {
			index = (state.Round - 1) % len(state.World.Bombsites)
		}
		return botPositionBounds(state.World.Bombsites[index]), 0, true
	}
	if state.World.Mode == "hostage" {
		mode := 0
		if player.Team == "ct" {
			mode = 1
			for _, hostage := range state.Hostages {
				if hostage.Following == seat && !hostage.Rescued && hostage.Health > 0 && len(state.World.RescueZones) > 0 {
					return botRescueDestination(state, seat), 2, true
				}
			}
		}
		goal, distance := botHostageDestination(state, player)
		return goal, mode, distance != int(^uint(0)>>1)
	}
	return Vec3{}, 0, false
}

func botRescueInterior(world *Map, point Vec3) bool {
	feet := Vec3{point.X, point.Y, point.Z - 36*Unit}
	if world.RescueLegacy {
		for _, spawn := range world.SpawnsCT {
			if Distance(feet, Vec3{spawn.X, spawn.Y, spawn.Z}) < 192*Unit {
				return true
			}
		}
		return false
	}
	for _, zone := range world.RescueZones {
		if objectiveInBounds(feet, zone) && feet.X >= zone.Min.X+64*Unit && feet.X <= zone.Max.X-64*Unit && feet.Y >= zone.Min.Y+64*Unit && feet.Y <= zone.Max.Y-64*Unit {
			return true
		}
	}
	return false
}

func botRescueDestination(state *State, seat int) Vec3 {
	player := &state.Players[seat]
	world := &state.World
	if player.BotDecision > 0 && player.BotRouteMode == 2 && botRescueInterior(world, player.BotGoal) {
		return player.BotGoal
	}
	position := Position(*player)
	for _, hostage := range state.Hostages {
		if hostage.Following == seat && !hostage.Rescued && hostage.Health > 0 {
			position = Vec3{hostage.X, hostage.Y, hostage.Z + 36*Unit}
			break
		}
	}
	start, bestDistance := -1, int(^uint(0)>>1)
	for index, point := range world.Nav {
		if botNodeForbidden(botNodeFlags(world, index), true, true) {
			continue
		}
		distance := Distance(position, point)
		if distance < bestDistance {
			start, bestDistance = index, distance
		}
	}
	best, bestCost := -1, int(^uint(0)>>1)
	if start >= 0 {
		for index, point := range world.Nav {
			if botNodeForbidden(botNodeFlags(world, index), true, true) || !botRescueInterior(world, point) {
				continue
			}
			costs := botCachedCosts(world, index, true, true)
			if len(costs) != len(world.Nav) {
				costs = botGoalCosts(world, index, true, true, botReverseEdges(world))
			}
			if costs[start] < bestCost {
				best, bestCost = index, costs[start]
			}
		}
	}
	if best >= 0 {
		return world.Nav[best]
	}
	goal := position
	bestDistance = int(^uint(0) >> 1)
	for _, zone := range world.RescueZones {
		point := botPositionBounds(zone)
		distance := Distance(position, point)
		if distance < bestDistance {
			goal, bestDistance = point, distance
		}
	}
	return goal
}

func botConnection(world *Map, from, to int) int {
	if from < 0 || from >= len(world.NavEdges) || from >= len(world.NavConnections) {
		return 0
	}
	for edge, node := range world.NavEdges[from] {
		if node == to && edge < len(world.NavConnections[from]) {
			return world.NavConnections[from][edge]
		}
	}
	return 0
}

func botBuildPath(world *Map, position, goal Vec3, mode int) ([]int, int) {
	if len(world.Nav) == 0 || len(world.NavEdges) != len(world.Nav) {
		return nil, -1
	}
	start, target := botNearestNode(world, position), botNearestNode(world, goal)
	if start < 0 || target < 0 {
		return nil, -1
	}
	hostage, ground := mode == 2, mode > 0
	costs := botCachedCosts(world, target, hostage, ground)
	maximum := int(^uint(0) >> 1)
	if len(costs) != len(world.Nav) {
		costs = botGoalCosts(world, target, hostage, ground, botReverseEdges(world))
	}
	if botNodeForbidden(botNodeFlags(world, target), hostage, ground) || costs[start] == maximum {
		target = botReachableTarget(world, start, goal, hostage, ground)
		costs = botCachedCosts(world, target, hostage, ground)
		if len(costs) != len(world.Nav) {
			costs = botGoalCosts(world, target, hostage, ground, botReverseEdges(world))
		}
	}
	path := []int{start}
	current := start
	for current != target && len(path) < 128 {
		next, best := -1, maximum
		for _, node := range world.NavEdges[current] {
			if node < 0 || node >= len(world.Nav) || costs[node] == maximum || botNodeForbidden(botNodeFlags(world, node), hostage, ground) || hostage && botJumpEdge(world, current, node) {
				continue
			}
			if costs[current] != maximum && costs[node] >= costs[current] {
				continue
			}
			cost := costs[node] + Distance(world.Nav[current], world.Nav[node])
			if cost < best || cost == best && (next < 0 || node < next) {
				next, best = node, cost
			}
		}
		if next < 0 {
			break
		}
		path = append(path, next)
		current = next
	}
	return path, start
}

func StepBotNavigation(state *State) {
	if state.Finished || state.Phase != "live" {
		return
	}
	stepBotPerception(state)
	if len(state.World.Nav) == 0 {
		return
	}
	for seat := range state.Players {
		player := &state.Players[seat]
		if !player.Alive {
			player.BotPath = nil
			continue
		}
		goal, mode, valid := botDestination(state, seat)
		if !valid {
			player.BotPath = nil
			continue
		}
		replan := player.BotDecision <= 0 || mode != player.BotRouteMode || Distance(goal, player.BotGoal) > 16*Unit
		if len(player.BotPath) > 0 {
			node := player.BotPath[0]
			replan = replan || node < 0 || node >= len(state.World.Nav)
			if node >= 0 && node < len(state.World.Nav) {
				replan = replan || Distance(Position(*player), state.World.Nav[node]) > 512*Unit || state.Frame-player.BotDecision > 500 && Distance(Position(*player), state.World.Nav[node]) > 96*Unit
			}
		} else if state.Frame-player.BotDecision >= 100 && Distance(Position(*player), goal) > 96*Unit {
			replan = true
		}
		if replan {
			player.BotPath, player.BotTarget = botBuildPath(&state.World, Position(*player), goal, mode)
			player.BotGoal, player.BotRouteMode, player.BotDecision = goal, mode, state.Frame
		}
		for len(player.BotPath) > 0 {
			node := player.BotPath[0]
			if node < 0 || node >= len(state.World.Nav) {
				player.BotPath = nil
				break
			}
			point := state.World.Nav[node]
			position := Position(*player)
			if player.Grounded {
				if player.Crouch && botNodeFlags(&state.World, node)&botCrouch == 0 {
					position.Z += 18 * Unit
				} else if !player.Crouch && botNodeFlags(&state.World, node)&botCrouch != 0 {
					position.Z -= 18 * Unit
				}
			}
			if botHorizontalDistance(position, point) > 8*Unit || objectiveAbs(position.Z-point.Z) > 20*Unit {
				break
			}
			player.BotTarget = node
			player.BotPath = player.BotPath[1:]
			player.BotDecision = state.Frame
		}
	}
}

func botCommittedWaypoint(state *State, seat int, goal Vec3, mode int) (Vec3, int, int, bool) {
	if seat < 0 || seat >= len(state.Players) {
		return Vec3{}, 0, 0, false
	}
	player := &state.Players[seat]
	if player.BotDecision <= 0 || player.BotRouteMode != mode || len(player.BotPath) == 0 || Distance(goal, player.BotGoal) > 16*Unit {
		return Vec3{}, 0, 0, false
	}
	node := player.BotPath[0]
	if node < 0 || node >= len(state.World.Nav) {
		return Vec3{}, 0, 0, false
	}
	return state.World.Nav[node], botNodeFlags(&state.World, node), botConnection(&state.World, player.BotTarget, node), true
}
