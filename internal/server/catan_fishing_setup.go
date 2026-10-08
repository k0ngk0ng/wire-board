package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func publicCatanFishingSea(scenario string) bool {
	switch scenario {
	case "shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world":
		return true
	}
	return false
}

func publicCatanFishingSeaExtended(scenario string) bool {
	return publicCatanFishingSea(scenario)
}

func (r *Room) validateCatanFishing() error {
	if r.CatanFishingLakes && (!r.CatanFishing || !publicCatanExplorerScenario(r.CatanScenario)) {
		return fmt.Errorf("湖泊选项仅适用于探索者渔夫组合")
	}
	if !r.CatanFishing {
		return nil
	}
	if publicCatanExplorerScenario(r.CatanScenario) {
		if r.Kind != "catan" || !validCatanExplorerOptions(r.CatanOptions) || r.CatanSeafarers != nil || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanBaseConfiguration != nil {
			return fmt.Errorf("探索者渔夫不能混用航海家、普通双人或基础配置")
		}
		return nil // Scenario validation checks player counts and city compatibility.
	}
	expected, optionErr := game.NormalizeCatanOptions(game.CatanOptions{FiveSix: r.Capacity > 4, Helpers: r.CatanOptions.Helpers, AllHelpers: r.CatanOptions.AllHelpers})
	if optionErr != nil || r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 6 || !publicCatanFishingSea(r.CatanScenario) || (r.Capacity > 4 && !publicCatanFishingSeaExtended(r.CatanScenario)) || r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != r.CatanScenario || r.CatanCitiesKnights != nil || r.CatanOptions != expected || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanBaseConfiguration != nil {
		return fmt.Errorf("八种渔夫海图支持三至六人，可叠加助手、友善强盗与港口霸主，暂不支持海图骑士组合")
	}
	if (r.CatanScenario == "desert" || r.CatanScenario == "tribe" || r.Capacity > 4 && (r.CatanScenario == "islands" || r.CatanScenario == "cloth")) && r.CatanSeafarers.Layout != "fixed" {
		return fmt.Errorf("穿越沙漠、遗忘部落及五六人六岛、布匹的捕鱼组合使用固定地图")
	}
	if r.CatanScenario == "new_world" {
		_, err := game.NewCatanFishingNewWorld(r.Capacity, r.CatanOptions, r.CatanNewWorldMap)
		return err
	}
	return nil
}

func (r *Room) setCatanFishing(enabled bool) error {
	if r.Kind != "catan" || r.Status != "waiting" || (!publicCatanSeaScenario(r.CatanScenario) && !publicCatanExplorerScenario(r.CatanScenario)) {
		return fmt.Errorf("只能在航海家或探索者等待房间选择渔夫组合")
	}
	next := *r
	next.CatanFishing = enabled
	if !enabled {
		next.CatanFishingLakes = false
	}
	if err := next.validateCatanScenario(); err != nil {
		return err
	}
	if r.CatanFishing == enabled {
		return nil
	}
	r.CatanFishing = enabled
	r.CatanFishingLakes = next.CatanFishingLakes
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

func (r *Room) setCatanFishingLakes(enabled bool) error {
	if r.Kind != "catan" || r.Status != "waiting" || !r.CatanFishing || !publicCatanExplorerScenario(r.CatanScenario) {
		return fmt.Errorf("只能在探索者渔夫开局前选择湖泊")
	}
	next := *r
	next.CatanFishingLakes = enabled
	if err := next.validateCatanScenario(); err != nil {
		return err
	}
	if r.CatanFishingLakes == enabled {
		return nil
	}
	r.CatanFishingLakes = enabled
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
