package engine

const (
	deathNoThrow = iota
	deathForward
	deathBackward
	deathHitVelocity
	deathHitVelocityNoLift
)

func applyDeathImpulse(state *State, player *Player, attacker, damage, hitgroup int, weapon string, source Vec3, hasSource bool) {
	seed := uint32(state.Seed) ^ uint32(state.Frame)*2246822519 ^ uint32(player.Seat+1)*3266489917 ^ uint32(player.Life)*668265263
	if seed == 0 {
		seed = 1
	}
	random := func(minimum, maximum int) int {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		return minimum + int(seed%uint32(maximum-minimum+1))
	}
	mode := deathNoThrow
	if player.Crouch {
		mode = deathBackward
	} else if hasSource && (weapon == "c4" || weapon == "hegrenade") {
		blast := Sub(Position(*player), source)
		distance := Distance(Position(*player), source)
		horizontal, vertical := 0, 0
		if weapon == "c4" {
			remaining := max(0, 2300*Unit-distance)
			horizontal, vertical = remaining/4, remaining*4/11
		} else {
			horizontal = max(0, 500*Unit-distance)
			vertical = max(0, 350*Unit-distance) * 3 / 2
		}
		player.DeathVX, player.DeathVY = 0, 0
		if distance > 0 {
			player.DeathVX = int(int64(blast.X) * int64(horizontal) / int64(distance))
			player.DeathVY = int(int64(blast.Y) * int64(horizontal) / int64(distance))
		}
		player.DeathVZ = vertical
		return
	} else if hitgroup == 1 {
		choice := random(0, 8)
		if damage > 60 {
			choice++
		}
		switch choice {
		case 1, 2:
			mode = deathBackward
		case 3, 4:
			mode = deathForward
		case 5, 6:
			mode = deathHitVelocity
		}
	} else if hitgroup == 5 {
		mode = deathHitVelocityNoLift
		if random(0, 1) != 0 {
			mode = deathHitVelocity
		}
	}
	if mode == deathNoThrow {
		return
	}
	direction, speed, vertical := Direction(player.Yaw, player.Pitch), 0, 0
	if mode == deathForward || mode == deathBackward {
		speed, vertical = random(100*Unit, 200*Unit), random(50*Unit, 100*Unit)
		if mode == deathBackward {
			speed = -speed
		}
	} else {
		if attacker < 0 || attacker >= len(state.Players) {
			return
		}
		direction = Direction(state.Players[attacker].Yaw, state.Players[attacker].Pitch)
		speed = random(200*Unit, 300*Unit)
		vertical = int(int64(direction.Z) * int64(speed) / NormalUnit)
		if mode == deathHitVelocity {
			vertical = random(200*Unit, 300*Unit)
		}
	}
	player.DeathVX = int(int64(direction.X) * int64(speed) / NormalUnit)
	player.DeathVY = int(int64(direction.Y) * int64(speed) / NormalUnit)
	player.DeathVZ = vertical
}

func droppedItemVelocity(player *Player) Vec3 {
	velocity := Vec3{player.VX, player.VY, player.VZ}
	if !player.Alive && player.DeathAt > 0 {
		velocity = Vec3{player.DeathVX, player.DeathVY, player.DeathVZ}
	}
	return Scale(velocity, 3, 4)
}
