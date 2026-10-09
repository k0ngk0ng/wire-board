package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func publicCatanHarborsScenario(scenario string) bool {
	return publicCatanCaravanSea(scenario) || scenario == "" || scenario == "cities-knights" || scenario == "fishing" || publicCatanSeaScenario(scenario)
}

func (r *Room) publicCatanHarborsAvailable() bool {
	return (r.Kind == "catan" && publicCatanCaravanSea(r.CatanScenario) && r.Capacity >= 2 && r.Capacity <= 6) || r.publicCatanTwoVariantsAvailable() || ((r.isCatanBaseRecipe() || r.isCatanStandaloneKnightsRecipe()) && r.Capacity >= 3 && r.Capacity <= 6 && (r.Capacity > 4) == r.CatanOptions.FiveSix) || r.Kind == "catan" && r.Capacity >= 3 && r.Capacity <= 6 && (r.Capacity > 4) == r.CatanOptions.FiveSix && r.CatanTwoRules == "" && publicCatanHarborsScenario(r.CatanScenario)
}

// Existing internally configured extended rooms retain their setup path.
func (r *Room) setCatanHarbors(request game.CatanHarborsSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" {
		return errors.New("只能在卡坦岛开局前调整港口霸主")
	}
	setup, err := game.NormalizeCatanHarborsSetup(request)
	if err != nil {
		return err
	}
	if r.CatanHarbors != nil && *r.CatanHarbors == setup {
		return nil
	}
	r.CatanHarbors = &setup
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
