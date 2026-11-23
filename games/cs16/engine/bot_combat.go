package engine

func botVisibleEnemy(state *State, seat int) (int, int) {
	player := &state.Players[seat]
	closest, distance := -1, 8192*Unit
	if state.Frame < player.FlashUntil {
		return closest, distance
	}
	for index, target := range state.Players {
		if !target.Alive || target.Team == player.Team || objectiveAbs(botAngleDifference(AngleTo(target.X-player.X, target.Y-player.Y), player.Yaw)) > 8192 {
			continue
		}
		next := Distance(Eye(*player), Eye(target))
		if next < distance && Visible(state, seat, index) {
			closest, distance = index, next
		}
	}
	return closest, distance
}

func stepBotPerception(state *State) {
	if state.Frame%10 != 0 {
		return
	}
	for seat := range state.Players {
		player := &state.Players[seat]
		stepBotMemory(state, seat)
		if !player.Alive {
			player.BotEnemy, player.BotSeenAt = -1, 0
			continue
		}
		if player.BotAimSeed == 0 {
			player.BotAimSeed = uint32(state.Seed) ^ uint32((seat+1)*1103515245) ^ uint32(player.Life*12345)
			if player.BotAimSeed == 0 {
				player.BotAimSeed = 1
			}
		}
		enemy, _ := botVisibleEnemy(state, seat)
		if enemy != player.BotEnemy || enemy >= 0 && player.BotSeenAt <= 0 {
			player.BotEnemy, player.BotSeenAt = enemy, state.Frame
		}
		if enemy < 0 {
			player.BotSeenAt = 0
		}
		if state.World.Mode != "bomb" || !state.Bomb.Planted {
			player.BotBombKnown = false
			continue
		}
		point := bombPosition(state.Bomb)
		if player.Team == "t" || botPointVisible(state, player, Vec3{point.X, point.Y, point.Z + 8*Unit}) || Distance(Position(*player), point) <= 1000*Unit {
			player.BotBombKnown, player.BotBombPoint = true, point
		}
		if player.Team == "ct" && !player.BotBombKnown && len(state.World.Bombsites) > 1 {
			index := player.BotSearchSite % len(state.World.Bombsites)
			goal := botPositionBounds(state.World.Bombsites[index])
			if botHorizontalDistance(Position(*player), goal) < 96*Unit && objectiveAbs(player.Z-goal.Z) < 100*Unit {
				player.BotSearchSite = (index + 1) % len(state.World.Bombsites)
			}
		}
	}
}

func botAimNoise(seed uint32, axis int, bucket int64) int {
	value := seed ^ uint32(axis+1)*2246822519 ^ uint32(bucket)*3266489917
	value ^= value << 13
	value ^= value >> 17
	value ^= value << 5
	return int(value%20001) - 10000
}

func botChestPoint(player Player) Vec3 {
	loadCombatModels()
	model, valid := combatModels[player.Character]
	if !valid {
		model = combatModels["urban"]
	}
	key := weaponExtension(player.Weapon) + "Stand"
	fallback := "stand"
	if player.Crouch {
		key, fallback = weaponExtension(player.Weapon)+"Crouch", "crouch"
	}
	pose := model.Poses[key]
	if len(pose) == 0 {
		pose = model.Poses[fallback]
	}
	point, count := Vec3{}, 0
	for _, box := range model.Hitboxes {
		if box.Group != 2 || box.Bone < 0 || box.Bone >= len(pose) {
			continue
		}
		bone := pose[box.Bone]
		local := Vec3{(box.Min.X + box.Max.X) / 2, (box.Min.Y + box.Max.Y) / 2, (box.Min.Z + box.Max.Z) / 2}
		point.X += bone.X + int((int64(bone.XX)*int64(local.X)+int64(bone.XY)*int64(local.Y)+int64(bone.XZ)*int64(local.Z))/NormalUnit)
		point.Y += bone.Y + int((int64(bone.YX)*int64(local.X)+int64(bone.YY)*int64(local.Y)+int64(bone.YZ)*int64(local.Z))/NormalUnit)
		point.Z += bone.Z + int((int64(bone.ZX)*int64(local.X)+int64(bone.ZY)*int64(local.Y)+int64(bone.ZZ)*int64(local.Z))/NormalUnit)
		count++
	}
	if count == 0 {
		return Vec3{player.X, player.Y, player.Z + 8*Unit}
	}
	point.X, point.Y, point.Z = point.X/count, point.Y/count, point.Z/count
	direction := Direction(player.Yaw, 0)
	return Vec3{player.X + int((int64(direction.X)*int64(point.X)-int64(direction.Y)*int64(point.Y))/NormalUnit), player.Y + int((int64(direction.Y)*int64(point.X)+int64(direction.X)*int64(point.Y))/NormalUnit), player.Z + point.Z}
}

func botAimAngles(state *State, seat, enemy int) (int, int, bool) {
	player := &state.Players[seat]
	eye, point := Eye(*player), botChestPoint(state.Players[enemy])
	focus := min(7500, max(6500, int(state.Frame-player.BotSeenAt)*10000/(2*FPS)))
	error := Distance(eye, point) * (10000 - focus) / 100000
	bucket, phase := state.Frame/25, int(state.Frame%25)
	seed := player.BotAimSeed
	if seed == 0 {
		seed = uint32(seat+1) * 1103515245
	}
	offset := [3]int{}
	for axis := range offset {
		from, to := botAimNoise(seed, axis, bucket-1), botAimNoise(seed, axis, bucket)
		offset[axis] = int(int64(from*(25-phase)+to*phase) * int64(error) / (25 * 10000))
	}
	point = Add(point, Vec3{offset[0], offset[1], offset[2]})
	dx, dy, dz := point.X-eye.X, point.Y-eye.Y, point.Z-eye.Z
	yaw := AngleTo(dx, dy)
	pitch := max(-16000, min(16000, -botAngleDifference(AngleTo(Length2D(dx, dy), dz), 0)))
	yawDifference, pitchDifference := botAngleDifference(yaw, player.Yaw), pitch-player.Pitch
	aligned := objectiveAbs(yawDifference) <= 1024 && objectiveAbs(pitchDifference) <= 1024
	yaw = (player.Yaw + Clamp(yawDifference, -4096, 4096) + 65536) % 65536
	pitch = player.Pitch + Clamp(pitchDifference, -4096, 4096)
	return yaw, pitch, aligned
}

func botKeepBombMovement(state *State, seat int, input Input) Input {
	point, _ := botBombDestination(state, seat)
	yaw, pitch := input.Yaw, input.Pitch
	input.Crouch = false
	navigation := botNavigate(state, seat, point, false, input)
	sine, cosine := SinCos(navigation.Yaw - yaw)
	input.MoveY = (navigation.MoveY*cosine + navigation.MoveX*sine) / NormalUnit
	input.MoveX = (navigation.MoveX*cosine - navigation.MoveY*sine) / NormalUnit
	if length := Length2D(input.MoveX, input.MoveY); length > 1000 {
		input.MoveX = input.MoveX * 1000 / length
		input.MoveY = input.MoveY * 1000 / length
	}
	input.Jump, input.Crouch, input.Use = navigation.Jump, navigation.Crouch, navigation.Use
	input.Yaw, input.Pitch = yaw, pitch
	return input
}
