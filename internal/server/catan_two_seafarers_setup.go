package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func (r *Room) twoCatanSeafarers() bool {
	return r.Kind == "catan" && r.Capacity == 2 && r.CatanTwoRules == game.CatanTwoRules && game.CatanTwoSeafarersScenario(r.CatanTwoScenario)
}
func (r *Room) setCatanTwoSeafarers(request game.CatanSeafarersSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" || r.Capacity != 2 || r.CatanTwoRules != game.CatanTwoRules {
		return errors.New("只能在双人等待房间选择航海家地图")
	}
	setup, err := game.NormalizeCatanTwoSeafarersSetup(request)
	if err != nil {
		return err
	}
	if r.CatanSeafarers != nil && *r.CatanSeafarers == setup && r.CatanTwoScenario == setup.Scenario {
		return nil
	}
	next := *r
	next.CatanTwoScenario = setup.Scenario
	next.CatanSeafarers = &setup
	next.CatanNewWorldMap = nil
	if setup.Scenario == "new_world" {
		next.CatanNewWorldMap, err = next.generateCatanWorldMap()
		if err != nil {
			return err
		}
	}
	if err = next.validateCatanTwoSetup(); err != nil {
		return err
	}
	r.CatanTwoScenario, r.CatanSeafarers, r.CatanNewWorldMap = next.CatanTwoScenario, next.CatanSeafarers, next.CatanNewWorldMap
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

func (r *Room) validateCatanTwoFishingSeafarers() error {
	if !r.twoCatanSeafarers() || r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != r.CatanTwoScenario || r.CatanScenario != "" || r.CatanFishingLakes || r.CatanCitiesKnights != nil || r.CatanBaseConfiguration != nil || !game.CatanTwoHelpersOptions(r.CatanTwoScenario, r.CatanOptions) {
		return errors.New("双人捕鱼需要适用的航海家剧本，骑士组合尚未接通")
	}
	setup, err := game.NormalizeCatanTwoFishingSeafarersSetup(*r.CatanSeafarers)
	if err != nil {
		return err
	}
	if setup != *r.CatanSeafarers {
		return errors.New("双人捕鱼海图配置无效")
	}
	if setup.Scenario == "new_world" {
		_, err = game.NewCatanTwoFishingSeafarers(2, r.CatanOptions, setup, r.CatanNewWorldMap)
	}
	return err
}
