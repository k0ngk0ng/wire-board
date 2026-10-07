package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Shared by public standalone three/four-player rooms and internal combinations.
func (r *Room) setCatanCitiesKnights(setup game.CatanCitiesKnightsSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" {
		return fmt.Errorf("只能在城市与骑士开局前调整设置")
	}
	if r.friendlyRobberEnabled() {
		return fmt.Errorf("友善强盗目前仅核验基础版及港口霸主组合")
	}
	if err := r.validateCatanCitiesKnightsMap(); err != nil {
		return err
	}
	if r.CatanOptions.Helpers || r.CatanOptions.AllHelpers {
		return fmt.Errorf("Helpers尚无与城市与骑士组合的官方兼容规则")
	}
	if (r.Capacity > 4) != r.CatanOptions.FiveSix {
		return fmt.Errorf("城市与骑士人数与扩充不一致")
	}
	normalized, err := game.NormalizeCatanCitiesKnightsSetup(max(3, r.Capacity), setup)
	if err != nil {
		return err
	}
	if r.CatanCitiesKnights != nil && *r.CatanCitiesKnights == normalized {
		return nil
	}
	r.CatanCitiesKnights = &normalized
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

// Both expansion fields are provisioned internally. HTTP commands only edit
// an existing field, so this does not enable a public combination entry point.
func (r *Room) validateCatanCitiesKnightsMap() error {
	if r.CatanBaseConfiguration != nil {
		return fmt.Errorf("城市与骑士不能使用基础地图配置")
	}
	if r.CatanSeafarers != nil {
		if !game.CatanCitiesKnightsSeafarersSupported(r.CatanSeafarers.Scenario) {
			return fmt.Errorf("该航海家剧本的城市骑士组合规则尚未接入")
		}
		if r.CatanSeafarers.Scenario == "new_world" {
			if r.CatanNewWorldMap == nil {
				return fmt.Errorf("请先确认新世界地图")
			}
			return nil
		}
	}
	if r.CatanNewWorldMap != nil {
		return fmt.Errorf("只有新世界组合可以使用预设地图")
	}
	return nil
}
