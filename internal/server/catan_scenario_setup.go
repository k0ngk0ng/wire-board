package server

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Public three/four-player T&B recipes. Extended boards remain gated until
// the 2025 letter-to-number disc mapping is independently verified.
func (r *Room) validateCatanScenario() error {
	if r.CatanScenario == "" {
		return nil
	}
	if r.CatanScenario != "rivers" && r.CatanScenario != "caravans" {
		return errors.New("未知卡坦剧本")
	}
	if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 4 || len(r.Seats) > r.Capacity {
		return errors.New("河流与商队当前支持三至四人牌桌；双人请使用双人变体")
	}
	if r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanCitiesKnights != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
		return errors.New("所选剧本与其他扩展的组合尚未开放")
	}
	return nil
}

func (r *Room) setCatanScenario(scenario string) error {
	if r.Kind != "catan" || r.Status != "waiting" || r.CatanTwoRules != "" {
		return errors.New("只能在三至四人卡坦开局前选择剧本")
	}
	next := *r
	next.CatanScenario = scenario
	if err := next.validateCatanScenario(); err != nil {
		return err
	}
	if r.CatanScenario == scenario {
		return nil
	}
	r.CatanScenario = scenario
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
