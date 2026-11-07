package engine

// Versus economy is a platform balance profile. Prices, damage and original
// campaign counters remain separate; saved profiles 0/2 retain their economy.
func (s State) openingSun() int {
	if s.RulesVersion < 3 {
		return 300
	}
	switch s.Map.ID {
	case "night":
		return 300
	case "fog":
		return 400
	case "pool":
		return 600
	case "roof", "roofnight":
		return 700
	default:
		return 500
	}
}

func (s State) sunSupplyTicks() int64 {
	if s.RulesVersion < 3 {
		return 5000 / TickMS
	}
	if s.Map.Roof {
		return 3000 / TickMS
	}
	if s.Map.Night {
		return 8000 / TickMS
	}
	return 4000 / TickMS
}

func (s *State) firstSunMS() int64 {
	span := 951
	if s.RulesVersion >= 3 {
		span = 301
	}
	return int64((300 + s.random(span)) * 10)
}

func (s *State) productionIntervalMS(kind string) int64 {
	if s.RulesVersion >= 3 && kind != "marigold" {
		return int64((1800 + s.random(201)) * 10)
	}
	return int64((2350 + s.random(151)) * 10)
}

func (s State) startingDeck(side string) []string {
	if s.RulesVersion < 3 {
		return DefaultDeck(s.Map.ID, side)
	}
	if side == "plants" {
		if s.Map.Roof {
			return []string{"sunflower", "flowerpot", "cabbagepult", "kernelpult", "melonpult", "cherrybomb", "jalapeno", "pumpkin", "blover", "umbrellaleaf"}
		}
		return DefaultDeck(s.Map.ID, side)
	}
	switch s.Map.ID {
	case "night":
		return []string{"zombie", "conehead", "buckethead", "football", "jackinthebox", "zomboni", "jalapenohead", "gargantuar", "peahead", "gatlinghead"}
	case "pool", "fog":
		return []string{"zombie", "conehead", "duckytube", "snorkel", "dolphin", "balloon", "football", "gargantuar", "peahead", "gatlinghead"}
	case "roofnight":
		return []string{"zombie", "conehead", "buckethead", "football", "catapult", "bungee", "gargantuar", "zomboss", "peahead", "gatlinghead"}
	default:
		return []string{"zombie", "conehead", "buckethead", "polevaulter", "football", "balloon", "digger", "gargantuar", "peahead", "gatlinghead"}
	}
}
