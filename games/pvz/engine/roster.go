package engine

import "slices"

// In profile 4 the saved deck is only the player's quickbar. All catalog units
// remain subject to the ordinary price, placement, recharge and unlock rules.
func (s State) canUseCard(actor int, kind string) bool {
	if actor < 0 || actor >= len(s.Players) {
		return false
	}
	p := s.Players[actor]
	_, known := PlantSpec(kind)
	if p.Side == "zombies" {
		_, known = ZombieSpec(kind)
	}
	return known && (s.RulesVersion >= 4 || slices.Contains(p.Deck, kind))
}

func rosterKinds(side string) []string {
	specs := plantSpecs
	if side == "zombies" {
		specs = zombieSpecs
	}
	kinds := make([]string, len(specs))
	for i, spec := range specs {
		kinds[i] = spec.Kind
	}
	return kinds
}
