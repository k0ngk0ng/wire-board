package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Only internal provisioning can enable this draft. It is intentionally absent
// from public creation/selection requests until complete expansion acceptance.
func (r *Room) setCatanTwo() error {
	if r.Kind != "catan" || r.Status != "waiting" || len(r.Seats) > 2 {
		return errors.New("双人变体只能设置在至多两人的待开始牌桌")
	}
	next := *r
	next.Capacity, next.CatanTwoRules = 2, game.CatanTwoRules
	if err := next.validateCatanTwoSetup(); err != nil {
		return err
	}
	r.Capacity, r.CatanTwoRules = next.Capacity, next.CatanTwoRules
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

func (r *Room) validateCatanTwoSetup() error {
	if r.CatanTwoRules == "" {
		return nil
	}
	if r.Kind != "catan" || r.CatanTwoRules != game.CatanTwoRules || r.Capacity != 2 || len(r.Seats) > 2 || r.CatanOptions != (game.CatanOptions{}) || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanCitiesKnights != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
		return errors.New("双人卡坦人数、版本或尚未核对的组合无效")
	}
	return nil
}
