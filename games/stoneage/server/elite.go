package stoneage

import (
	_ "embed"
	"fmt"
	"strconv"
)

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

// New art must never silently use the old 120ms missing-timing fallback.
func verifyEliteTiming() {
	timingCatalog()
	for _, pet := range eliteCatalog.Pets {
		for _, direction := range []string{"3", "7"} {
			t := nativeTiming.Graphics[strconv.Itoa(pet.Graphic)][direction]
			if t.Attack.Duration <= 0 || t.Hit.Duration <= 0 || t.Dead.Duration <= 0 {
				panic(fmt.Sprintf("missing elite native timing: %d/%s", pet.Graphic, direction))
			}
		}
	}
}
