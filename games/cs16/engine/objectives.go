package engine

func objectivePosition(player *Player) Vec3 { return Vec3{player.X, player.Y, player.Z} }
func objectiveAbs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
func objectiveInBounds(position Vec3, bounds Bounds) bool {
	return position.X >= bounds.Min.X && position.X <= bounds.Max.X && position.Y >= bounds.Min.Y && position.Y <= bounds.Max.Y && position.Z >= bounds.Min.Z && position.Z <= bounds.Max.Z
}
func objectiveNear(a, b Vec3, radius int) bool { return Distance(a, b) <= radius }
func objectiveStill(player *Player) bool {
	return player.Input.MoveX == 0 && player.Input.MoveY == 0 && objectiveAbs(player.VX) <= 5*Unit && objectiveAbs(player.VY) <= 5*Unit && player.Grounded
}

func removeC4(player *Player) {
	items := player.Inventory[:0]
	for _, item := range player.Inventory {
		if item.ID != "c4" {
			items = append(items, item)
		}
	}
	player.Inventory = items
	if player.Weapon == "c4" {
		player.Weapon = objectiveBestWeapon(player)
		SyncWeapon(player)
	}
}

func bombPosition(bomb Bomb) Vec3 { return Vec3{bomb.X, bomb.Y, bomb.Z} }

func stepBomb(state *State) {
	if !state.Bomb.Planted {
		if state.Bomb.Carrier >= 0 && state.Bomb.Carrier < len(state.Players) {
			carrier := &state.Players[state.Bomb.Carrier]
			if !carrier.Alive || carrier.Input.Drop && carrier.Weapon == "c4" {
				state.Bomb.X, state.Bomb.Y, state.Bomb.Z = carrier.X, carrier.Y, carrier.Z-28*Unit
				state.Bomb.Carrier, state.Bomb.Dropped = -1, true
				removeC4(carrier)
				carrier.Action, carrier.ActionSince = "", 0
				Emit(state, Event{Kind: "bomb_dropped", Seat: carrier.Seat, Target: -1, Weapon: "c4", X: state.Bomb.X, Y: state.Bomb.Y, Z: state.Bomb.Z})
			} else {
				inside := false
				for _, site := range state.World.Bombsites {
					if objectiveInBounds(objectivePosition(carrier), site) {
						inside = true
						break
					}
				}
				planting := carrier.Team == "t" && carrier.Weapon == "c4" && carrier.Input.Fire && objectiveStill(carrier) && inside
				if !planting {
					if carrier.Action == "plant" {
						carrier.Action, carrier.ActionSince = "", 0
					}
				} else {
					if carrier.Action != "plant" {
						carrier.Action, carrier.ActionSince = "plant", state.Frame
						Emit(state, Event{Kind: "plant_start", Seat: carrier.Seat, Target: -1, Weapon: "c4", Value: 3 * FPS})
					}
					if state.Frame-carrier.ActionSince >= 3*FPS {
						start := objectivePosition(carrier)
						trace := TraceWorld(state, start, Vec3{start.X, start.Y, start.Z - 40*Unit}, -1)
						state.Bomb.X, state.Bomb.Y, state.Bomb.Z = trace.End.X, trace.End.Y, trace.End.Z+Unit
						state.Bomb.Planted, state.Bomb.Dropped, state.Bomb.Carrier = true, false, -1
						state.Bomb.ExplodesAt, state.Bomb.Defuser, state.Bomb.DefuseAt = state.Frame+45*FPS, -1, 0
						objectiveMoney(carrier, 300)
						removeC4(carrier)
						carrier.Action, carrier.ActionSince = "", 0
						Emit(state, Event{Kind: "bomb_planted", Seat: carrier.Seat, Target: -1, Weapon: "c4", X: state.Bomb.X, Y: state.Bomb.Y, Z: state.Bomb.Z, Value: 45 * FPS})
					}
				}
			}
		}
		if state.Bomb.Dropped {
			for seat := range state.Players {
				player := &state.Players[seat]
				if player.Alive && player.Team == "t" && objectiveNear(objectivePosition(player), bombPosition(state.Bomb), 64*Unit) {
					state.Bomb.Carrier, state.Bomb.Dropped = seat, false
					if !objectiveHasWeapon(player, "c4") {
						player.Inventory = append(player.Inventory, WeaponState{ID: "c4", Ammo: 1})
					}
					Emit(state, Event{Kind: "bomb_pickup", Seat: seat, Target: -1, Weapon: "c4", X: player.X, Y: player.Y, Z: player.Z})
					break
				}
			}
		}
	}
	if !state.Bomb.Planted {
		return
	}
	validDefuser := -1
	for seat := range state.Players {
		player := &state.Players[seat]
		eligible := player.Alive && player.Team == "ct" && player.Input.Use && objectiveStill(player) && objectiveNear(objectivePosition(player), bombPosition(state.Bomb), 75*Unit)
		if eligible {
			trace := TraceWorld(state, Eye(*player), Vec3{state.Bomb.X, state.Bomb.Y, state.Bomb.Z + 8*Unit}, -1)
			eligible = !trace.StartSolid && trace.Fraction >= NormalUnit
		}
		if !eligible {
			if player.Action == "defuse" {
				player.Action, player.ActionSince = "", 0
			}
			continue
		}
		if state.Bomb.Defuser == seat || validDefuser < 0 {
			validDefuser = seat
		}
	}
	if validDefuser < 0 {
		state.Bomb.Defuser, state.Bomb.DefuseAt = -1, 0
	} else if state.Bomb.Defuser != validDefuser && state.Frame < state.Bomb.ExplodesAt {
		state.Bomb.Defuser = validDefuser
		length := int64(10 * FPS)
		if state.Players[validDefuser].Defuser {
			length = 5 * FPS
		}
		state.Bomb.DefuseAt = state.Frame + length
		state.Players[validDefuser].Action, state.Players[validDefuser].ActionSince = "defuse", state.Frame
		Emit(state, Event{Kind: "defuse_start", Seat: validDefuser, Target: -1, Weapon: "c4", Value: int(length)})
	}
	if validDefuser >= 0 && state.Bomb.Defuser == validDefuser && state.Bomb.DefuseAt > 0 && state.Frame >= state.Bomb.DefuseAt && state.Bomb.DefuseAt <= state.Bomb.ExplodesAt {
		objectiveMoney(&state.Players[validDefuser], 300)
		Emit(state, Event{Kind: "bomb_defused", Seat: validDefuser, Target: -1, Weapon: "c4", X: state.Bomb.X, Y: state.Bomb.Y, Z: state.Bomb.Z})
		RoundWon(state, "ct", "bomb_defused")
		return
	}
	if state.Frame >= state.Bomb.ExplodesAt {
		Emit(state, Event{Kind: "bomb_exploded", Seat: -1, Target: -1, Weapon: "c4", X: state.Bomb.X, Y: state.Bomb.Y, Z: state.Bomb.Z})
		for seat := range state.Players {
			p := &state.Players[seat]
			if !p.Alive {
				continue
			}
			distance := Distance(objectivePosition(p), bombPosition(state.Bomb))
			if distance < 1750*Unit {
				damage := max(0, 500-distance*500/(1750*Unit))
				DamagePlayer(state, -1, seat, damage, 500, 0, "c4")
			}
		}
		RoundWon(state, "t", "bomb_exploded")
	}
}

func stepHostages(state *State) {
	for seat := range state.Players {
		player := &state.Players[seat]
		if !player.Alive || player.Team != "ct" || !player.Input.Use || player.Previous.Use {
			continue
		}
		closest, distance := -1, 80*Unit
		for i, hostage := range state.Hostages {
			if hostage.Rescued || hostage.Health <= 0 {
				continue
			}
			point := Vec3{hostage.X, hostage.Y, hostage.Z}
			d := Distance(objectivePosition(player), point)
			eye := Eye(*player)
			usePoint := Vec3{hostage.X, hostage.Y, Clamp(eye.Z, hostage.Z, hostage.Z+62*Unit)}
			offset := Sub(usePoint, eye)
			forward := Direction(player.Yaw, player.Pitch)
			if d > distance || int64(Dot(offset, forward))*1000 < int64(Distance(Vec3{}, offset))*800 || TraceWorld(state, eye, usePoint, -1).Fraction < NormalUnit {
				continue
			}
			closest, distance = i, d
		}
		if closest >= 0 {
			hostage := &state.Hostages[closest]
			if hostage.Following == seat {
				hostage.Following = -1
			} else {
				hostage.Following = seat
				if !hostage.Touched {
					objectiveMoney(player, 150)
					hostage.Touched = true
				}
			}
			Emit(state, Event{Kind: "hostage_use", Seat: seat, Target: hostage.ID, X: hostage.X, Y: hostage.Y, Z: hostage.Z})
		}
	}
	rescued, total, alive := 0, len(state.Hostages), 0
	for i := range state.Hostages {
		hostage := &state.Hostages[i]
		if hostage.Rescued {
			rescued++
			continue
		}
		if hostage.Health <= 0 {
			continue
		}
		alive++
		moved := false
		if hostage.Following >= 0 && hostage.Following < len(state.Players) {
			leader := &state.Players[hostage.Following]
			if !leader.Alive || leader.Team != "ct" {
				hostage.Following = -1
			} else {
				position := Vec3{hostage.X, hostage.Y, hostage.Z}
				target := objectivePosition(leader)
				target.Z -= 36 * Unit
				if leader.Crouch {
					target.Z += 18 * Unit
				}
				if Distance(position, target) > 45*Unit {
					next := objectiveFollowRoute(state, hostage, target)
					hostage.X, hostage.Y, hostage.Z = next.X, next.Y, next.Z
					moved = true
				} else {
					objectiveSettleHostage(state, hostage)
					moved = true
				}
				if objectiveRescueReached(state.World, Vec3{hostage.X, hostage.Y, hostage.Z}) {
					hostage.Rescued = true
					rescued++
					alive--
					objectiveMoney(leader, 1000)
					Emit(state, Event{Kind: "hostage_rescued", Seat: leader.Seat, Target: hostage.ID, X: hostage.X, Y: hostage.Y, Z: hostage.Z})
				}
			}
		}
		if !moved {
			objectiveSettleHostage(state, hostage)
		}
	}
	if total > 0 && alive == 0 && rescued*2 >= total {
		RoundWon(state, "ct", "hostages_rescued")
	}
}

func objectiveRescueReached(world Map, feet Vec3) bool {
	if world.RescueLegacy {
		for _, spawn := range world.SpawnsCT {
			if Distance(feet, Vec3{spawn.X, spawn.Y, spawn.Z}) < 256*Unit {
				return true
			}
		}
		return false
	}
	for _, zone := range world.RescueZones {
		if objectiveInBounds(feet, zone) {
			return true
		}
	}
	return false
}

func objectiveHostageRoute(state *State, hostage *Hostage) *HostageRoute {
	index := -1
	for i := range state.HostageRoutes {
		if state.HostageRoutes[i].ID == hostage.ID {
			index = i
			break
		}
	}
	if index < 0 {
		state.HostageRoutes = append(state.HostageRoutes, HostageRoute{ID: hostage.ID, Following: hostage.Following, From: -1})
		index = len(state.HostageRoutes) - 1
	}
	return &state.HostageRoutes[index]
}

func objectiveSettleHostage(state *State, hostage *Hostage) {
	route := objectiveHostageRoute(state, hostage)
	if route.Following != hostage.Following {
		route.Path = nil
		route.Following = hostage.Following
	}
	position := Vec3{hostage.X, hostage.Y, hostage.Z}
	next := objectiveHostageMove(state, position, position, &route.VZ)
	hostage.X, hostage.Y, hostage.Z = next.X, next.Y, next.Z
}

func objectiveFollowRoute(state *State, hostage *Hostage, target Vec3) Vec3 {
	position := Vec3{hostage.X, hostage.Y, hostage.Z}
	route := objectiveHostageRoute(state, hostage)
	if len(state.World.Nav) == 0 {
		return objectiveHostageMove(state, position, target, &route.VZ)
	}
	center := Vec3{position.X, position.Y, position.Z + 36*Unit}
	goal := Vec3{target.X, target.Y, target.Z + 36*Unit}
	replan := route.PlannedAt <= 0 || route.Following != hostage.Following || len(route.Path) == 0 && Distance(route.Goal, goal) > 128*Unit
	if len(route.Path) > 0 {
		node := route.Path[0]
		replan = replan || node < 0 || node >= len(state.World.Nav)
		if node >= 0 && node < len(state.World.Nav) {
			replan = replan || Distance(center, state.World.Nav[node]) > 512*Unit
		}
	} else if Distance(center, goal) > 64*Unit {
		replan = true
	}
	if replan {
		route.Path, route.From = botBuildPath(&state.World, center, goal, 2)
		route.Goal, route.PlannedAt, route.Following = goal, state.Frame, hostage.Following
	}
	for len(route.Path) > 0 {
		node := route.Path[0]
		if node < 0 || node >= len(state.World.Nav) {
			route.Path = nil
			break
		}
		point := state.World.Nav[node]
		if botHorizontalDistance(center, point) > 8*Unit || objectiveAbs(center.Z-point.Z) > 20*Unit {
			break
		}
		route.From = node
		route.Path = route.Path[1:]
	}
	if len(route.Path) > 0 {
		point := state.World.Nav[route.Path[0]]
		target = Vec3{point.X, point.Y, point.Z - 36*Unit}
	}
	return objectiveHostageMove(state, position, target, &route.VZ)
}

func objectiveHostageMove(state *State, position, target Vec3, vertical *int) Vec3 {
	physics := *state
	physics.Players = nil
	center := Vec3{position.X, position.Y, position.Z + 36*Unit}
	dx, dy := target.X-position.X, target.Y-position.Y
	length := Length2D(dx, dy)
	speed := min(180*Unit, length*FPS)
	proxy := Player{Seat: -1, Alive: true, X: center.X, Y: center.Y, Z: center.Z, VZ: *vertical}
	if length > 0 {
		proxy.VX = dx * speed / length
		proxy.VY = dy * speed / length
	}
	groundCheck(&physics, &proxy)
	grounded := proxy.Grounded
	if !grounded {
		proxy.VZ -= 800 * Unit / FPS / 2
	}
	origin := Position(proxy)
	velocity := Vec3{proxy.VX, proxy.VY, proxy.VZ}
	slideMove(&physics, &proxy)
	if grounded && Length2D(proxy.X-origin.X, proxy.Y-origin.Y) < Length2D(velocity.X, velocity.Y)/FPS {
		flat := proxy
		up := TraceWorld(&physics, origin, Vec3{origin.X, origin.Y, origin.Z + 18*Unit}, 0)
		if !up.StartSolid {
			proxy.X, proxy.Y, proxy.Z = up.End.X, up.End.Y, up.End.Z
			proxy.VX, proxy.VY, proxy.VZ = velocity.X, velocity.Y, velocity.Z
			slideMove(&physics, &proxy)
			down := TraceWorld(&physics, Position(proxy), Vec3{proxy.X, proxy.Y, proxy.Z - 18*Unit}, 0)
			if !down.StartSolid && down.Normal.Z >= NormalUnit*7/10 && Length2D(proxy.X-origin.X, proxy.Y-origin.Y) > Length2D(flat.X-origin.X, flat.Y-origin.Y) {
				proxy.X, proxy.Y, proxy.Z = down.End.X, down.End.Y, down.End.Z
			} else {
				proxy = flat
			}
		}
	}
	groundCheck(&physics, &proxy)
	if !proxy.Grounded {
		proxy.VZ -= 800 * Unit / FPS / 2
	}
	*vertical = proxy.VZ
	return Vec3{proxy.X, proxy.Y, proxy.Z - 36*Unit}
}
