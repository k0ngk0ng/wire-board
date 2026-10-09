package server

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func publicCatanSeaScenario(scenario string) bool {
	switch scenario {
	case "shores", "islands", "fog", "desert", "tribe", "cloth", "pirate_islands", "wonders", "new_world":
		return true
	}
	return false
}

func publicCatanExplorerScenario(scenario string) bool {
	return scenario == "land-ho" || publicCatanExplorerExtended(scenario)
}

// Explorer has its own player-count rules; only Helpers belong in this field.
func validCatanExplorerOptions(options game.CatanOptions) bool {
	normalized, err := game.NormalizeCatanOptions(options)
	return err == nil && normalized == options && !options.FiveSix
}

func publicCatanExplorerExtended(scenario string) bool {
	return scenario == "spices-for-catan" || scenario == "pirate-lairs" || scenario == "fish-for-catan" || scenario == "explorers-and-pirates"
}

func publicCatanTradersCombination(scenario string) bool {
	switch scenario {
	case "rivers-caravans", "rivers-attack", "rivers-transport", "caravans-attack", "caravans-transport", "attack-transport":
		return true
	}
	return false
}

func publicCatanFlexibleScenario(scenario string) bool {
	return scenario == "caravans-islands" || scenario == "caravans-shores" || scenario == "caravans-desert" || scenario == "caravans-tribe" || publicCatanRiversSea(scenario) || publicCatanExplorerScenario(scenario) || scenario == "transport" || (publicCatanTradersCombination(scenario))
}

// Public recipes include two-to-six-player Explorer missions.
// The other scenarios/combinations retain
// their separate acceptance gates; public sea rooms carry both the selected
// scenario and its normalized map configuration.
func (r *Room) validateCatanScenario() error {
	if r.CatanRiversWorldMap != nil && (r.Kind != "catan" || r.CatanScenario != "rivers-new-world") {
		return errors.New("河流预备地图与剧本不匹配")
	}
	if err := r.validateCatanEvents(); err != nil {
		return err
	}
	if err := r.validateCatanFishing(); err != nil {
		return err
	}
	if r.friendlyRobberEnabled() {
		if err := r.validateCatanFriendlyRobber(max(3, r.Capacity)); err != nil {
			return err
		}
	}
	if r.CatanScenario == "" {
		return nil
	}
	if r.CatanScenario == "caravans-islands" || r.CatanScenario == "caravans-shores" || r.CatanScenario == "caravans-desert" || r.CatanScenario == "caravans-tribe" {
		maximum := 6
		minimum := 2
		if r.CatanScenario == "caravans-islands" {
			minimum, maximum = 3, 4
		}

		if r.Kind != "catan" || r.Capacity < minimum || r.Capacity > maximum || len(r.Seats) > r.Capacity || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanBaseConfiguration != nil || r.CatanCitiesKnights != nil || r.CatanFishing || r.CatanFishingLakes || r.CatanHarbors != nil || r.CatanFriendlyRobber != nil {
			return errors.New("商队海图人数或组合无效；四岛支持三四人，其余支持二至六人")
		}
		return nil
	}
	if publicCatanRiversSea(r.CatanScenario) {
		return r.validateCatanRiversSea()
	}
	if publicCatanTradersCombination(r.CatanScenario) {
		if r.Kind != "catan" || r.Capacity < 2 || r.Capacity > 6 || len(r.Seats) > r.Capacity || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanFishing || r.CatanFishingLakes {
			return errors.New("商人与蛮族组合支持二至六人；开局按人数启用双人规则或扩大地图，可叠加城市骑士和事件牌")
		}
		if r.CatanCitiesKnights != nil {
			setup, err := r.normalizeCatanCombinationKnights(*r.CatanCitiesKnights)
			if err != nil || setup != *r.CatanCitiesKnights {
				return errors.New("组合城市骑士布局或规则版本无效")
			}
		}
		return nil
	}
	if r.CatanScenario == "transport" {
		if r.Kind != "catan" || r.Capacity < 2 || r.Capacity > 6 || len(r.Seats) > r.Capacity || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
			return errors.New("运输支持二至六人；按实际人数启用双人规则或五六人配对回合，不混用其他扩展配置")
		}
		if r.CatanCitiesKnights != nil {
			setup, err := r.normalizeCatanCombinationKnights(*r.CatanCitiesKnights)
			if err != nil || setup != *r.CatanCitiesKnights {
				return errors.New("运输城市骑士布局或规则版本无效")
			}
		}
		return nil
	}
	if r.CatanScenario == "barbarian-attack" {
		if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 6 || len(r.Seats) > r.Capacity || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
			return errors.New("蛮族进攻支持三至六人，开局按实际人数选择地图；不混用其他扩展配置")
		}
		if r.CatanCitiesKnights != nil {
			setup, err := r.normalizeCatanCombinationKnights(*r.CatanCitiesKnights)
			if err != nil || setup != *r.CatanCitiesKnights {
				return errors.New("蛮族城市骑士布局或规则版本无效")
			}
		}
		return nil
	}
	if r.CatanScenario == "cities-knights" {
		options, optionErr := game.NormalizeCatanOptions(game.CatanOptions{FiveSix: r.Capacity > 4, Helpers: r.CatanOptions.Helpers, AllHelpers: r.CatanOptions.AllHelpers})
		if optionErr != nil || r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 6 || len(r.Seats) > r.Capacity || r.CatanOptions != options || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanCitiesKnights == nil {
			return errors.New("城市与骑士支持三至六人随机地图，五六人需启用配对回合扩充；其他组合需使用对应配置")
		}
		setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, *r.CatanCitiesKnights)
		if err != nil || setup != *r.CatanCitiesKnights {
			return errors.New("城市与骑士布局或规则版本无效")
		}
		return nil
	}
	if publicCatanExplorerScenario(r.CatanScenario) {
		maximum := 6
		if r.Kind != "catan" || r.Capacity < 2 || r.Capacity > maximum || len(r.Seats) > r.Capacity || !validCatanExplorerOptions(r.CatanOptions) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
			return errors.New("探索者支持二至六人，城市骑士组合也支持双人；初航五六人或骑士使用本站自由开局规则")
		}
		if r.CatanCitiesKnights != nil {
			setup, err := r.normalizeCatanCombinationKnights(*r.CatanCitiesKnights)
			if err != nil || setup != *r.CatanCitiesKnights {
				return errors.New("探索者与城市骑士组合也支持双人随机地图")
			}
		}
		return nil
	}
	if publicCatanSeaScenario(r.CatanScenario) {
		maximum := 6
		if r.CatanFishing && !publicCatanFishingSeaExtended(r.CatanScenario) {
			maximum = 4
		}
		if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > maximum || len(r.Seats) > r.Capacity || (r.Capacity > 4) != r.CatanOptions.FiveSix {
			return errors.New("航海家支持三至六人；五六人必须启用人数扩充，渔夫组合支持新海岸、六岛、迷雾、沙漠、部落、布匹、奇迹和新世界")
		}
		if _, err := game.NormalizeCatanOptions(r.CatanOptions); err != nil {
			return err
		}
		if r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanBaseConfiguration != nil {
			return errors.New("所选航海家剧本与该扩展的组合尚未开放")
		}
		if r.CatanCitiesKnights != nil {
			setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, *r.CatanCitiesKnights)
			if err != nil || setup != *r.CatanCitiesKnights || !game.CatanCitiesKnightsSeafarersSupported(r.CatanScenario) {
				return errors.New("该航海家与城市骑士配置无效")
			}
		}
		if r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != r.CatanScenario {
			return errors.New("航海家剧本与地图配置不一致")
		}
		normalized, err := game.NormalizeCatanSeafarersSetup(r.Capacity, *r.CatanSeafarers)
		if err != nil || normalized != *r.CatanSeafarers {
			return errors.New("航海家地图布局或规则版本无效")
		}
		if r.CatanScenario == "new_world" {
			return game.ValidateCatanNewWorldMap(r.Capacity, r.CatanNewWorldMap)
		}
		if r.CatanNewWorldMap != nil {
			return errors.New("只有新世界可以使用开局前确认的地图")
		}
		return nil
	}
	if r.CatanScenario != "rivers" && r.CatanScenario != "caravans" && r.CatanScenario != "fishing" {
		return errors.New("未知卡坦剧本")
	}
	maximum := 6
	if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > maximum || len(r.Seats) > r.Capacity {
		return errors.New("该剧本人数无效；河流、商队和渔夫支持三至六人；河流和商队另有双人变体")
	}
	requested := game.CatanOptions{FiveSix: r.Capacity > 4}
	if r.CatanScenario == "fishing" {
		requested.Helpers, requested.AllHelpers = r.CatanOptions.Helpers, r.CatanOptions.AllHelpers
	}
	options, optionErr := game.NormalizeCatanOptions(requested)
	if optionErr != nil || r.CatanOptions != options || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || (r.CatanScenario != "fishing" && (r.CatanFriendlyRobber != nil || r.CatanHarbors != nil)) || (r.CatanCitiesKnights != nil && r.CatanScenario != "fishing" && r.CatanScenario != "caravans" && r.CatanScenario != "rivers") || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
		return errors.New("所选剧本与其他扩展的组合尚未开放")
	}
	if r.CatanCitiesKnights != nil {
		setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, *r.CatanCitiesKnights)
		if err != nil || setup != *r.CatanCitiesKnights {
			return errors.New("城市骑士组合布局或规则版本无效")
		}
	}
	return nil
}

func (r *Room) setCatanScenario(scenario string) error {
	if r.Kind != "catan" || r.Status != "waiting" || (r.CatanTwoRules != "" && !publicCatanFlexibleScenario(scenario)) {
		return errors.New("只能在对应人数的卡坦开局前选择剧本")
	}
	if r.Capacity > 4 && (publicCatanFlexibleScenario(r.CatanScenario) || r.CatanScenario == "barbarian-attack") && !publicCatanExplorerScenario(scenario) && scenario != "barbarian-attack" && scenario != "transport" && (!publicCatanTradersCombination(scenario)) && !publicCatanRiversSea(scenario) && scenario != "caravans-tribe" && scenario != "caravans-desert" && scenario != "caravans-shores" {
		return errors.New("五六席牌桌只能选择支持该人数的剧本")
	}
	next := *r
	next.CatanScenario = scenario
	if scenario != "rivers-new-world" {
		next.CatanRiversWorldMap = nil
	}
	if (scenario == "caravans-islands" || scenario == "caravans-shores" || scenario == "caravans-desert" || scenario == "caravans-tribe") || (r.CatanScenario == "caravans-islands" || r.CatanScenario == "caravans-shores" || r.CatanScenario == "caravans-desert" || r.CatanScenario == "caravans-tribe") || publicCatanTradersCombination(scenario) || publicCatanRiversSea(scenario) || publicCatanRiversSea(r.CatanScenario) {
		next.CatanOptions = game.CatanOptions{}
	}
	if publicCatanExplorerScenario(scenario) && !publicCatanExplorerScenario(r.CatanScenario) {
		next.CatanOptions, _ = game.NormalizeCatanOptions(game.CatanOptions{Helpers: r.CatanOptions.Helpers, AllHelpers: r.CatanOptions.AllHelpers})
	}
	if scenario != "" {
		next.CatanBaseConfiguration = nil
	}
	if scenario != "" && scenario != "cities-knights" && scenario != "fishing" && !publicCatanSeaScenario(scenario) {
		next.CatanFriendlyRobber = nil
	}
	if !publicCatanHarborsScenario(scenario) {
		next.CatanHarbors = nil
	}
	if !publicCatanExplorerScenario(scenario) {
		next.CatanFishingLakes = false
	}
	if scenario != "rivers" && scenario != "caravans" && scenario != "barbarian-attack" && scenario != "transport" && !publicCatanSeaScenario(scenario) && !publicCatanExplorerScenario(scenario) {
		next.CatanFishing = false
	}
	if publicCatanFlexibleScenario(scenario) {
		next.CatanTwoRules, next.CatanTwoScenario = "", ""
	} else if r.Capacity == 2 {
		return errors.New("两人牌桌请通过双人剧本选择器切回双人规则")
	}
	if publicCatanSeaScenario(scenario) {
		if r.CatanSeafarers != nil && !publicCatanSeaScenario(r.CatanScenario) {
			return errors.New("此组合房间不能切换为普通航海剧本")
		}
		if r.CatanScenario != scenario || r.CatanSeafarers == nil {
			setup, err := game.NormalizeCatanSeafarersSetup(r.Capacity, game.CatanSeafarersSetup{Scenario: scenario})
			if err != nil {
				return err
			}
			next.CatanSeafarers = &setup
			next.CatanNewWorldMap = nil
			if scenario == "new_world" {
				next.CatanNewWorldMap, err = next.generateCatanWorldMap()
				if err != nil {
					return err
				}
			}
		}
	} else if publicCatanSeaScenario(r.CatanScenario) {
		next.CatanSeafarers = nil
		next.CatanNewWorldMap = nil
	}
	if (scenario == "caravans-islands" || scenario == "caravans-shores" || scenario == "caravans-desert" || scenario == "caravans-tribe") || (r.CatanScenario == "caravans-islands" || r.CatanScenario == "caravans-shores" || r.CatanScenario == "caravans-desert" || r.CatanScenario == "caravans-tribe") || publicCatanRiversSea(scenario) || publicCatanRiversSea(r.CatanScenario) {
		next.CatanCitiesKnights = nil
	}
	if scenario == "cities-knights" {
		setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, game.CatanCitiesKnightsSetup{})
		if err != nil {
			return err
		}
		next.CatanCitiesKnights = &setup
	} else if (publicCatanExplorerScenario(r.CatanScenario) && !publicCatanExplorerScenario(scenario)) || ((r.CatanScenario == "fishing" || r.CatanScenario == "caravans" || r.CatanScenario == "rivers" || r.CatanScenario == "transport" || (publicCatanTradersCombination(r.CatanScenario))) && scenario != r.CatanScenario) || r.CatanScenario == "cities-knights" || (publicCatanSeaScenario(r.CatanScenario) && !publicCatanSeaScenario(scenario)) {
		next.CatanCitiesKnights = nil
	}
	if !next.publicCatanEventsAvailable() {
		next.CatanEvents = ""
	}
	if err := next.validateCatanScenario(); err != nil {
		return err
	}
	if r.CatanScenario == scenario {
		return nil
	}
	r.CatanRiversWorldMap = next.CatanRiversWorldMap
	r.CatanScenario = scenario
	r.CatanOptions = next.CatanOptions
	r.CatanEvents = next.CatanEvents
	r.CatanBaseConfiguration = next.CatanBaseConfiguration
	r.CatanHarbors = next.CatanHarbors
	r.CatanFriendlyRobber = next.CatanFriendlyRobber
	r.CatanFishing = next.CatanFishing
	r.CatanFishingLakes = next.CatanFishingLakes
	r.CatanTwoRules, r.CatanTwoScenario = next.CatanTwoRules, next.CatanTwoScenario
	r.CatanCitiesKnights = next.CatanCitiesKnights
	r.CatanSeafarers = next.CatanSeafarers
	r.CatanNewWorldMap = next.CatanNewWorldMap
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
