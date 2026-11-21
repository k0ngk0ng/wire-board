package engine

func PredictMotion(world Map, player Player, input Input, frames int) Player {
	return PredictMotionWithBrushes(world, player, input, frames, nil)
}

func PredictMotionWithBrushes(world Map, player Player, input Input, frames int, brushes []BrushState) Player {
	return PredictMotionWithDynamics(world, player, input, frames, brushes, nil)
}

func PredictMotionWithDynamics(world Map, player Player, input Input, frames int, brushes []BrushState, players []Player) Player {
	state := State{World: world, Phase: "live", Brushes: brushes, Players: players}
	frames = Clamp(frames, 0, 20)
	for range frames {
		player.Previous = player.Input
		player.Input = input
		movePlayer(&state, &player)
		if input.Select != "" && input.Select != player.Weapon {
			for _, item := range player.Inventory {
				if item.ID == input.Select {
					player.Weapon = item.ID
					SyncWeapon(&player)
					break
				}
			}
		}
	}
	return player
}

func groundCheck(state *State, player *Player) TraceResult {
	hull := 0
	if player.Crouch {
		hull = 1
	}
	position := Position(*player)
	trace := traceMovement(state, player.Seat, position, Add(position, Vec3{Z: -2 * Unit}), hull)
	player.Grounded = player.VZ <= 0 && trace.Fraction < NormalUnit && trace.Normal.Z >= NormalUnit*7/10 && !trace.StartSolid
	if player.Grounded {
		player.X, player.Y, player.Z = trace.End.X, trace.End.Y, trace.End.Z
		if player.VZ < 0 {
			player.VZ = 0
		}
	}
	return trace
}

func movePlayer(state *State, player *Player) {
	if !player.Alive {
		return
	}
	input := player.Input
	player.LandingSpeed = 0
	player.Yaw = ((input.Yaw % 65536) + 65536) % 65536
	player.Pitch = Clamp(input.Pitch, -16000, 16000)
	if state.Phase != "live" {
		player.VX, player.VY, player.VZ = 0, 0, 0
		return
	}
	if input.Crouch && !player.Crouch {
		player.Crouch = true
		if player.Grounded {
			player.Z -= 18 * Unit
		}
	} else if !input.Crouch && player.Crouch {
		standing := Position(*player)
		if player.Grounded {
			standing.Z += 18 * Unit
		}
		if !traceMovement(state, player.Seat, standing, standing, 0).StartSolid {
			player.Crouch = false
			player.Z = standing.Z
		}
	}
	recoverPosition(state, player)
	wasGrounded, fallingVelocity := player.Grounded, player.VZ
	groundCheck(state, player)
	if !wasGrounded && player.Grounded {
		player.LandingSpeed = max(0, -fallingVelocity)
	}
	player.WaterLevel = waterLevel(state.World, *player)
	ladder := false
	for _, bounds := range state.World.Ladders {
		expanded := bounds
		expanded.Min = Sub(expanded.Min, Vec3{16 * Unit, 16 * Unit, 36 * Unit})
		expanded.Max = Add(expanded.Max, Vec3{16 * Unit, 16 * Unit, 36 * Unit})
		if Contains(expanded, Position(*player)) {
			ladder = true
			break
		}
	}
	if ladder && !input.Jump {
		sy, cy := SinCos(player.Yaw)
		player.VX = int((int64(input.MoveX)*int64(sy)*200 + int64(input.MoveY)*int64(cy)*64) * Unit / (1000 * NormalUnit))
		player.VY = int((-int64(input.MoveX)*int64(cy)*200 + int64(input.MoveY)*int64(sy)*64) * Unit / (1000 * NormalUnit))
		player.VZ = input.MoveY * 200 * Unit / 1000
		player.Grounded = false
		slideMove(state, player)
		return
	}
	if player.WaterLevel >= 2 {
		swimMove(state, player)
		return
	}
	if player.Grounded {
		speed := Length2D(player.VX, player.VY)
		if speed > 0 {
			drop := max(speed, 100*Unit) * 4 / FPS
			scale := max(0, speed-drop)
			player.VX = player.VX * scale / speed
			player.VY = player.VY * scale / speed
		}
	}
	speed := WeaponMoveSpeed(player)
	if input.Walk {
		speed = speed * 52 / 100
	}
	if player.Crouch {
		speed = speed / 3
	}
	magnitude := Length2D(input.MoveX, input.MoveY)
	if magnitude > 0 {
		moveX, moveY := input.MoveX, input.MoveY
		if magnitude > 1000 {
			moveX = moveX * 1000 / magnitude
			moveY = moveY * 1000 / magnitude
			magnitude = 1000
		}
		sy, cy := SinCos(player.Yaw)
		wish := Vec3{int((int64(cy)*int64(moveY) + int64(sy)*int64(moveX)) / 1000), int((int64(sy)*int64(moveY) - int64(cy)*int64(moveX)) / 1000), 0}
		wishLength := Length2D(wish.X, wish.Y)
		if wishLength > 0 {
			wish = Scale(wish, NormalUnit, wishLength)
			wishSpeed := speed * magnitude / 1000
			current := Dot(Vec3{player.VX, player.VY, 0}, wish)
			limit := wishSpeed
			if !player.Grounded {
				limit = min(limit, 30*Unit)
			}
			add := limit - current
			if add > 0 {
				acceleration := min(add, 10*wishSpeed/FPS)
				player.VX += int(int64(wish.X) * int64(acceleration) / NormalUnit)
				player.VY += int(int64(wish.Y) * int64(acceleration) / NormalUnit)
			}
		}
	}
	if player.Grounded && input.Jump && !player.Previous.Jump {
		player.VZ = Sqrt(int64(2*800*45) * Unit * Unit)
		player.Grounded = false
	}
	if !player.Grounded {
		player.VZ -= 800 * Unit / FPS / 2
	}
	origin := Position(*player)
	velocity := Vec3{player.VX, player.VY, player.VZ}
	fallingVelocity = player.VZ
	slideMove(state, player)
	if player.Grounded {
		flat := *player
		blocked := Distance(origin, Position(*player)) < Length2D(velocity.X, velocity.Y)/FPS
		if blocked {
			player.X, player.Y, player.Z = origin.X, origin.Y, origin.Z
			player.VX, player.VY, player.VZ = velocity.X, velocity.Y, velocity.Z
			hull := 0
			if player.Crouch {
				hull = 1
			}
			up := traceMovement(state, player.Seat, origin, Add(origin, Vec3{Z: 18 * Unit}), hull)
			if !up.StartSolid {
				player.X, player.Y, player.Z = up.End.X, up.End.Y, up.End.Z
				slideMove(state, player)
				down := traceMovement(state, player.Seat, Position(*player), Add(Position(*player), Vec3{Z: -18 * Unit}), hull)
				if down.Normal.Z >= NormalUnit*7/10 && !down.StartSolid {
					player.X, player.Y, player.Z = down.End.X, down.End.Y, down.End.Z
					flatDistance := Length2D(flat.X-origin.X, flat.Y-origin.Y)
					stepDistance := Length2D(player.X-origin.X, player.Y-origin.Y)
					if stepDistance <= flatDistance {
						*player = flat
					}
				} else {
					*player = flat
				}
			} else {
				*player = flat
			}
		}
	}
	if !player.Grounded {
		player.VZ -= 800 * Unit / FPS / 2
	}
	groundCheck(state, player)
	if !wasGrounded && player.Grounded {
		player.LandingSpeed = max(player.LandingSpeed, -fallingVelocity)
	}
}

func waterLevel(world Map, player Player) int {
	hull := mapHull(world, -1)
	height := 36 * Unit
	if player.Crouch {
		height = 18 * Unit
	}
	level := 0
	for _, position := range []Vec3{{player.X, player.Y, player.Z - height + Unit}, Position(player), Eye(player)} {
		contents := PointContents(hull, position)
		if contents <= ContentsWater && contents >= -5 {
			level++
		} else {
			break
		}
	}
	return level
}

func swimMove(state *State, player *Player) {
	input := player.Input
	forward := Direction(player.Yaw, player.Pitch)
	sine, cosine := SinCos(player.Yaw)
	wish := Vec3{int((int64(forward.X)*int64(input.MoveY) + int64(sine)*int64(input.MoveX)) / 1000), int((int64(forward.Y)*int64(input.MoveY) - int64(cosine)*int64(input.MoveX)) / 1000), int(int64(forward.Z) * int64(input.MoveY) / 1000)}
	speed := WeaponMoveSpeed(player)
	wish = Scale(wish, speed, NormalUnit)
	length := Distance(Vec3{}, wish)
	if input.Jump {
		player.VZ = 100 * Unit
	}
	if input.MoveX == 0 && input.MoveY == 0 {
		wish.Z -= 60 * Unit
	}
	length = Distance(Vec3{}, wish)
	wishSpeed := min(length, speed) * 80 / 100
	velocity := Vec3{player.VX, player.VY, player.VZ}
	currentSpeed := Distance(Vec3{}, velocity)
	newSpeed := max(0, currentSpeed-currentSpeed*4/FPS)
	if currentSpeed > 0 {
		velocity = Scale(velocity, newSpeed, currentSpeed)
	}
	if wishSpeed > newSpeed && length > 0 {
		acceleration := min(wishSpeed-newSpeed, 10*wishSpeed/FPS)
		velocity = Add(velocity, Scale(wish, acceleration, length))
	}
	player.VX, player.VY, player.VZ = velocity.X, velocity.Y, velocity.Z
	player.Grounded = false
	slideMove(state, player)
}

func slideMove(state *State, player *Player) {
	hull := 0
	if player.Crouch {
		hull = 1
	}
	remaining := NormalUnit
	planes := make([]Vec3, 0, 5)
	original := Vec3{player.VX, player.VY, player.VZ}
	for bump := 0; bump < 4; bump++ {
		position := Position(*player)
		velocity := Vec3{player.VX, player.VY, player.VZ}
		end := Add(position, Scale(velocity, remaining, FPS*NormalUnit))
		trace := traceMovement(state, player.Seat, position, end, hull)
		if trace.AllSolid {
			player.VX, player.VY, player.VZ = 0, 0, 0
			return
		}
		if trace.Fraction > 0 {
			player.X, player.Y, player.Z = trace.End.X, trace.End.Y, trace.End.Z
		}
		if trace.Fraction == NormalUnit {
			break
		}
		remaining = int(int64(remaining) * int64(NormalUnit-trace.Fraction) / NormalUnit)
		if len(planes) >= 5 {
			player.VX, player.VY, player.VZ = 0, 0, 0
			break
		}
		planes = append(planes, trace.Normal)
		valid := false
		for _, plane := range planes {
			candidate := clipVelocity(velocity, plane)
			valid = true
			for _, other := range planes {
				if Dot(candidate, other) < -1 {
					valid = false
					break
				}
			}
			if valid {
				player.VX, player.VY, player.VZ = candidate.X, candidate.Y, candidate.Z
				break
			}
		}
		if !valid {
			if len(planes) == 2 {
				a, b := planes[0], planes[1]
				crease := Vec3{int((int64(a.Y)*int64(b.Z) - int64(a.Z)*int64(b.Y)) / NormalUnit), int((int64(a.Z)*int64(b.X) - int64(a.X)*int64(b.Z)) / NormalUnit), int((int64(a.X)*int64(b.Y) - int64(a.Y)*int64(b.X)) / NormalUnit)}
				length := Distance(Vec3{}, crease)
				if length > 0 {
					crease = Scale(crease, NormalUnit, length)
					candidate := Scale(crease, Dot(velocity, crease), NormalUnit)
					player.VX, player.VY, player.VZ = candidate.X, candidate.Y, candidate.Z
				} else {
					player.VX, player.VY, player.VZ = 0, 0, 0
				}
			} else {
				player.VX, player.VY, player.VZ = 0, 0, 0
			}
		}
		if Dot(Vec3{player.VX, player.VY, player.VZ}, original) <= 0 {
			player.VX, player.VY, player.VZ = 0, 0, 0
			break
		}
	}
}

func clipVelocity(velocity, normal Vec3) Vec3 {
	backoff := Dot(velocity, normal)
	result := Sub(velocity, Scale(normal, backoff, NormalUnit))
	if Abs(result.X) < Unit/10 {
		result.X = 0
	}
	if Abs(result.Y) < Unit/10 {
		result.Y = 0
	}
	if Abs(result.Z) < Unit/10 {
		result.Z = 0
	}
	return result
}
