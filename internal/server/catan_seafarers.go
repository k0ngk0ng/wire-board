package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Called on command's private room copy. Creation deliberately cannot provision
// this field yet, so standard rooms cannot activate unfinished expansions.
func (r *Room) setCatanSeafarers(request game.CatanSeafarersSetup) error {
	if r.CatanBaseConfiguration != nil || r.CatanCitiesKnights != nil {
		return fmt.Errorf("该房间尚不能与航海家剧本组合")
	}
	setup, err := game.NormalizeCatanSeafarersSetup(max(3, r.Capacity), request)
	if err != nil {
		return err
	}
	if r.CatanSeafarers != nil && *r.CatanSeafarers == setup {
		return nil
	}
	var world *game.CatanNewWorldMap
	if setup.Scenario == "new_world" {
		world, err = game.GenerateCatanNewWorldMap(max(3, r.Capacity))
		if err != nil {
			return err
		}
	}
	r.CatanSeafarers, r.CatanNewWorldMap = &setup, world
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
