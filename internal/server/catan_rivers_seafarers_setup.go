package server

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func publicCatanRiversSeaSetup(scenario string) (game.CatanRiversSeafarersSetup, bool) {
	setup := game.CatanRiversSeafarersSetup{}
	switch scenario {
	case "rivers-shores":
		setup.Scenario = "shores"
	case "rivers-fog":
		setup.Scenario = "fog"
	case "rivers-desert":
		setup.Scenario = "desert"
		setup.Layout = "rivers-across"
	case "rivers-desert-belt":
		setup.Scenario = "desert"
		setup.Layout = "desert-belt"
	case "rivers-tribe":
		setup.Scenario = "tribe"
	case "rivers-new-world":
		setup.Scenario = "new_world"
	default:
		return setup, false
	}
	return setup, true
}
func publicCatanRiversSea(scenario string) bool {
	_, ok := publicCatanRiversSeaSetup(scenario)
	return ok
}
func (r *Room) validateCatanRiversSea() error {
	setup, ok := publicCatanRiversSeaSetup(r.CatanScenario)
	if !ok {
		return errors.New("河流海图剧本无效")
	}
	if r.Kind != "catan" || len(r.Seats) > r.Capacity || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanBaseConfiguration != nil || r.CatanCitiesKnights != nil || r.CatanHarbors != nil || r.CatanFriendlyRobber != nil || r.CatanFishing || r.CatanFishingLakes {
		return errors.New("河流海图暂支持独立组合与事件牌，其他叠加尚未接通")
	}
	_, err := game.NormalizeCatanRiversSeafarersSetup(r.Capacity, setup)
	return err
}
