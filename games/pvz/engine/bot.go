package engine

import "slices"

// BotAction only reads the acting seat's supplied world. The host supplies a
// visibility projection in fog; no hidden-state lookup or state mutation occurs.
// Candidate placement uses the same pure preconditions as Apply, avoiding a
// full state copy for every square considered in the real-time host lock.
func BotAction(s *State, actor int) *Action {
	if actor < 0 || actor >= len(s.Players) || s.Phase != "prepare" && s.Phase != "battle" {
		return nil
	}
	if s.RulesVersion >= 4 {
		visible := *s
		visible.Players = append([]Player(nil), s.Players...)
		visible.Players[actor].Deck = rosterKinds(s.Players[actor].Side)
		s = &visible
	}
	p := s.Players[actor]
	if p.Seq >= 2147483647 {
		return nil
	}
	makeAction := func(typ, kind string, row, col int) *Action {
		return &Action{Type: typ, Kind: kind, Row: row, Col: col, Epoch: s.Epoch, Seq: p.Seq + 1}
	}
	for _, v := range s.Pickups {
		if v.Owner >= 0 && v.Owner < len(s.Players) && s.Players[v.Owner].Team == p.Team && v.AutoAt > s.Tick {
			return &Action{Type: "collect", PickupID: v.ID, Epoch: s.Epoch, Seq: p.Seq + 1}
		}
	}
	if p.Side == "zombies" {
		return botZombieAction(s, actor, makeAction)
	}
	return botPlantAction(s, actor, makeAction)
}

type botActionMaker func(string, string, int, int) *Action

func botCardReady(s *State, actor int, kind string, c Spec) bool {
	p := s.Players[actor]
	return slices.Contains(p.Deck, kind) && p.Money >= c.Cost && p.Cooldowns[kind] <= s.Tick && s.TeamCooldowns[p.Team][kind] <= s.Tick
}
func botIsShooter(kind string) bool {
	switch kind {
	case "peashooter", "snowpea", "repeater", "gatlingpea", "threepeater", "splitpea", "starfruit", "puffshroom", "seashroom", "scaredyshroom", "fumeshroom", "gloomshroom", "cactus", "cattail", "cabbagepult", "kernelpult", "melonpult", "wintermelon", "cobcannon":
		return true
	}
	return false
}
func botIsIncome(kind string) bool {
	return kind == "sunflower" || kind == "sunshroom" || kind == "twinsunflower" || kind == "marigold"
}
func botEnemy(z Zombie) bool { return z.HP > 0 && z.Allegiance != "plants" }

func botPlantAction(s *State, actor int, makeAction botActionMaker) *Action {
	p := s.Players[actor]
	rows := s.rowCount()
	nearest := [MaxRows]int{}
	shooters := [MaxRows]int{}
	rowBodies := [MaxRows]int{}
	ownIncome, totalIncome := 0, 0
	for row := 0; row < rows; row++ {
		nearest[row] = 12000
	}
	for _, v := range s.Plants {
		if v.HP <= 0 || v.Row < 0 || v.Row >= rows {
			continue
		}
		if v.Layer == "body" {
			rowBodies[v.Row]++
		}
		if botIsShooter(v.Kind) && !v.Asleep {
			shooters[v.Row]++
		}
		if botIsIncome(v.Kind) {
			totalIncome++
			if v.Owner == actor {
				ownIncome++
			}
		}
	}
	for _, z := range s.Zombies {
		if botEnemy(z) && z.Row >= 0 && z.Row < rows {
			nearest[z.Row] = min(nearest[z.Row], z.X)
		}
	}
	// Charged artillery is an explicit ordinary action, never a forged impact.
	if s.Phase == "battle" {
		for _, v := range s.Plants {
			if v.Owner != actor || v.Kind != "cobcannon" || v.Asleep || v.HP <= 0 || v.ReadyAt > s.Tick || v.ChargeMS > s.Tick*TickMS {
				continue
			}
			target := -1
			score := -1
			for i, z := range s.Zombies {
				if !botEnemy(z) || z.Digging {
					continue
				}
				weight := 20 + (10000-z.X)/100
				for _, other := range s.Zombies {
					if botEnemy(other) && abs(other.Row-z.Row) <= 1 && abs(other.X-z.X) <= 1400 {
						weight += 10
					}
				}
				if weight > score {
					score = weight
					target = i
				}
			}
			if target >= 0 {
				z := s.Zombies[target]
				return &Action{Type: "ability", PlantID: v.ID, Row: z.Row, Col: max(0, min(Columns-1, z.X/1000)), Epoch: s.Epoch, Seq: p.Seq + 1}
			}
		}
	}
	bestScore := -1
	var best *Action
	offer := func(score int, a *Action) {
		if score > bestScore {
			bestScore = score
			best = a
		}
	}
	// An available coffee card wakes a real sleeping target before buying more
	// sleeping mushrooms; it can assist an ally while preserving plant ownership.
	if coffee, ok := PlantSpec("coffeebean"); ok && botCardReady(s, actor, "coffeebean", coffee) {
		for _, v := range s.Plants {
			if !v.Asleep || v.HP <= 0 || v.Layer != "body" {
				continue
			}
			if s.validatePlantPlace(actor, coffee, v.Row, v.Col) == nil {
				score := 95
				if v.Owner == actor {
					score += 5
				}
				offer(score, makeAction("plant", "coffeebean", v.Row, v.Col))
			}
		}
	}
	for _, kind := range p.Deck {
		c, ok := PlantSpec(kind)
		if !ok || kind == "coffeebean" {
			continue
		}
		effective := kind
		if kind == "imitater" {
			effective = p.ImitaterKind
			c, ok = PlantSpec(effective)
			if !ok || effective == "imitater" || c.UpgradeFrom != "" {
				continue
			}
		}
		// Daytime mushrooms require a wake-up action; prefer active plants from
		// the open pool instead of filling lanes with sleeping mushrooms.
		if s.RulesVersion >= 4 && c.Mushroom && !s.Map.Night {
			continue
		}
		if !botCardReady(s, actor, kind, c) || !specScene(c, s.Map.ID) {
			continue
		}
		if c.UpgradeFrom != "" {
			for _, base := range s.Plants {
				if base.Owner != actor || base.Kind != c.UpgradeFrom || base.HP <= 0 {
					continue
				}
				if s.validatePlantPlace(actor, c, base.Row, base.Col) != nil {
					continue
				}
				score := 75
				if botIsIncome(effective) {
					score = 55
					if ownIncome < 3 {
						score = 80
					}
				}
				if nearest[base.Row] < 7000 {
					score += 20
				}
				offer(score, makeAction("plant", kind, base.Row, base.Col))
			}
			continue
		}
		if effective == "gravebuster" {
			for _, g := range s.Graves {
				if s.validatePlantPlace(actor, c, g.Row, g.Col) == nil {
					offer(80, makeAction("plant", kind, g.Row, g.Col))
				}
			}
			continue
		}
		for offset := 0; offset < rows; offset++ {
			row := (offset + actor) % rows
			if (effective == "spikeweed" || effective == "spikerock") && (s.Map.Roof || s.isWater(row)) {
				continue
			}
			for col := 0; col < Columns; col++ {
				score := botPlantSquareScore(s, actor, effective, row, col, shooters, nearest, rowBodies, ownIncome, totalIncome)
				if score < 0 {
					continue
				}
				if s.validatePlantPlace(actor, c, row, col) == nil {
					a := makeAction("plant", kind, row, col)
					if kind == "imitater" {
						a.ImitateKind = effective
					}
					offer(score, a)
					continue
				}
				// A body candidate can request its missing legal support. Requiring the
				// combined price avoids spending the last night-time funds on empty pots.
				if c.Layer == "body" || c.Layer == "shell" {
					support := ""
					if s.isWater(row) && !c.WaterOnly {
						support = "lilypad"
					}
					if s.Map.Roof {
						support = "flowerpot"
					}
					if support != "" {
						base, yes := PlantSpec(support)
						if yes && botCardReady(s, actor, support, base) && p.Money >= base.Cost+c.Cost && s.validatePlantPlace(actor, base, row, col) == nil {
							offer(score-1, makeAction("plant", support, row, col))
						}
					}
				}
			}
		}
	}
	return best
}

func botPlantSquareScore(s *State, actor int, kind string, row, col int, shooters [MaxRows]int, nearest [MaxRows]int, bodies [MaxRows]int, ownIncome, totalIncome int) int {
	p := s.Players[actor]
	threat := nearest[row] < 11000
	urgent := nearest[row] < 3500
	front := max(0, min(Columns-1, nearest[row]/1000))
	position := max(0, 12-abs(col-2)*4)
	score := 25 + position
	if botIsIncome(kind) {
		if totalIncome >= 6 || ownIncome >= 4 {
			return -1
		}
		score = 60 - ownIncome*16 - abs(col)*8
		if ownIncome < 2 {
			score += 15
		}
		if urgent {
			score -= 25
		}
		return score
	}
	switch kind {
	case "lilypad", "flowerpot":
		// Standalone support decks are legal and useful for cooperative builders.
		// With other bodies available, their desired placement drives the support.
		if len(p.Deck) > 1 {
			return -1
		}
		return 20 + position
	case "plantern":
		if !s.Map.Fog && len(p.Deck) > 1 {
			return -1
		}
		for _, v := range s.Plants {
			if v.Kind == kind && v.HP > 0 && abs(v.Row-row) <= 1 && abs(v.Col-col) <= 2 {
				return -1
			}
		}
		return 65 - abs(col-4)*10 - abs(row-s.rowCount()/2)*4
	case "cherrybomb", "jalapeno", "iceshroom", "doomshroom", "squash", "tanglekelp":
		if s.Phase != "battle" || !threat {
			return -1
		}
		if kind == "tanglekelp" && !s.isWater(row) {
			return -1
		}
		score = 45
		if urgent {
			score += 60
		}
		if kind == "iceshroom" {
			return score - abs(col-1)*4
		}
		if kind == "jalapeno" {
			return score - abs(col-2)*3
		}
		return score + max(0, 30-abs(col-front)*15)
	case "blover":
		airborne := false
		for _, z := range s.Zombies {
			if botEnemy(z) && z.Kind == "balloon" && z.Altitude > 0 {
				airborne = true
			}
		}
		if !airborne && (!s.Map.Fog || s.FogClearedUntil > s.Tick || s.Phase != "battle") {
			return -1
		}
		if airborne {
			return 105 - abs(col-1)*3
		}
		return 25 - abs(col-1)*3
	case "wallnut", "tallnut", "pumpkin":
		if kind == "pumpkin" {
			body := s.bodyAt(row, col)
			if body >= 0 {
				score = 55
				if threat {
					score += 25
				}
				return score
			}
			if len(p.Deck) > 1 {
				return -1
			}
		}
		score = 30 - abs(col-6)*9
		if threat {
			score += 25
		}
		if urgent {
			score += 25
		}
		return score
	case "spikeweed", "spikerock", "garlic":
		return 32 - abs(col-6)*9 + boolBotScore(threat, 20)
	case "chomper":
		return 35 - abs(col-5)*8 + boolBotScore(threat, 35)
	case "magnetshroom":
		score = 30 - abs(col-4)*6
		for _, z := range s.Zombies {
			if botEnemy(z) && z.Armor > 0 && abs(z.Row-row) <= 1 {
				score += 15
			}
		}
		return score
	case "umbrellaleaf":
		score = 25 - abs(col-2)*4
		protected := 0
		for _, v := range s.Plants {
			if v.HP > 0 && v.Layer == "body" && abs(v.Row-row) <= 1 && abs(v.Col-col) <= 1 {
				protected++
			}
		}
		return score + protected*7
	case "torchwood":
		if col < 3 {
			return -1
		}
		return 25 - abs(col-4)*7 + shooters[row]*10
	}
	if botIsShooter(kind) {
		score = 38 + position
		if shooters[row] == 0 {
			score += 26
		} else {
			score -= min(shooters[row], 4) * 8
		}
		if threat {
			score += 20
		}
		if urgent {
			score += 20
		}
		if kind == "puffshroom" || kind == "fumeshroom" || kind == "gloomshroom" || kind == "seashroom" {
			score += 15 - abs(col-5)*8
		}
		if kind == "cattail" {
			score += 20
		}
		if s.Map.Roof && (kind == "peashooter" || kind == "snowpea" || kind == "repeater" || kind == "gatlingpea") {
			score -= 20
			if col >= 4 {
				score += 25
			}
		}
		return score
	}
	if kind == "hypnoshroom" {
		return 35 - abs(col-5)*9 + boolBotScore(threat, 30)
	}
	return score - bodies[row]*2
}
func boolBotScore(condition bool, amount int) int {
	if condition {
		return amount
	}
	return 0
}

func botZombieAction(s *State, actor int, makeAction botActionMaker) *Action {
	p := s.Players[actor]
	if s.Phase != "battle" || len(s.Zombies) >= MaxZombies {
		return nil
	}
	rows := s.rowCount()
	bestScore := -1 << 30
	var best *Action
	for _, kind := range p.Deck {
		c, ok := ZombieSpec(kind)
		if !ok || !botCardReady(s, actor, kind, c) || !specScene(c, s.Map.ID) {
			continue
		}
		if kind == "zomboss" {
			present := false
			for _, z := range s.Zombies {
				if z.Kind == kind && z.HP > 0 {
					present = true
				}
			}
			if present {
				continue
			}
		}
		for offset := 0; offset < rows; offset++ {
			row := (offset + actor + s.Round) % rows
			if s.LaneCooldowns[row] > s.Tick || c.WaterOnly && !s.isWater(row) || s.isWater(row) && !c.WaterOnly && !c.WaterAllowed {
				continue
			}
			if c.RequiresIce {
				ice := false
				for _, v := range s.Terrain {
					if v.Kind == "ice" && v.Row == row && v.Until > s.Tick && v.ToX >= 9000 {
						ice = true
					}
				}
				if !ice {
					continue
				}
			}
			defense := 0
			antiAir := false
			for _, plant := range s.Plants {
				if plant.HP <= 0 || plant.Row != row {
					continue
				}
				if !plant.Asleep && botIsShooter(plant.Kind) {
					defense += 15
				}
				if plant.Kind == "wallnut" || plant.Kind == "tallnut" || plant.Kind == "pumpkin" {
					defense += plant.HP / 400
				}
				if plant.Kind == "cactus" || plant.Kind == "cattail" || plant.Kind == "blover" {
					antiAir = true
				}
				if kind == "digger" && (plant.Kind == "splitpea" || plant.Kind == "starfruit") {
					defense += 20
				}
			}
			strength := 0
			for _, z := range s.Zombies {
				if z.HP > 0 && z.Allegiance != "plants" && z.Row == row {
					strength += 3
					if z.X < 3000 {
						strength += 4
					}
				}
			}
			score := 100 - defense + strength + min(c.Cost/20, 30)
			if s.Mowers[row].State == "spent" {
				score += 20
			}
			if kind == "balloon" && !antiAir {
				score += 25
			}
			if kind == "digger" {
				score += 12
			}
			if kind == "dancing" {
				score += 8
			}
			if s.Tick-s.BattleStartTick > 1200 && c.Cost >= 175 {
				score += 15
			}
			col := 0
			if c.Target == "cell" {
				target := -1
				weight := -1
				for i, plant := range s.Plants {
					if plant.HP <= 0 || plant.Row != row {
						continue
					}
					value := plant.HP / 100
					spec, _ := PlantSpec(plant.Kind)
					value += spec.Cost
					covered := false
					for _, u := range s.Plants {
						if u.Kind == "umbrellaleaf" && !u.Asleep && u.HP > 0 && abs(u.Row-row) <= 1 && abs(u.Col-plant.Col) <= 1 {
							covered = true
						}
					}
					if covered {
						value -= 1000
					}
					if value > weight {
						target = i
						weight = value
					}
				}
				if target < 0 || weight < 0 {
					continue
				}
				col = s.Plants[target].Col
				score += weight / 5
			}
			if score > bestScore {
				bestScore = score
				best = makeAction("spawn", kind, row, col)
			}
		}
	}
	return best
}
