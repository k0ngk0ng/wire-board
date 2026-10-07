package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func publicCatanHarborsScenario(scenario string) bool {
	return scenario == "" || scenario == "cities-knights" || publicCatanSeaScenario(scenario)
}

func (r *Room) publicCatanHarborsAvailable() bool {
	return ((r.isCatanBaseRecipe() || r.isCatanStandaloneKnightsRecipe()) && r.Capacity >= 3 && r.Capacity <= 6 && (r.Capacity > 4) == r.CatanOptions.FiveSix) || r.Kind == "catan" && r.Capacity >= 3 && r.Capacity <= 4 && !r.CatanOptions.FiveSix && !r.CatanFishing && r.CatanTwoRules == "" && publicCatanHarborsScenario(r.CatanScenario)
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
	if setup.Enabled && (r.CatanOptions.Helpers || r.CatanOptions.AllHelpers) {
		return errors.New("港口霸主与助手的组合尚未核验")
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
