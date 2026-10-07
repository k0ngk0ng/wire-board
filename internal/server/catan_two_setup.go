package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Two-seat creation pins the rules; waiting hosts can choose among verified
// base, Rivers and Caravans recipes before all players ready up again.
func (r *Room) setCatanTwo() error {
	return r.setCatanTwoScenario("")
}

func (r *Room) setCatanTwoScenario(scenario string) error {
	if r.Kind != "catan" || r.Status != "waiting" || len(r.Seats) > 2 {
		return errors.New("双人变体只能设置在至多两人的待开始牌桌")
	}
	next := *r
	next.Capacity, next.CatanTwoRules = 2, game.CatanTwoRules
	next.CatanTwoScenario = scenario
	if err := next.validateCatanTwoSetup(); err != nil {
		return err
	}
	if r.Capacity == next.Capacity && r.CatanTwoRules == next.CatanTwoRules && r.CatanTwoScenario == next.CatanTwoScenario {
		return nil
	}
	r.Capacity, r.CatanTwoRules = next.Capacity, next.CatanTwoRules
	r.CatanTwoScenario = next.CatanTwoScenario
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

func (r *Room) validateCatanTwoSetup() error {
	if r.CatanTwoRules == "" && r.CatanTwoScenario == "" {
		return nil
	}
	if r.CatanTwoScenario != "" && r.CatanTwoScenario != "rivers" && r.CatanTwoScenario != "caravans" {
		return errors.New("双人剧本尚未接入")
	}
	if r.Kind != "catan" || r.CatanTwoRules != game.CatanTwoRules || r.Capacity != 2 || len(r.Seats) > 2 || r.CatanOptions != (game.CatanOptions{}) || r.CatanScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanCitiesKnights != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
		return errors.New("双人卡坦人数、版本或尚未核对的组合无效")
	}
	return nil
}
