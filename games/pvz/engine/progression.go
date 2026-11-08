package engine

// Special-unit staging is a versus balance rule, not an original campaign
// unlock. Existing stored matches keep their frozen rule profile.
func UnlockSeconds(kind string) int {
	switch kind {
	case "football", "dancing", "peahead", "wallnuthead":
		return 45
	case "snorkel", "dolphin", "jackinthebox", "pogo", "ladder", "squashhead", "tallnuthead":
		return 60
	case "balloon", "digger", "bungee", "zomboni", "bobsled", "catapult", "yeti", "jalapenohead", "gatlinghead":
		return 90
	case "gargantuar", "redeye", "zomboss":
		return 120
	default:
		return 0
	}
}

// EnableProgression selects the current versus profile for a newly created
// match. Restore never calls it, so a saved match keeps its original economy.
func EnableProgression(s *State) {
	s.RulesVersion = 4
	for i := range s.Players {
		s.Players[i].Deck = s.startingDeck(s.Players[i].Side)
		if s.Players[i].Side == "plants" {
			s.Players[i].Money = s.openingSun() / (len(s.Players) / 2)
		}
	}
	s.NextSunTick = s.Tick + s.sunSupplyTicks()
	s.seedUnlocks()
}
func (s *State) unlockAt(kind string) int64 {
	if s.RulesVersion < 2 {
		return 0
	}
	start := s.BattleStartTick
	if s.Phase == "prepare" {
		start = s.PhaseEndTick
	}
	return start + int64(UnlockSeconds(kind)*1000/TickMS)
}
func (s *State) seedUnlocks() {
	if s.RulesVersion < 2 {
		return
	}
	for i := range s.Players {
		p := &s.Players[i]
		if p.Side != "zombies" {
			continue
		}
		for _, c := range zombieSpecs {
			if UnlockSeconds(c.Kind) > 0 {
				p.Cooldowns[c.Kind] = s.unlockAt(c.Kind)
			}
		}
	}
}
func CatalogFor(s State) Catalog {
	c := GetCatalog()
	if s.RulesVersion >= 3 {
		for i := range c.Plants {
			switch c.Plants[i].Kind {
			case "sunflower":
				c.Plants[i].Description = "种下后 3～6 秒首次产出 25 阳光，此后每 18～20 秒产出；收益归种植者。"
			case "twinsunflower":
				c.Plants[i].Description = "升级后 3～6 秒首次产出双份阳光，此后每 18～20 秒产出 50 阳光；收益归种植者。"
			case "sunshroom":
				c.Plants[i].Description = "种下后 3～6 秒首次产出阳光，此后每 18～20 秒产出；幼体每次 15，长大后每次 25。"
			}
		}
	}
	if s.RulesVersion >= 2 {
		for i := range c.Zombies {
			c.Zombies[i].UnlockAfterSeconds = UnlockSeconds(c.Zombies[i].Kind)
			if c.Zombies[i].UnlockAfterSeconds > 0 {
				c.Zombies[i].UnlockAt = s.unlockAt(c.Zombies[i].Kind)
			}
		}
	}
	return c
}
