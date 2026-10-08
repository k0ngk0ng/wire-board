package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func (r *Room) publicCatanFriendlyAvailable() bool {
	return r.publicCatanTwoVariantsAvailable() || ((r.isCatanBaseRecipe() || r.isCatanStandaloneKnightsRecipe()) && r.Capacity >= 3 && r.Capacity <= 6 && (r.Capacity > 4) == r.CatanOptions.FiveSix) || r.Kind == "catan" && r.Capacity >= 3 && r.Capacity <= 6 && (r.Capacity > 4) == r.CatanOptions.FiveSix && r.CatanTwoRules == "" && (r.CatanScenario == "" || r.CatanScenario == "cities-knights" || r.CatanScenario == "fishing" || publicCatanSeaScenario(r.CatanScenario))
}

// Existing internal recipes continue to use the same normalized setup.
func (r *Room) setCatanFriendlyRobber(request game.CatanFriendlyRobberSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" {
		return errors.New("只能在卡坦岛开局前调整友善强盗")
	}
	setup, err := game.NormalizeCatanFriendlyRobberSetup(request)
	if err != nil {
		return err
	}
	if setup.Enabled {
		if err := r.validateCatanFriendlyRobber(max(3, r.Capacity)); err != nil {
			return err
		}
	}

	if r.CatanFriendlyRobber != nil && *r.CatanFriendlyRobber == setup {
		return nil
	}
	r.CatanFriendlyRobber = &setup
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
func (r *Room) friendlyRobberEnabled() bool {
	return r.CatanFriendlyRobber != nil && r.CatanFriendlyRobber.Enabled
}

func (r *Room) validateCatanFriendlyRobber(n int) error {
	if r.CatanTwoRules != "" && !r.publicCatanTwoVariantsAvailable() {
		return errors.New("此双人剧本的友善强盗组合尚未接通")
	}
	if r.CatanFishing {
		if err := r.validateCatanFishing(); err != nil {
			return err
		}
	}
	if r.CatanNewWorldMap != nil && (r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != "new_world") {
		return errors.New("新世界需要对应的航海家地图配置")
	}
	if r.CatanSeafarers != nil && !game.CatanFriendlySeafarersSupported(n, r.CatanSeafarers.Scenario) {
		return errors.New("此人数或剧本暂不支持友善强盗，请更换剧本或关闭此变体")
	}
	return nil
}
func (r *Room) catanFriendlyMinimumPlayers() int {
	if r.publicCatanTwoVariantsAvailable() {
		return 2
	}
	if r.CatanOptions.FiveSix {
		return 5
	}
	return 3
}
