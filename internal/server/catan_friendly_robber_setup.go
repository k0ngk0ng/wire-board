package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Public commands may edit an existing internally provisioned field only.
func (r *Room) setCatanFriendlyRobber(request game.CatanFriendlyRobberSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" {
		return errors.New("只能在卡坦岛开局前调整友善强盗")
	}
	setup, err := game.NormalizeCatanFriendlyRobberSetup(request)
	if err != nil {
		return err
	}
	if setup.Enabled && (r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanCitiesKnights != nil || r.CatanOptions.Helpers || r.CatanOptions.AllHelpers) {
		return errors.New("友善强盗目前仅核验基础版及港口霸主组合")
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
