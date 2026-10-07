package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func publicCatanFishingSea(scenario string) bool {
	switch scenario {
	case "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world":
		return true
	}
	return false
}

func (r *Room) validateCatanFishing() error {
	if !r.CatanFishing {
		return nil
	}
	if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 4 || !publicCatanFishingSea(r.CatanScenario) || r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != r.CatanScenario || r.CatanCitiesKnights != nil || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.friendlyRobberEnabled() || (r.CatanHarbors != nil && r.CatanHarbors.Enabled) || r.CatanBaseConfiguration != nil {
		return fmt.Errorf("此渔夫与航海家组合支持三四人，不能混用其他扩展配置")
	}
	if (r.CatanScenario == "desert" || r.CatanScenario == "tribe") && r.CatanSeafarers.Layout != "fixed" {
		return fmt.Errorf("穿越沙漠和遗忘部落的捕鱼组合使用固定地图")
	}
	if r.CatanScenario == "new_world" {
		_, err := game.NewCatanFishingNewWorld(r.Capacity, r.CatanOptions, r.CatanNewWorldMap)
		return err
	}
	return nil
}

func (r *Room) setCatanFishing(enabled bool) error {
	if r.Kind != "catan" || r.Status != "waiting" || !publicCatanSeaScenario(r.CatanScenario) {
		return fmt.Errorf("只能在航海家等待房间选择渔夫组合")
	}
	next := *r
	next.CatanFishing = enabled
	if err := next.validateCatanScenario(); err != nil {
		return err
	}
	if r.CatanFishing == enabled {
		return nil
	}
	r.CatanFishing = enabled
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

func (r *Room) generateCatanWorldMap() (*game.CatanNewWorldMap, error) {
	if r.CatanFishing {
		return game.GenerateCatanFishingNewWorldMap(max(3, r.Capacity))
	}
	return game.GenerateCatanNewWorldMap(max(3, r.Capacity))
}
