package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Public creation remains closed; commands can only edit a provisioned field.
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
