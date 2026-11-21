package engine

func teamIndex(team string) int {
	if team == "ct" {
		return 1
	}
	return 0
}

func objectiveMoney(player *Player, amount int) {
	player.Money = min(16000, max(0, player.Money+amount))
}

func InitRound(state *State) {
	state.Round++
	state.Phase = "freeze"
	state.PhaseEndsAt = state.Frame + 3*FPS
	state.BuyEndsAt = state.PhaseEndsAt + 90*FPS
	state.LastWinner = ""
	state.Grenades = []Grenade{}
	state.Drops = []Drop{}
	state.History = nil
	state.Bomb = Bomb{Carrier: -1, Defuser: -1}
	state.Hostages = make([]Hostage, len(state.World.Hostages))
	state.HostageRoutes = nil
	for i, spawn := range state.World.Hostages {
		state.Hostages[i] = Hostage{ID: i + 1, X: spawn.X, Y: spawn.Y, Z: spawn.Z, Health: 100, Following: -1}
	}
	state.Brushes = make([]BrushState, len(state.World.Brushes))
	for i, brush := range state.World.Brushes {
		state.Brushes[i] = BrushState{ID: brush.ID, Open: brush.Open, Health: brush.Health}
	}
	spawnCounts := [2]int{}
	terrorists := []int{}
	for seat := range state.Players {
		p := &state.Players[seat]
		survived := p.Alive && p.Health > 0 && p.Life > 0
		if !survived {
			p.Inventory = nil
			p.Armor = 0
			p.Helmet = false
			p.Defuser = false
			InitInventory(p)
		}
		withoutC4 := p.Inventory[:0]
		for _, item := range p.Inventory {
			if item.ID != "c4" {
				withoutC4 = append(withoutC4, item)
			}
		}
		p.Inventory = withoutC4
		p.Alive, p.Health = true, 100
		p.Life++
		p.VX, p.VY, p.VZ = 0, 0, 0
		p.RemX, p.RemY, p.RemZ = 0, 0, 0
		p.Crouch, p.Grounded = false, true
		p.Input, p.Previous, p.Pending = Input{}, Input{}, nil
		p.Action, p.ActionSince = "", 0
		p.RecoilPitch, p.RecoilYaw = 0, 0
		p.FlashUntil, p.FlashStrength = 0, 0
		p.NextAttack = state.Frame
		p.BotGoal, p.BotPath, p.BotTarget, p.BotDecision = Vec3{}, nil, -1, 0
		p.BotRouteMode = 0
		index := teamIndex(p.Team)
		spawns := state.World.SpawnsT
		if p.Team == "ct" {
			spawns = state.World.SpawnsCT
		} else {
			terrorists = append(terrorists, seat)
		}
		if len(spawns) > 0 {
			spawn := spawns[(spawnCounts[index]+state.Round-1)%len(spawns)]
			p.X, p.Y, p.Z, p.Yaw, p.Pitch = spawn.X, spawn.Y, spawn.Z, spawn.Yaw, 0
		}
		spawnCounts[index]++
		p.Input = Input{Life: p.Life, Yaw: p.Yaw, Pitch: p.Pitch}
		p.Previous = p.Input
		if p.Weapon == "c4" || !objectiveHasWeapon(p, p.Weapon) {
			p.Weapon = objectiveBestWeapon(p)
		}
		SyncWeapon(p)
	}
	if state.World.Mode == "bomb" && len(terrorists) > 0 {
		carrier := terrorists[(state.Round-1)%len(terrorists)]
		state.Bomb.Carrier = carrier
		state.Players[carrier].Inventory = append(state.Players[carrier].Inventory, WeaponState{ID: "c4", Ammo: 1})
	}
	if len(state.World.WorldWeapons) > 0 {
		InitMapWeapons(state)
	}
	Emit(state, Event{Kind: "round_start", Seat: -1, Target: -1, Value: state.Round})
}

func objectiveHasWeapon(player *Player, id string) bool {
	for _, item := range player.Inventory {
		if item.ID == id {
			return true
		}
	}
	return false
}

func objectiveBestWeapon(player *Player) string {
	for _, item := range player.Inventory {
		if weapon, ok := WeaponByID(item.ID); ok && weapon.Slot == 1 {
			return item.ID
		}
	}
	for _, item := range player.Inventory {
		if item.ID != "knife" && item.ID != "c4" {
			return item.ID
		}
	}
	return "knife"
}

func RoundWon(state *State, team, reason string) {
	if state.Finished || state.Phase != "live" {
		return
	}
	state.Phase = "round_end"
	state.PhaseEndsAt = state.Frame + 5*FPS
	state.LastWinner = team
	if team == "t" || team == "ct" {
		winner := teamIndex(team)
		loser := 1 - winner
		state.TeamScores[winner]++
		if state.LoserBonusBase <= 0 {
			state.LoserBonusBase = 1400
		}
		if state.LossStreak[winner] > 1 {
			state.LoserBonusBase = 1500
		}
		state.LossStreak[winner] = 0
		state.LossStreak[loser]++
		bonus := 3000
		if state.World.Mode == "bomb" || reason == "time" {
			bonus = 3250
		}
		if reason == "bomb_exploded" {
			bonus = 3500
		}
		if reason == "hostages_rescued" {
			bonus = 2500
		}
		remaining, rescued := 0, 0
		for _, hostage := range state.Hostages {
			if hostage.Rescued {
				rescued++
			} else {
				remaining++
			}
		}
		bonus += min(2000, remaining*150)
		loss := min(3000, state.LoserBonusBase+max(0, state.LossStreak[loser]-1)*500)
		for seat := range state.Players {
			p := &state.Players[seat]
			if p.Team == team {
				objectiveMoney(p, bonus)
			} else {
				if !(reason == "time" && p.Alive && (p.Team == "t" && state.World.Mode == "bomb" || p.Team == "ct" && state.World.Mode == "hostage")) {
					objectiveMoney(p, loss)
				}
				if p.Team == "t" && state.Bomb.Planted {
					objectiveMoney(p, 800)
				}
			}
			if p.Team == "ct" {
				objectiveMoney(p, rescued*750)
			}
		}
	}
	for seat := range state.Players {
		state.Players[seat].Action = ""
		state.Players[seat].ActionSince = 0
	}
	winnerValue := 0
	if team == "t" || team == "ct" {
		winnerValue = teamIndex(team) + 1
	}
	Emit(state, Event{Kind: "round_end", Seat: -1, Target: -1, Weapon: reason, Value: winnerValue})
}

func StepObjectives(state *State) {
	if state.Finished {
		return
	}
	if state.Phase == "round_end" {
		if state.Frame >= state.PhaseEndsAt {
			if state.Round >= state.Options.MaxRounds {
				state.Finished, state.Phase = true, "finished"
				state.PhaseEndsAt = 0
				Emit(state, Event{Kind: "match_end", Seat: -1, Target: -1})
			} else {
				InitRound(state)
			}
		}
		return
	}
	if state.Phase == "freeze" {
		if state.Frame >= state.PhaseEndsAt {
			state.Phase = "live"
			if state.World.Mode == "elimination" {
				state.PhaseEndsAt = 0
			} else {
				state.PhaseEndsAt = state.Frame + int64(state.Options.RoundSeconds)*FPS
			}
			Emit(state, Event{Kind: "round_live", Seat: -1, Target: -1})
		}
		return
	}
	if state.Phase != "live" {
		return
	}
	if state.World.Mode == "bomb" {
		stepBomb(state)
	} else if state.World.Mode == "hostage" {
		stepHostages(state)
	}
	if state.Phase != "live" {
		return
	}
	alive := [2]int{}
	for _, player := range state.Players {
		if player.Alive {
			alive[teamIndex(player.Team)]++
		}
	}
	if alive[0] == 0 && alive[1] == 0 && !state.Bomb.Planted {
		RoundWon(state, "draw", "elimination")
		return
	}
	if alive[1] == 0 {
		RoundWon(state, "t", "elimination")
		return
	}
	if alive[0] == 0 && !state.Bomb.Planted {
		RoundWon(state, "ct", "elimination")
		return
	}
	if state.World.Mode != "elimination" && state.Frame >= state.PhaseEndsAt && !state.Bomb.Planted {
		winner := "ct"
		if state.World.Mode == "hostage" {
			winner = "t"
		}
		RoundWon(state, winner, "time")
	}
}
