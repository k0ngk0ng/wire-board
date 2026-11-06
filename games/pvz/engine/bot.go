package engine

// BotAction uses the same public board and wallet as its seat. Proposals never
// mutate the world, and the authoritative Apply performs every purchase.
func BotAction(s *State, actor int) *Action {
	if actor < 0 || actor >= len(s.Players) || s.Phase != "prepare" && s.Phase != "battle" {
		return nil
	}
	p := s.Players[actor]
	action := func(kind, unit string, row, col int) *Action {
		return &Action{Type: kind, Kind: unit, Row: row, Col: col, Epoch: s.Epoch, Seq: p.Seq + 1}
	}
	can := func(kind string) bool {
		c, ok := PlantSpec(kind)
		if p.Side == "zombies" {
			c, ok = ZombieSpec(kind)
		}
		return ok && p.Money >= c.Cost && p.Cooldowns[kind] <= s.Tick && s.TeamCooldowns[p.Team][kind] <= s.Tick
	}
	if p.Side == "zombies" {
		if s.Phase != "battle" || len(s.Zombies) >= MaxZombies {
			return nil
		}
		bestRow := -1
		bestScore := int(^uint(0) >> 1)
		for off := range Rows {
			row := (off + actor + s.Round) % Rows
			if s.LaneCooldowns[row] > s.Tick {
				continue
			}
			score := 0
			for _, plant := range s.Plants {
				if plant.Row == row {
					switch plant.Kind {
					case "peashooter":
						score += 8
					case "repeater":
						score += 16
					case "snowpea":
						score += 14
					case "wallnut":
						score += plant.HP / 500
					case "tallnut":
						score += plant.HP / 500
					case "potatomine":
						score += 5
					}
				}
			}
			for _, z := range s.Zombies {
				if z.Row == row {
					score -= 2
				}
			}
			if s.Mowers[row].State == "spent" {
				score -= 5
			}
			if score < bestScore {
				bestScore = score
				bestRow = row
			}
		}
		if bestRow < 0 {
			return nil
		}
		age := s.Tick - s.BattleStartTick
		order := []string{"conehead", "zombie", "newspaper", "polevaulter", "buckethead", "football"}
		if age > 1200 {
			order = []string{"football", "buckethead", "polevaulter", "newspaper", "conehead", "zombie"}
		}
		for _, kind := range order {
			if can(kind) {
				return action("spawn", kind, bestRow, 0)
			}
		}
		return nil
	}
	occupied := [Rows][Columns]bool{}
	shooters := [Rows]int{}
	suns := 0
	nearest := [Rows]int{10000, 10000, 10000, 10000, 10000}
	for _, plant := range s.Plants {
		occupied[plant.Row][plant.Col] = true
		if plant.Kind == "sunflower" {
			suns++
		}
		if plant.Kind == "peashooter" || plant.Kind == "repeater" || plant.Kind == "snowpea" {
			shooters[plant.Row]++
		}
	}
	for _, z := range s.Zombies {
		nearest[z.Row] = min(nearest[z.Row], z.X)
	}
	if s.Phase == "battle" && can("cherrybomb") {
		for row := range Rows {
			if nearest[row] < 2500 {
				for col := range Columns {
					if !occupied[row][col] && abs(col*1000+500-nearest[row]) < 1400 {
						return action("plant", "cherrybomb", row, col)
					}
				}
			}
		}
	}
	// Address the weakest threatened lane before investing more in income.
	for off := range Rows {
		row := (off + actor) % Rows
		if shooters[row] == 0 && nearest[row] < 9000 && can("peashooter") {
			for col := 1; col < 5; col++ {
				if !occupied[row][col] {
					return action("plant", "peashooter", row, col)
				}
			}
		}
	}
	if suns < min(6, 2+int((s.Tick-s.BattleStartTick)/1000)) && can("sunflower") {
		for off := range Rows {
			row := (off + actor) % Rows
			if !occupied[row][0] {
				return action("plant", "sunflower", row, 0)
			}
		}
	}
	for off := range Rows {
		row := (off + actor) % Rows
		if shooters[row] == 0 && can("peashooter") {
			for col := 1; col < 5; col++ {
				if !occupied[row][col] {
					return action("plant", "peashooter", row, col)
				}
			}
		}
	}
	if can("wallnut") {
		for off := range Rows {
			row := (off + actor) % Rows
			if nearest[row] < 8500 && !occupied[row][6] {
				return action("plant", "wallnut", row, 6)
			}
		}
	}
	for _, kind := range []string{"snowpea", "repeater", "peashooter"} {
		if !can(kind) {
			continue
		}
		row := -1
		least := 100
		for off := range Rows {
			r := (off + actor) % Rows
			if shooters[r] < least {
				row = r
				least = shooters[r]
			}
		}
		for col := 2; col < 6; col++ {
			if !occupied[row][col] {
				return action("plant", kind, row, col)
			}
		}
	}
	if can("potatomine") {
		for off := range Rows {
			row := (off + actor) % Rows
			if !occupied[row][7] {
				return action("plant", "potatomine", row, 7)
			}
		}
	}
	return nil
}
