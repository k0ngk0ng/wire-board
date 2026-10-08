package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Called on command's private room copy. Public rooms only expose their
// accepted scenario catalogue; internal combination recipes remain separate.
func (r *Room) setCatanSeafarers(request game.CatanSeafarersSetup) error {
	if r.CatanTwoRules != "" {
		return r.setCatanTwoSeafarers(request)
	}
	if r.Kind != "catan" || r.Status != "waiting" || r.CatanBaseConfiguration != nil {
		return fmt.Errorf("该房间尚不能与航海家剧本组合")
	}
	if publicCatanSeaScenario(r.CatanScenario) && !publicCatanSeaScenario(request.Scenario) {
		return fmt.Errorf("该航海家剧本尚未开放")
	}
	if r.friendlyRobberEnabled() && !game.CatanFriendlySeafarersSupported(max(3, r.Capacity), request.Scenario) {
		return fmt.Errorf("此人数或剧本暂不支持友善强盗，请更换剧本或关闭此变体")
	}
	if r.CatanCitiesKnights != nil && !game.CatanCitiesKnightsSeafarersSupported(request.Scenario) {
		return fmt.Errorf("该航海家剧本的城市骑士组合规则尚未接入")
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
		world, err = r.generateCatanWorldMap()
		if err != nil {
			return err
		}
	}
	r.CatanSeafarers, r.CatanNewWorldMap = &setup, world
	if publicCatanSeaScenario(r.CatanScenario) {
		r.CatanScenario = setup.Scenario
	}
	if err := r.validateCatanFishing(); err != nil {
		return err
	}
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
