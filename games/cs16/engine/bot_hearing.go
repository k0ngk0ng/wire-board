package engine

type BotMemory struct {
	Life             int   `json:"life"`
	Round            int   `json:"round"`
	SoundCursor      int64 `json:"soundCursor"`
	HeardSource      int   `json:"heardSource"`
	HeardLife        int   `json:"heardLife"`
	HeardPoint       Vec3  `json:"heardPoint"`
	HeardFrame       int64 `json:"heardFrame"`
	HeardSince       int64 `json:"heardSince"`
	HeardUntil       int64 `json:"heardUntil"`
	InvestigatePoint Vec3  `json:"investigatePoint"`
	ProgressPoint    Vec3  `json:"progressPoint"`
	ProgressAt       int64 `json:"progressAt"`
	EscapePoint      Vec3  `json:"escapePoint"`
	EscapeUntil      int64 `json:"escapeUntil"`
	EscapeCount      int   `json:"escapeCount"`
}

func botMemory(state *State, seat int) *BotMemory {
	if seat < 0 || seat >= len(state.Players) || seat >= len(state.BotMemories) {
		return nil
	}
	player, memory := &state.Players[seat], &state.BotMemories[seat]
	if !player.Alive || memory.Life != player.Life || memory.Round != state.Round {
		return nil
	}
	return memory
}

func botSoundRadius(event Event) int {
	switch event.Kind {
	case "step", "knife_air", "knife_hit", "knife_wall":
		return 1250
	case "land":
		return 512
	case "shot", "fire":
		if event.Weapon == "knife" || event.Weapon == "c4" {
			return 0
		}
		if event.Value == 1 && (event.Weapon == "usp" || event.Weapon == "m4a1") {
			return 500
		}
		return 2048
	}
	return 0
}

func botClearSound(memory *BotMemory) {
	memory.HeardSource, memory.HeardLife = -1, 0
	memory.HeardPoint, memory.HeardFrame, memory.HeardUntil = Vec3{}, 0, 0
	memory.HeardSince = 0
	memory.InvestigatePoint = Vec3{}
}

func stepBotMemory(state *State, seat int) {
	for len(state.BotMemories) < len(state.Players) {
		state.BotMemories = append(state.BotMemories, BotMemory{})
	}
	player, memory := &state.Players[seat], &state.BotMemories[seat]
	if memory.Life != player.Life || memory.Round != state.Round || !player.Alive || memory.ProgressAt > state.Frame {
		cursor := int64(0)
		if memory.Life > 0 || !player.Alive {
			cursor = state.EventID
		}
		*memory = BotMemory{Life: player.Life, Round: state.Round, SoundCursor: cursor, HeardSource: -1, ProgressPoint: Position(*player), ProgressAt: state.Frame}
	}
	if !player.Alive {
		return
	}
	if memory.HeardUntil > 0 {
		valid := memory.HeardSource >= 0 && memory.HeardSource < len(state.Players)
		if valid {
			source := state.Players[memory.HeardSource]
			valid = source.Alive && source.Team != player.Team && source.Life == memory.HeardLife
		}
		if !valid || memory.HeardUntil <= state.Frame || memory.HeardFrame > state.Frame || state.Frame-memory.HeardFrame >= 30 && botHorizontalDistance(Position(*player), memory.HeardPoint) <= 64*Unit && Abs(player.Z-memory.HeardPoint.Z) <= 80*Unit {
			botClearSound(memory)
		}
	}
	for _, event := range state.Events {
		if event.ID <= memory.SoundCursor {
			continue
		}
		memory.SoundCursor = event.ID
		radius := botSoundRadius(event)
		if radius == 0 || event.Seat < 0 || event.Seat >= len(state.Players) || event.Seat == seat || event.Frame > state.Frame || state.Frame-event.Frame > 50 || event.Frame < memory.HeardFrame {
			continue
		}
		source := state.Players[event.Seat]
		if !source.Alive || source.Team == player.Team || event.Life > 0 && event.Life != source.Life {
			continue
		}
		dx, dy, dz := int64(event.X-player.X), int64(event.Y-player.Y), int64(event.Z-player.Z)
		limit := int64(radius * Unit)
		if dx*dx+dy*dy+dz*dz > limit*limit {
			continue
		}
		if memory.HeardUntil <= state.Frame || memory.HeardSince == 0 {
			memory.HeardSince = event.Frame
		}
		if memory.HeardUntil <= state.Frame || botHorizontalDistance(Position(*player), memory.InvestigatePoint) <= 64*Unit && Abs(player.Z-memory.InvestigatePoint.Z) <= 80*Unit {
			memory.InvestigatePoint = Vec3{event.X, event.Y, event.Z}
		}
		memory.HeardSource, memory.HeardLife = event.Seat, source.Life
		memory.HeardPoint = Vec3{event.X, event.Y, event.Z}
		memory.HeardFrame, memory.HeardUntil = event.Frame, event.Frame+250
	}
	stepBotProgress(state, seat, memory)
}

func botHearingDestination(state *State, seat int) (Vec3, bool) {
	memory := botMemory(state, seat)
	if memory == nil || memory.HeardFrame > state.Frame || memory.HeardUntil <= state.Frame {
		return Vec3{}, false
	}
	player := &state.Players[seat]
	if player.Action == "plant" || player.Action == "defuse" || state.World.Mode == "bomb" && (state.Bomb.Planted && player.Team == "ct" || state.Bomb.Carrier == seat && InBombZone(state, seat)) {
		return Vec3{}, false
	}
	if state.World.Mode == "hostage" && player.Team == "ct" {
		for _, hostage := range state.Hostages {
			if hostage.Following == seat && !hostage.Rescued && hostage.Health > 0 {
				return Vec3{}, false
			}
		}
	}
	return memory.InvestigatePoint, true
}

func botInvestigateSound(state *State, seat int, point Vec3, input Input) Input {
	player := &state.Players[seat]
	input.Select = botBestWeapon(player)
	input = botNavigate(state, seat, point, state.World.Mode == "hostage" && player.Team == "ct", input)
	if memory := botMemory(state, seat); memory != nil && state.Frame-memory.HeardSince < 30 {
		yaw := AngleTo(memory.HeardPoint.X-player.X, memory.HeardPoint.Y-player.Y)
		input.Yaw = (player.Yaw + Clamp(botAngleDifference(yaw, player.Yaw), -2048, 2048) + 65536) % 65536
		input.MoveX, input.MoveY = 0, 0
	}
	input.Fire, input.Secondary = false, false
	return input
}

func botGuardInput(state *State, seat int, goal Vec3, input Input) Input {
	player := &state.Players[seat]
	position := Position(*player)
	point := Sub(Scale(position, 2, 1), goal)
	best := 0
	for _, candidate := range state.World.Nav {
		distance := botHorizontalDistance(position, candidate)
		if distance < 160*Unit || distance > 600*Unit || Abs(candidate.Z-position.Z) > 96*Unit {
			continue
		}
		score := botHorizontalDistance(candidate, goal)
		if score > best && botPointVisible(state, player, Add(candidate, Vec3{Z: 17 * Unit})) {
			point, best = candidate, score
		}
	}
	yaw := 0
	if botHorizontalDistance(position, point) > Unit {
		yaw = AngleTo(point.X-position.X, point.Y-position.Y)
	}
	switch (state.Frame/200 + int64(seat)) % 4 {
	case 1:
		yaw += 4096
	case 3:
		yaw -= 4096
	}
	input.Yaw = (player.Yaw + Clamp(botAngleDifference(yaw, player.Yaw), -2048, 2048) + 65536) % 65536
	input.Pitch = 0
	return input
}
