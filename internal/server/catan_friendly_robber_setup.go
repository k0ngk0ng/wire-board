package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func (r *Room) publicCatanFriendlyAvailable() bool {
	return r.Kind == "catan" && r.Capacity >= 3 && r.Capacity <= 4 && !r.CatanOptions.FiveSix && !r.CatanFishing && r.CatanTwoRules == "" && (r.CatanScenario == "" || publicCatanSeaScenario(r.CatanScenario))
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
	if r.CatanFishing || r.CatanNewWorldMap != nil || r.CatanCitiesKnights != nil || r.CatanOptions.Helpers || r.CatanOptions.AllHelpers {
		return errors.New("友善强盗与渔夫、新世界、城市骑士或助手的组合尚未核验")
	}
	if r.CatanSeafarers != nil && r.CatanSeafarers.Scenario == "shores" && n == 3 {
		return errors.New("新海岸的三人地图没有沙漠，请凑齐4人或更换剧本")
	}
	if r.CatanSeafarers != nil && !game.CatanFriendlySeafarersSupported(n, r.CatanSeafarers.Scenario) {
		return errors.New("此人数或剧本暂不支持友善强盗，请更换剧本或关闭此变体")
	}
	return nil
}
func (r *Room) catanFriendlyMinimumPlayers() int {
	if r.CatanOptions.FiveSix {
		return 5
	}
	if r.CatanSeafarers != nil && r.CatanSeafarers.Scenario == "shores" {
		return 4
	}
	return 3
}
