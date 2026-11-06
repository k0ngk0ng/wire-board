package engine

import "slices"

func SceneInfo(id string) (MapInfo, bool) {
	m := MapInfo{ID: id, Rows: 5, Cols: 9, WaterRows: []int{}}
	switch id {
	case "day":
	case "night":
		m.Night = true
	case "pool":
		m.Rows = 6
		m.WaterRows = []int{2, 3}
	case "fog":
		m.Rows = 6
		m.WaterRows = []int{2, 3}
		m.Night = true
		m.Fog = true
	case "roof":
		m.Roof = true
	case "roofnight":
		m.Roof = true
		m.Night = true
	default:
		return m, false
	}
	return m, true
}
func (s *State) rowCount() int {
	if s.Map.Rows == 0 {
		return Rows
	}
	return s.Map.Rows
}
func (s *State) isWater(row int) bool { return slices.Contains(s.Map.WaterRows, row) }
func (s *State) RowCount() int        { return s.rowCount() }
func (s *State) IsWater(row int) bool { return s.isWater(row) }
func DefaultDeck(scene, side string) []string {
	if side == "zombies" {
		if scene == "pool" || scene == "fog" {
			return []string{"zombie", "conehead", "duckytube", "snorkel", "dolphin", "balloon", "football", "bungee", "gargantuar", "digger"}
		}
		if scene == "roofnight" {
			return []string{"zombie", "conehead", "buckethead", "football", "catapult", "bungee", "ladder", "gargantuar", "redeye", "zomboss"}
		}
		return []string{"zombie", "conehead", "buckethead", "newspaper", "polevaulter", "football", "balloon", "digger", "gargantuar", "imp"}
	}
	switch scene {
	case "night":
		return []string{"sunshroom", "puffshroom", "fumeshroom", "gravebuster", "hypnoshroom", "iceshroom", "wallnut", "magnetshroom", "doomshroom", "coffeebean"}
	case "pool":
		return []string{"sunflower", "peashooter", "lilypad", "tanglekelp", "cattail", "cactus", "wallnut", "cherrybomb", "repeater", "pumpkin"}
	case "fog":
		return []string{"sunshroom", "puffshroom", "lilypad", "seashroom", "plantern", "blover", "magnetshroom", "fumeshroom", "cactus", "pumpkin"}
	case "roof", "roofnight":
		return []string{"sunflower", "flowerpot", "cabbagepult", "kernelpult", "melonpult", "wintermelon", "umbrellaleaf", "pumpkin", "coffeebean", "cobcannon"}
	default:
		return []string{"sunflower", "peashooter", "repeater", "snowpea", "wallnut", "potatomine", "cherrybomb", "tallnut", "torchwood", "pumpkin"}
	}
}
func (s *State) bodyAt(row, col int) int {
	for i, p := range s.Plants {
		if p.HP > 0 && p.Layer == "body" && PlantOccupies(p, row, col) {
			return i
		}
	}
	return -1
}
func (s *State) layerAt(row, col int, layer string) int {
	for i, p := range s.Plants {
		if p.HP > 0 && p.Row == row && p.Col == col && p.Layer == layer {
			return i
		}
	}
	return -1
}
func (s *State) addPickup(kind string, owner, row, x, amount int) int {
	if len(s.Pickups) >= 128 || owner < 0 || owner >= len(s.Players) || amount <= 0 {
		return 0
	}
	id := s.id()
	s.Pickups = append(s.Pickups, Pickup{ID: id, Owner: owner, Row: row, X: x, Y: row*1000 + 500, Amount: amount, Kind: kind, AutoAt: s.Tick + 60, ExpiresAt: s.Tick + 200})
	return id
}
func (s *State) addCrater(row, col int, duration int64) {
	if row < 0 || row >= s.rowCount() || col < 0 || col >= Columns {
		return
	}
	for i := range s.Craters {
		if s.Craters[i].Row == row && s.Craters[i].Col == col {
			s.Craters[i].Until = max(s.Craters[i].Until, s.Tick+duration)
			return
		}
	}
	s.Craters = append(s.Craters, Crater{ID: s.id(), Row: row, Col: col, Until: s.Tick + duration})
}
func (s *State) removeGrave(id int) bool {
	for i, g := range s.Graves {
		if g.ID == id {
			s.Graves = append(s.Graves[:i], s.Graves[i+1:]...)
			return true
		}
	}
	return false
}
func (s *State) collectPickup(id int) bool {
	for i, p := range s.Pickups {
		if p.ID == id {
			s.Players[p.Owner].Money = min(MaxMoney, s.Players[p.Owner].Money+p.Amount)
			s.emit("collect", p.Row, p.X, p.Owner, p.Kind, p.ID, p.Amount)
			s.Pickups = append(s.Pickups[:i], s.Pickups[i+1:]...)
			return true
		}
	}
	return false
}
func (s *State) worldStep() {
	for i := range s.Plants {
		p := &s.Plants[i]
		if p.Kind == "imitater" && s.Tick >= p.BornAt+40 && s.Tick < p.ReadyAt && p.State != "imitater-explode" {
			p.State = "imitater-explode"
			p.Flags["phaseAt"] = s.Tick
		}
		if p.Kind == "imitater" && p.ReadyAt <= s.Tick {
			s.transformImitater(i)
		} else if p.Asleep && p.Flags["wakeAt"] > 0 && p.Flags["wakeAt"] <= s.Tick {
			s.wakeCoffee(i)
		}
	}
	for i := len(s.Pickups) - 1; i >= 0; i-- {
		if s.Tick >= s.Pickups[i].AutoAt || s.Tick >= s.Pickups[i].ExpiresAt {
			s.collectPickup(s.Pickups[i].ID)
		}
	}
	live := s.Craters[:0]
	for _, c := range s.Craters {
		if c.Until > s.Tick {
			live = append(live, c)
		}
	}
	s.Craters = live
	terrain := s.Terrain[:0]
	for _, v := range s.Terrain {
		if v.Until > s.Tick {
			terrain = append(terrain, v)
		}
	}
	s.Terrain = terrain
	ladder := s.Ladders[:0]
	for _, v := range s.Ladders {
		found := false
		for _, p := range s.Plants {
			if p.ID == v.PlantID && p.HP > 0 {
				found = true
				break
			}
		}
		if found {
			ladder = append(ladder, v)
		}
	}
	s.Ladders = ladder
}
func (s *State) spawnZombie(kind string, row, x, owner int) int {
	spec, ok := ZombieSpec(kind)
	if !ok || len(s.Zombies) >= MaxZombies || owner < 0 || owner >= len(s.Players) || row < 0 || row >= s.rowCount() {
		return 0
	}
	z := Zombie{ID: s.id(), Owner: owner, Kind: kind, Row: row, X: x, HP: spec.HP, MaxHP: spec.HP, Armor: spec.Armor, MaxArmor: spec.Armor, State: "walking", BornAt: s.Tick, Allegiance: "zombies", Flags: map[string]int64{}}
	z.SpeedMilli = (230 + s.random(141)) * 29375 / 1000
	switch kind {
	case "football":
		z.SpeedMilli = (660 + s.random(21)) * 29375 / 1000
	case "polevaulter":
		z.SpecialSpeedMilli = (660 + s.random(21)) * 29375 / 1000
	case "newspaper":
		z.SpecialSpeedMilli = (890 + s.random(21)) * 29375 / 1000
	}
	s.initializeZombie(&z)
	s.Zombies = append(s.Zombies, z)
	s.emit("spawn", row, x, owner, kind, z.ID, 0)
	return z.ID
}

func PlantOccupies(p Plant, row, col int) bool {
	return p.Row == row && (p.Col == col || p.Kind == "cobcannon" && p.Col+1 == col)
}
