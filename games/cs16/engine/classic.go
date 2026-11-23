package engine

func InBombZone(state *State, seat int) bool {
	if seat < 0 || seat >= len(state.Players) || !state.Players[seat].Alive || state.World.Mode != "bomb" {
		return false
	}
	for _, site := range state.World.Bombsites {
		if objectiveInBounds(Position(state.Players[seat]), site) {
			return true
		}
	}
	return false
}

func stepFlashlight(state *State, player *Player) {
	if player.FlashlightAt == 0 {
		if player.FlashlightCharge == 0 {
			player.FlashlightCharge = 100
		}
		player.FlashlightAt = state.Frame + 20
	}
	if !player.Alive {
		player.Flashlight = false
		return
	}
	if player.Input.Flashlight && !player.Previous.Flashlight {
		player.Flashlight = !player.Flashlight && player.FlashlightCharge > 0
		player.FlashlightAt = state.Frame + 20
		if player.Flashlight {
			player.FlashlightAt = state.Frame + 120
		}
		Emit(state, Event{Kind: "flashlight", Seat: player.Seat, Value: player.FlashlightCharge, X: player.X, Y: player.Y, Z: player.Z})
	}
	if state.Frame < player.FlashlightAt {
		return
	}
	if player.Flashlight {
		player.FlashlightCharge = max(0, player.FlashlightCharge-1)
		player.FlashlightAt = state.Frame + 120
		if player.FlashlightCharge == 0 {
			player.Flashlight = false
			player.FlashlightAt = state.Frame + 20
			Emit(state, Event{Kind: "flashlight", Seat: player.Seat, X: player.X, Y: player.Y, Z: player.Z})
		}
	} else {
		player.FlashlightCharge = min(100, player.FlashlightCharge+1)
		player.FlashlightAt = state.Frame + 20
	}
}

func stepFootsteps(state *State, player *Player, before Vec3) {
	if state.Phase != "live" && state.Phase != "round_end" || !player.Alive || !player.Grounded || player.Input.Walk || player.Crouch || Length2D(player.VX, player.VY) <= 150*Unit || player.X == before.X && player.Y == before.Y || state.Frame < player.StepAt {
		return
	}
	player.StepAt = state.Frame + 30
	player.StepIndex = (player.StepIndex + 1) % 4
	Emit(state, Event{Kind: "step", Seat: player.Seat, Value: player.StepIndex, X: player.X, Y: player.Y, Z: player.Z, Life: player.Life})
}

func stepDroppedWeapons(state *State) {
	for index := range state.Drops {
		drop := &state.Drops[index]
		if !drop.Moving || drop.SpawnID > 0 {
			continue
		}
		drop.VZ -= 800 * Unit / FPS
		start := Vec3{drop.X, drop.Y, drop.Z}
		end := Vec3{drop.X + drop.VX/FPS, drop.Y + drop.VY/FPS, drop.Z + drop.VZ/FPS}
		trace := TraceWorld(state, start, end, -1)
		drop.X, drop.Y, drop.Z = trace.End.X, trace.End.Y, trace.End.Z
		if trace.StartSolid || trace.AllSolid {
			drop.Moving, drop.VX, drop.VY, drop.VZ = false, 0, 0, 0
			continue
		}
		if trace.Fraction < NormalUnit {
			if trace.Normal.Z >= NormalUnit*7/10 {
				drop.Moving, drop.VX, drop.VY, drop.VZ = false, 0, 0, 0
			} else {
				velocity := Vec3{drop.VX, drop.VY, drop.VZ}
				normal := Dot(velocity, trace.Normal)
				velocity = Sub(velocity, Scale(trace.Normal, normal, NormalUnit))
				drop.VX, drop.VY, drop.VZ = velocity.X, velocity.Y, velocity.Z
			}
		}
	}
}

func placeDroppedBomb(state *State, player *Player) {
	point := Position(*player)
	if player.Alive {
		point = combatAddRay(point, Direction(player.Yaw, 0), 40*Unit)
	}
	trace := TraceWorld(state, Position(*player), point, -1)
	point = trace.End
	floor := TraceWorld(state, point, Vec3{point.X, point.Y, point.Z - 4096*Unit}, -1)
	if !floor.StartSolid && floor.Fraction < NormalUnit {
		point = floor.End
	}
	state.Bomb.Carrier, state.Bomb.Dropped = -1, true
	state.Bomb.X, state.Bomb.Y, state.Bomb.Z = point.X, point.Y, point.Z
	state.Bomb.PickupAt = state.Frame + 30
	Emit(state, Event{Kind: "bomb_dropped", Seat: player.Seat, Target: -1, Weapon: "c4", X: point.X, Y: point.Y, Z: point.Z})
}
