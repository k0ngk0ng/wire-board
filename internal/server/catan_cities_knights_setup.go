package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Provisioned internally until the expansion UI and combinations are accepted.
func (r *Room) setCatanCitiesKnights(setup game.CatanCitiesKnightsSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" {
		return fmt.Errorf("只能在城市与骑士开局前调整设置")
	}
	if r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
		return fmt.Errorf("城市与骑士尚不支持与其他地图配置组合")
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
