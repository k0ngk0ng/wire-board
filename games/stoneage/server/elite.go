package stoneage

import _ "embed"

//go:embed elite_source.json
var eliteJSON []byte
var eliteCatalog = sourceCatalog

func previewStats(pets []PetDefinition, points int) {
	for i := range pets {
		u := unitFromBuild(budget(pets[i].Weights, points))
		pets[i].Stats = &CandidateStats{HP: u.HP, Attack: u.Attack, Defense: u.Defense, Speed: u.Speed}
	}
}

func (s *State) gearRate(id int, critical bool) int {
	if id%2 != 0 {
		return 0
	}
	total := 0
	for _, g := range s.Pool.gear(*s.Players[id/2].Loadout) {
		if critical {
			total += g.Critical
		} else {
			total += g.Evade
		}
	}
	return total
}
