package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Public creation cannot provision this setting until expansion acceptance.
func (r *Room) setCatanBaseConfiguration(request game.CatanBaseConfiguration) error {
	if r.Kind != "catan" || r.Status != "waiting" || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanCitiesKnights != nil {
		return fmt.Errorf("基础布局只能用于尚未开局的基础卡坦岛房间")
	}
	setup, err := game.NormalizeCatanBaseConfiguration(max(3, r.Capacity), request)
	if err != nil {
		return err
	}
	if (r.Capacity > 4) != r.CatanOptions.FiveSix {
		return fmt.Errorf("布局人数与五至六人扩充不一致")
	}
	if r.CatanBaseConfiguration != nil && *r.CatanBaseConfiguration == setup {
		return nil
	}
	r.CatanBaseConfiguration = &setup
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
