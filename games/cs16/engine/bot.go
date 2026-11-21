package engine

import "container/heap"

const (
	botUseButton = 1 << 0
	botCrouch    = 1 << 2
	botLadder    = 1 << 5
	botNoHostage = 1 << 8
	botJump      = 1 << 0
)

type botFrontierItem struct {
	node  int
	score int
	cost  int
}
type botFrontier []botFrontierItem

func (frontier botFrontier) Len() int { return len(frontier) }
func (frontier botFrontier) Less(i, j int) bool {
	if frontier[i].score == frontier[j].score {
		return frontier[i].node < frontier[j].node
	}
	return frontier[i].score < frontier[j].score
}
func (frontier botFrontier) Swap(i, j int)   { frontier[i], frontier[j] = frontier[j], frontier[i] }
func (frontier *botFrontier) Push(value any) { *frontier = append(*frontier, value.(botFrontierItem)) }
func (frontier *botFrontier) Pop() any {
	old := *frontier
	value := old[len(old)-1]
	*frontier = old[:len(old)-1]
	return value
}

func botAngleDifference(a, b int) int     { value := (a - b + 32768) & 65535; return value - 32768 }
func botHorizontalDistance(a, b Vec3) int { return Distance(Vec3{a.X, a.Y, 0}, Vec3{b.X, b.Y, 0}) }
func botPositionBounds(bounds Bounds) Vec3 {
	return Vec3{bounds.Min.X + (bounds.Max.X-bounds.Min.X)/2, bounds.Min.Y + (bounds.Max.Y-bounds.Min.Y)/2, bounds.Min.Z + (bounds.Max.Z-bounds.Min.Z)/2}
}

func BotPurchase(state *State, seat int) string {
	if seat < 0 || seat >= len(state.Players) || state.Finished || state.Phase != "freeze" && state.Phase != "live" || state.Frame > state.BuyEndsAt {
		return ""
	}
	p := &state.Players[seat]
	if !p.Alive {
		return ""
	}
	if !CanBuy(state, seat) {
		return ""
	}
	primary := false
	for _, item := range p.Inventory {
		if weapon, ok := WeaponByID(item.ID); ok && weapon.Slot == 1 {
			primary = true
			break
		}
	}
	if !primary {
		choices := []string{"ak47", "galil", "mac10"}
		if p.Team == "ct" {
			choices = []string{"m4a1", "famas", "tmp"}
		}
		for _, choice := range choices {
			if weapon, ok := WeaponByID(choice); ok && p.Money >= weapon.Price {
				return choice
			}
		}
	}
	if p.Armor < 100 && p.Money >= 1000 {
		return "vesthelm"
	}
	if p.Armor < 100 && p.Money >= 650 {
		return "vest"
	}
	if p.Team == "ct" && state.World.Mode == "bomb" && !p.Defuser && p.Money >= 200 {
		return "defuser"
	}
	for _, item := range p.Inventory {
		weapon, ok := WeaponByID(item.ID)
		if !ok || weapon.AmmoType == "" || item.Reserve >= weapon.Reserve || p.Money < weapon.AmmoPrice {
			continue
		}
		if weapon.Slot == 1 {
			return "primammo"
		}
		if weapon.Slot == 2 {
			return "secammo"
		}
	}
	if primary && !objectiveHasWeapon(p, "hegrenade") && p.Money >= 300 {
		return "hegrenade"
	}
	if primary && !objectiveHasWeapon(p, "flashbang") && p.Money >= 200 {
		return "flashbang"
	}
	return ""
}

func BotInput(state *State, seat int) (input Input) {
	if seat < 0 || seat >= len(state.Players) {
		return Input{}
	}
	p := &state.Players[seat]
	input = Input{Seq: p.ReceivedSeq + 1, Life: p.Life, ViewFrame: state.Frame, Yaw: p.Yaw, Pitch: p.Pitch}
	defer func() { botAvoidPlayers(state, seat, &input); botAvoidFriendlyFire(state, seat, &input) }()
	if !p.Alive || state.Finished || state.Phase != "live" {
		return input
	}
	if (state.Frame+int64(seat))/10 == (p.Input.QueuedFrame+int64(seat))/10 && p.AppliedSeq > 0 {
		input = p.Input
		input.Seq, input.Life, input.ViewFrame = p.ReceivedSeq+1, p.Life, state.Frame
		return input
	}
	position := objectivePosition(p)
	bombKnown := false
	if state.World.Mode == "bomb" && state.Bomb.Planted && p.Team == "ct" {
		_, bombKnown = botBombDestination(state, seat)
	}
	if state.World.Mode == "bomb" && state.Bomb.Planted && p.Team == "ct" && bombKnown && objectiveNear(position, bombPosition(state.Bomb), 70*Unit) {
		input.Use = true
		return input
	}
	closest, enemyDistance := -1, 8192*Unit
	eye := Eye(*p)
	if state.Frame >= p.FlashUntil {
		for i := range state.Players {
			target := &state.Players[i]
			if !target.Alive || target.Team == p.Team {
				continue
			}
			aim := Vec3{target.X, target.Y, target.Z + 16*Unit}
			distance := Distance(eye, aim)
			if distance >= enemyDistance {
				continue
			}
			if objectiveAbs(botAngleDifference(AngleTo(target.X-p.X, target.Y-p.Y), p.Yaw)) > 8192 {
				continue
			}
			if !Visible(state, seat, i) {
				continue
			}
			closest, enemyDistance = i, distance
		}
	}
	if closest >= 0 {
		target := &state.Players[closest]
		dx, dy, dz := target.X-p.X, target.Y-p.Y, target.Z+16*Unit-eye.Z
		input.Yaw = AngleTo(dx, dy)
		input.Pitch = -botAngleDifference(AngleTo(botHorizontalDistance(eye, Vec3{target.X, target.Y, target.Z}), dz), 0)
		input.Pitch = max(-16000, min(16000, input.Pitch))
		input.Select = objectiveBestWeapon(p)
		weapon, ok := WeaponByID(input.Select)
		input.Fire = ok && (weapon.ID != "knife" || enemyDistance < 70*Unit)
		if !weapon.Automatic && weapon.ID != "knife" && state.Frame%40 >= 20 {
			input.Fire = false
		}
		input.Crouch = enemyDistance > 500*Unit && state.Frame%200 < 150
		if p.Ammo == 0 && p.Reserve > 0 {
			input.Reload = true
			input.Fire = false
		}
		if enemyDistance > 900*Unit && ok && (weapon.ID == "awp" || weapon.ID == "scout" || weapon.ID == "sg550" || weapon.ID == "g3sg1" || weapon.ID == "aug" || weapon.ID == "sg552") {
			for _, item := range p.Inventory {
				if item.ID == weapon.ID && item.Zoom == 0 {
					input.Secondary = true
					input.Fire = false
				}
			}
		}
		if enemyDistance > 600*Unit {
			input.MoveY = 200
		}
		if weapon.ID == "knife" {
			input.MoveY = 1000
		}
		return input
	}
	goal := position
	if state.World.Mode == "bomb" {
		if state.Bomb.Planted {
			goal, _ = botBombDestination(state, seat)
			if p.Team == "t" && botHorizontalDistance(position, goal) < 240*Unit {
				input.Yaw = (p.Yaw + 2048) & 65535
				return input
			}
		} else if state.Bomb.Dropped && p.Team == "t" {
			goal = bombPosition(state.Bomb)
		} else if len(state.World.Bombsites) > 0 {
			index := (seat/2 + state.Round - 1) % len(state.World.Bombsites)
			if state.Bomb.Carrier == seat {
				index = (state.Round - 1) % len(state.World.Bombsites)
			}
			goal = botPositionBounds(state.World.Bombsites[index])
			if state.Bomb.Carrier == seat {
				for _, site := range state.World.Bombsites {
					if objectiveInBounds(position, site) && p.Grounded {
						input.Select = "c4"
						input.Fire = true
						return input
					}
				}
			}
		}
	} else if state.World.Mode == "hostage" {
		following := false
		for _, hostage := range state.Hostages {
			if hostage.Following == seat && !hostage.Rescued && hostage.Health > 0 {
				following = true
				break
			}
		}
		if p.Team == "ct" && following && len(state.World.RescueZones) > 0 {
			goal = botRescueDestination(state, seat)
			for _, hostage := range state.Hostages {
				if hostage.Following == seat && !hostage.Rescued && botHorizontalDistance(position, Vec3{hostage.X, hostage.Y, hostage.Z}) > 180*Unit {
					return input
				}
			}
		} else {
			var best int
			goal, best = botHostageDestination(state, p)
			if p.Team == "ct" && best < 75*Unit {
				input.Yaw = AngleTo(goal.X-position.X, goal.Y-position.Y)
				input.Pitch = 0
				input.Use = state.Frame%20 < 10
				return input
			}
			if p.Team == "t" && best < 220*Unit {
				input.Yaw = (p.Yaw + 2048) & 65535
				return input
			}
		}
	}
	if p.Ammo == 0 && p.Reserve > 0 {
		input.Reload = true
	}
	input.Select = objectiveBestWeapon(p)
	return botNavigate(state, seat, goal, state.World.Mode == "hostage" && p.Team == "ct", input)
}

func botNavigate(state *State, seat int, goal Vec3, escort bool, input Input) Input {
	if seat < 0 || seat >= len(state.Players) {
		return input
	}
	position := Position(state.Players[seat])
	carryingHostages := false
	for _, hostage := range state.Hostages {
		if hostage.Following == seat && !hostage.Rescued && hostage.Health > 0 {
			carryingHostages = true
			break
		}
	}
	mode := 0
	if escort {
		mode = 1
	}
	if carryingHostages {
		mode = 2
	}
	waypoint, flags, connection, committed := botCommittedWaypoint(state, seat, goal, mode)
	if !committed {
		waypoint, flags, connection = botWaypointWithRules(state, position, goal, carryingHostages, escort || carryingHostages)
	}
	input.Yaw = AngleTo(waypoint.X-position.X, waypoint.Y-position.Y)
	input.Pitch = 0
	input.MoveY = 1000
	if flags&botCrouch != 0 {
		input.Crouch = true
	}
	if flags&botUseButton != 0 {
		input.Use = true
	}
	jumping := !carryingHostages && (connection&botJump != 0 || !escort && waypoint.Z-position.Z > 20*Unit)
	if jumping {
		input.Jump = state.Frame%20 < 10
		input.Crouch = !state.Players[seat].Grounded
	}
	if flags&botLadder != 0 {
		input.Pitch = -8192
		if waypoint.Z < position.Z {
			input.Pitch = 8192
			input.MoveY = -1000
		}
		input.Jump = false
	}
	if state.Players[seat].Grounded && !jumping && flags&botLadder == 0 {
		botArrivalSteering(&state.Players[seat], position, waypoint, &input)
	}
	forward := Direction(input.Yaw, 0)
	probe := position
	hull := 0
	if input.Crouch {
		hull = 1
		if !state.Players[seat].Crouch {
			probe.Z -= 18 * Unit
		}
	} else if state.Players[seat].Crouch {
		probe.Z += 18 * Unit
	}
	next := Vec3{probe.X + forward.X*36*Unit/NormalUnit, probe.Y + forward.Y*36*Unit/NormalUnit, probe.Z}
	blocked := TraceWorld(state, probe, next, hull).Fraction < NormalUnit
	if blocked {
		input.Use = true
		if !escort && flags&botLadder == 0 && !jumping && !input.Crouch && !state.Players[seat].Crouch {
			input.Jump = state.Frame%40 < 20
		}
		if !escort && state.Frame%200 >= 100 {
			input.MoveX = 1000
			if seat%2 == 1 {
				input.MoveX = -1000
			}
		}
		for i, brush := range state.World.Brushes {
			if brush.Kind != "func_breakable" || i >= len(state.Brushes) || state.Brushes[i].Health <= 0 {
				continue
			}
			if objectiveNear(position, botPositionBounds(brush.Bounds), 100*Unit) {
				input.Fire = true
				if weapon, ok := WeaponByID(input.Select); ok && !weapon.Automatic && state.Frame%40 >= 20 {
					input.Fire = false
				}
				break
			}
		}
	}
	return input
}

func botArrivalSteering(player *Player, position, target Vec3, input *Input) {
	speed := WeaponMoveSpeed(player)
	if player.Crouch || input.Crouch {
		speed /= 3
	}
	if speed <= 0 {
		return
	}
	distance := botHorizontalDistance(position, target)
	desiredSpeed := min(speed, max(60*Unit, distance*4))
	forward := Direction(input.Yaw, 0)
	wanted := Sub(Scale(forward, desiredSpeed*2, NormalUnit), Vec3{player.VX, player.VY, 0})
	if length := Length2D(wanted.X, wanted.Y); length > speed {
		wanted = Scale(wanted, speed, length)
	}
	right := Vec3{forward.Y, -forward.X, 0}
	input.MoveY = Clamp(Dot(wanted, forward)*1000/speed, -1000, 1000)
	input.MoveX = Clamp(Dot(wanted, right)*1000/speed, -1000, 1000)
}

func botAvoidPlayers(state *State, seat int, input *Input) {
	if input.MoveY <= 0 || seat < 0 || seat >= len(state.Players) {
		return
	}
	player := &state.Players[seat]
	forward := Direction(input.Yaw, 0)
	for index := range state.Players {
		other := &state.Players[index]
		if index == seat || !other.Alive || objectiveAbs(other.Z-player.Z) > 60*Unit {
			continue
		}
		dx, dy := other.X-player.X, other.Y-player.Y
		if int64(dx)*int64(dx)+int64(dy)*int64(dy) > int64(64*Unit)*int64(64*Unit) {
			continue
		}
		ahead := (int64(dx)*int64(forward.X) + int64(dy)*int64(forward.Y)) / NormalUnit
		if ahead <= 0 {
			continue
		}
		if TraceWorld(state, objectivePosition(player), objectivePosition(other), -1).Fraction < NormalUnit {
			continue
		}
		side := (int64(dx)*int64(forward.Y) - int64(dy)*int64(forward.X)) / NormalUnit
		input.MoveY = -250
		input.MoveX = 1000
		if side > 0 {
			input.MoveX = -1000
		}
		return
	}
}

func botNearestNode(world *Map, point Vec3) int {
	nearest, best := -1, int64(1<<62)
	for i, node := range world.Nav {
		dx, dy, dz := int64(node.X-point.X), int64(node.Y-point.Y), int64(node.Z-point.Z)
		cost := dx*dx + dy*dy + dz*dz*4
		if cost < best {
			nearest, best = i, cost
		}
	}
	return nearest
}

func botWaypoint(state *State, position, goal Vec3) (Vec3, int, int) {
	return botWaypointFor(state, position, goal, false)
}

func botWaypointFor(state *State, position, goal Vec3, hostage bool) (Vec3, int, int) {
	return botWaypointWithRules(state, position, goal, hostage, hostage)
}

func botNodeForbidden(flags int, hostage, ground bool) bool {
	return hostage && flags&(botNoHostage|botCrouch) != 0 || ground && flags&botLadder != 0
}

func botJumpEdge(world *Map, from, to int) bool {
	if from < 0 || from >= len(world.NavEdges) || from >= len(world.NavConnections) {
		return false
	}
	for edge, node := range world.NavEdges[from] {
		if node == to && edge < len(world.NavConnections[from]) {
			return world.NavConnections[from][edge]&botJump != 0
		}
	}
	return false
}

func botWaypointWithRules(state *State, position, goal Vec3, hostage, ground bool) (Vec3, int, int) {
	world := &state.World
	if len(world.Nav) == 0 || len(world.NavEdges) != len(world.Nav) {
		return goal, 0, 0
	}
	start, target := botNearestNode(world, position), botNearestNode(world, goal)
	if start < 0 || target < 0 {
		return goal, 0, 0
	}
	table := botCachedCosts(world, target, hostage, ground)
	if botNodeForbidden(botNodeFlags(world, target), hostage, ground) || len(table) == len(world.Nav) && table[start] == int(^uint(0)>>1) {
		target = botReachableTarget(world, start, goal, hostage, ground)
	}
	if waypoint, flags, connection, ok := botVisibleConnector(state, position, start, target, hostage, ground); ok {
		return waypoint, flags, connection
	}
	if start == target {
		if botHorizontalDistance(position, goal) < 32*Unit {
			return goal, 0, 0
		}
		return world.Nav[target], botNodeFlags(world, target), 0
	}
	costs := make([]int, len(world.Nav))
	previous := make([]int, len(world.Nav))
	connections := make([]int, len(world.Nav))
	for i := range costs {
		costs[i] = int(^uint(0) >> 1)
		previous[i] = -1
	}
	costs[start] = 0
	frontier := &botFrontier{{node: start, cost: 0, score: Distance(world.Nav[start], world.Nav[target])}}
	heap.Init(frontier)
	for frontier.Len() > 0 {
		current := heap.Pop(frontier).(botFrontierItem)
		if current.cost != costs[current.node] {
			continue
		}
		if current.node == target {
			break
		}
		for edge, next := range world.NavEdges[current.node] {
			if next < 0 || next >= len(world.Nav) {
				continue
			}
			if botNodeForbidden(botNodeFlags(world, next), hostage, ground) {
				continue
			}
			if hostage && botJumpEdge(world, current.node, next) {
				continue
			}
			cost := current.cost + Distance(world.Nav[current.node], world.Nav[next])
			if cost >= costs[next] {
				continue
			}
			costs[next], previous[next] = cost, current.node
			if current.node < len(world.NavConnections) && edge < len(world.NavConnections[current.node]) {
				connections[next] = world.NavConnections[current.node][edge]
			}
			heap.Push(frontier, botFrontierItem{node: next, cost: cost, score: cost + Distance(world.Nav[next], world.Nav[target])})
		}
	}
	if previous[target] < 0 {
		return world.Nav[start], botNodeFlags(world, start), 0
	}
	next := target
	for previous[next] != start && previous[next] >= 0 {
		next = previous[next]
	}
	if botHorizontalDistance(position, world.Nav[start]) > 12*Unit || objectiveAbs(position.Z-world.Nav[start].Z) > 40*Unit {
		trace := TraceWorld(state, position, world.Nav[next], 0)
		if trace.Fraction < NormalUnit {
			return world.Nav[start], botNodeFlags(world, start), 0
		}
	}
	return world.Nav[next], botNodeFlags(world, next), connections[next]
}

func botVisibleConnector(state *State, position Vec3, start, target int, hostage, ground bool) (Vec3, int, int, bool) {
	world := &state.World
	maximum := int(^uint(0) >> 1)
	costs := botCachedCosts(world, target, hostage, ground)
	if len(costs) != len(world.Nav) {
		costs = botGoalCosts(world, target, hostage, ground, botReverseEdges(world))
	}
	best, bestScore := -1, maximum
	for index, point := range world.Nav {
		if costs[index] == maximum || botNodeForbidden(botNodeFlags(world, index), hostage, ground) {
			continue
		}
		if index != start && costs[index] >= costs[start] {
			continue
		}
		distance := Distance(position, point)
		if index == start && index != target && botHorizontalDistance(position, point) < 12*Unit && objectiveAbs(position.Z-point.Z) <= 40*Unit {
			continue
		}
		ladder := botNodeFlags(world, index)&botLadder != 0
		if distance > 320*Unit && !(ladder && botHorizontalDistance(position, point) < 40*Unit && distance < 1200*Unit) {
			continue
		}
		if objectiveAbs(point.Z-position.Z) > 90*Unit && !ladder {
			continue
		}
		hull := 0
		if botNodeFlags(world, index)&botCrouch != 0 {
			hull = 1
		}
		trace := TraceWorld(state, position, point, hull)
		if trace.StartSolid && hull == 0 {
			raised := Vec3{position.X, position.Y, position.Z + 18*Unit}
			trace = TraceWorld(state, raised, point, hull)
		}
		if trace.StartSolid || trace.Fraction < NormalUnit {
			continue
		}
		score := costs[index] + distance
		if score < bestScore || score == bestScore && best >= 0 && costs[index] < costs[best] {
			best, bestScore = index, score
		}
	}
	if best < 0 {
		return Vec3{}, 0, 0, false
	}
	connection := 0
	for edge, node := range world.NavEdges[start] {
		if node == best && start < len(world.NavConnections) && edge < len(world.NavConnections[start]) {
			connection = world.NavConnections[start][edge]
			break
		}
	}
	return world.Nav[best], botNodeFlags(world, best), connection, true
}

func botReverseEdges(world *Map) [][]int {
	reverse := make([][]int, len(world.Nav))
	for from, edges := range world.NavEdges {
		for _, to := range edges {
			if to >= 0 && to < len(reverse) {
				reverse[to] = append(reverse[to], from)
			}
		}
	}
	return reverse
}

func botGoalCosts(world *Map, target int, hostage, ground bool, reverse [][]int) []int {
	maximum := int(^uint(0) >> 1)
	costs := make([]int, len(world.Nav))
	for index := range costs {
		costs[index] = maximum
	}
	if target < 0 || target >= len(costs) {
		return costs
	}
	costs[target] = 0
	frontier := &botFrontier{{node: target, cost: 0}}
	heap.Init(frontier)
	for frontier.Len() > 0 {
		current := heap.Pop(frontier).(botFrontierItem)
		if current.cost != costs[current.node] {
			continue
		}
		for _, next := range reverse[current.node] {
			if botNodeForbidden(botNodeFlags(world, next), hostage, ground) {
				continue
			}
			if hostage && botJumpEdge(world, next, current.node) {
				continue
			}
			cost := current.cost + Distance(world.Nav[current.node], world.Nav[next])
			if cost >= costs[next] {
				continue
			}
			costs[next] = cost
			heap.Push(frontier, botFrontierItem{node: next, cost: cost, score: cost})
		}
	}
	return costs
}

func prepareNavigation(world *Map) {
	reverse := botReverseEdges(world)
	world.NavCosts = make([][]int, len(world.Nav))
	world.NavHostageCosts = make([][]int, len(world.Nav))
	if world.Mode == "hostage" {
		world.NavGroundCosts = make([][]int, len(world.Nav))
	}
	for target := range world.Nav {
		world.NavCosts[target] = botGoalCosts(world, target, false, false, reverse)
		world.NavHostageCosts[target] = botGoalCosts(world, target, true, true, reverse)
		if world.Mode == "hostage" {
			world.NavGroundCosts[target] = botGoalCosts(world, target, false, true, reverse)
		}
	}
}

func botCachedCosts(world *Map, target int, hostage, ground bool) []int {
	if target < 0 {
		return nil
	}
	if hostage && target < len(world.NavHostageCosts) {
		return world.NavHostageCosts[target]
	}
	if ground && target < len(world.NavGroundCosts) {
		return world.NavGroundCosts[target]
	}
	if !hostage && !ground && target < len(world.NavCosts) {
		return world.NavCosts[target]
	}
	return nil
}

func botReachableTarget(world *Map, start int, goal Vec3, hostage, ground bool) int {
	seen := make([]bool, len(world.Nav))
	queue := []int{start}
	seen[start] = true
	best, bestDistance := start, Distance(world.Nav[start], goal)
	for cursor := 0; cursor < len(queue); cursor++ {
		node := queue[cursor]
		if distance := Distance(world.Nav[node], goal); distance < bestDistance && !botNodeForbidden(botNodeFlags(world, node), hostage, ground) {
			best, bestDistance = node, distance
		}
		for _, next := range world.NavEdges[node] {
			if next < 0 || next >= len(seen) || seen[next] || botNodeForbidden(botNodeFlags(world, next), hostage, ground) || hostage && botJumpEdge(world, node, next) {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return best
}

func botNodeFlags(world *Map, index int) int {
	if index >= 0 && index < len(world.NavFlags) {
		return world.NavFlags[index]
	}
	return 0
}
