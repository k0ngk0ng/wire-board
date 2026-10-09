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
	if publicCatanTransportSea(r.CatanScenario) {
		if r.Kind != "catan" || r.Capacity < 2 || r.Capacity > 6 || !validCatanTradersRoomOptions(r.CatanOptions) || r.CatanFishingLakes || r.CatanSeafarers != nil || r.CatanBaseConfiguration != nil {
			return fmt.Errorf("运输捕鱼海图配置无效")
		}
		return nil
	}
	if r.catanRiverRecipe() || r.catanCaravanRecipe() || r.catanAttackRecipe() || r.CatanScenario == "transport" {
		expected, _ := game.NormalizeCatanOptions(game.CatanOptions{FiveSix: r.Capacity > 4, Helpers: r.CatanOptions.Helpers, AllHelpers: r.CatanOptions.AllHelpers})
		if r.catanAttackRecipe() || r.CatanScenario == "transport" {
			expected, _ = game.NormalizeCatanOptions(game.CatanOptions{Helpers: r.CatanOptions.Helpers, AllHelpers: r.CatanOptions.AllHelpers})
		}
		if r.Kind != "catan" || r.Capacity < 2 || r.Capacity > 6 || r.CatanOptions != expected || r.CatanFishingLakes || r.CatanSeafarers != nil || r.CatanBaseConfiguration != nil {
			return fmt.Errorf("渔夫河流、商队、蛮族进攻或运输组合配置无效")
		}
		return nil
	}
	if r.CatanTwoRules != "" {
		return r.validateCatanTwoFishing()
	}
	if publicCatanExplorerScenario(r.CatanScenario) {
		if r.Kind != "catan" || !validCatanExplorerOptions(r.CatanOptions) || r.CatanSeafarers != nil || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanBaseConfiguration != nil {
			return fmt.Errorf("探索者渔夫不能混用航海家、普通双人或基础配置")
		}
		return nil // Scenario validation checks player counts and city compatibility.
	}
	expected, optionErr := game.NormalizeCatanOptions(game.CatanOptions{FiveSix: r.Capacity > 4, Helpers: r.CatanOptions.Helpers, AllHelpers: r.CatanOptions.AllHelpers})
	if optionErr != nil || r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 6 || !publicCatanFishingSea(r.CatanScenario) || (r.Capacity > 4 && !publicCatanFishingSeaExtended(r.CatanScenario)) || r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != r.CatanScenario || r.CatanOptions != expected || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanBaseConfiguration != nil {
		return fmt.Errorf("八种渔夫海图支持三至六人，可叠加友善强盗、港口霸主、助手与城市骑士")
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
	if r.Kind != "catan" || r.Status != "waiting" || (!r.catanRiverRecipe() && !r.catanCaravanRecipe() && !r.catanAttackRecipe() && r.CatanScenario != "transport" && !publicCatanTransportSea(r.CatanScenario) && !r.twoCatanSeafarers() && !r.twoCatanFishingKnights() && !publicCatanSeaScenario(r.CatanScenario) && !publicCatanExplorerScenario(r.CatanScenario)) {
		return fmt.Errorf("只能在河流、商队、蛮族进攻、运输、航海家或探索者等待房间选择渔夫组合")
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
	if r.CatanScenario == "caravans-new-world" {
		return game.GenerateCatanCaravansWorldMap(r.Capacity)
	}
	if r.twoCatanSeafarers() {
		if r.CatanFishing {
			return game.GenerateCatanFishingNewWorldMap(4)
		}
		return game.GenerateCatanNewWorldMap(4)
	}
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

func (r *Room) twoCatanFishingKnights() bool {
	return r.Kind == "catan" && r.Capacity == 2 && r.CatanTwoRules == game.CatanTwoRules && r.CatanTwoScenario == "cities-knights"
}
func (r *Room) validateCatanTwoFishing() error {
	if r.catanRiverRecipe() || r.catanCaravanRecipe() || r.catanAttackRecipe() || r.CatanScenario == "transport" {
		return r.validateCatanFishing()
	}
	if r.twoCatanFishingKnights() {
		if !game.CatanTwoHelpersOptions("cities-knights", r.CatanOptions) || r.CatanScenario != "" || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanFishingLakes || r.CatanCitiesKnights != nil || r.CatanBaseConfiguration != nil {
			return fmt.Errorf("双人渔夫骑士配置无效")
		}
		return nil
	}
	return r.validateCatanTwoFishingSeafarers()
}
