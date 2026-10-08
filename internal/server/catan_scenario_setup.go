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

func publicCatanExplorerExtended(scenario string) bool {
	return scenario == "spices-for-catan" || scenario == "pirate-lairs" || scenario == "fish-for-catan" || scenario == "explorers-and-pirates"
}

func publicCatanFlexibleScenario(scenario string) bool {
	return publicCatanExplorerScenario(scenario) || scenario == "transport"
}

// Public recipes include two-to-six-player Explorer missions.
// The other scenarios/combinations retain
// their separate acceptance gates; public sea rooms carry both the selected
// scenario and its normalized map configuration.
func (r *Room) validateCatanScenario() error {
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
	if r.CatanScenario == "transport" {
		if r.Kind != "catan" || r.Capacity < 2 || r.Capacity > 6 || len(r.Seats) > r.Capacity || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanCitiesKnights != nil {
			return errors.New("运输支持二至六人；按实际人数启用双人规则或五六人配对回合，不混用其他扩展配置")
		}
		return nil
	}
	if r.CatanScenario == "barbarian-attack" {
		if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 6 || len(r.Seats) > r.Capacity || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanCitiesKnights != nil {
			return errors.New("蛮族进攻支持三至六人，开局按实际人数选择地图；不混用其他扩展配置")
		}
		return nil
	}
	if r.CatanScenario == "cities-knights" {
		options, optionErr := game.NormalizeCatanOptions(game.CatanOptions{FiveSix: r.Capacity > 4})
		if optionErr != nil || r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 6 || len(r.Seats) > r.Capacity || r.CatanOptions != options || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.friendlyRobberEnabled() || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil || r.CatanCitiesKnights == nil {
			return errors.New("城市与骑士支持三至六人随机地图，五六人需启用配对回合扩充；其他组合需使用对应配置")
		}
		setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, *r.CatanCitiesKnights)
		if err != nil || setup != *r.CatanCitiesKnights {
			return errors.New("城市与骑士布局或规则版本无效")
		}
		return nil
	}
	if publicCatanExplorerScenario(r.CatanScenario) {
		maximum := 4
		if publicCatanExplorerExtended(r.CatanScenario) {
			maximum = 6
		}
		if r.Kind != "catan" || r.Capacity < 2 || r.Capacity > maximum || len(r.Seats) > r.Capacity || r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
			return errors.New("探险剧本人数或组合无效：初航两至四人，其余任务两至六人；城市骑士组合需三至六人且不适用于初航")
		}
		if r.CatanCitiesKnights != nil {
			setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, *r.CatanCitiesKnights)
			if !publicCatanExplorerExtended(r.CatanScenario) || err != nil || setup != *r.CatanCitiesKnights {
				return errors.New("探索任务与城市骑士组合需三至六人随机地图；初航不支持该组合")
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
			return errors.New("航海家支持三至六人；五六人必须启用人数扩充，渔夫组合目前仅迷雾、奇迹、新世界支持五六人")
		}
		if r.Capacity > 4 && ((r.CatanHarbors != nil && r.CatanHarbors.Enabled) || r.friendlyRobberEnabled()) {
			return errors.New("五六人航海家与港口霸主或友善强盗的公开组合尚未接通")
		}
		if _, err := game.NormalizeCatanOptions(r.CatanOptions); err != nil {
			return err
		}
		if r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanBaseConfiguration != nil {
			return errors.New("所选航海家剧本与该扩展的组合尚未开放")
		}
		if r.CatanCitiesKnights != nil {
			setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, *r.CatanCitiesKnights)
			if err != nil || setup != *r.CatanCitiesKnights || r.CatanOptions.Helpers || r.CatanOptions.AllHelpers || !game.CatanCitiesKnightsSeafarersSupported(r.CatanScenario) {
				return errors.New("该航海家与城市骑士配置无效，不能混用Helpers")
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
	maximum := 4
	if r.CatanScenario == "rivers" || r.CatanScenario == "caravans" {
		maximum = 6
	}
	if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > maximum || len(r.Seats) > r.Capacity {
		return errors.New("该剧本人数无效；河流和商队支持三至六人，双人请使用对应双人变体")
	}
	options, _ := game.NormalizeCatanOptions(game.CatanOptions{FiveSix: r.Capacity > 4})
	if r.CatanOptions != options || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || (r.CatanCitiesKnights != nil && r.CatanScenario != "fishing") || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
		return errors.New("所选剧本与其他扩展的组合尚未开放")
	}
	if r.CatanCitiesKnights != nil {
		setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, *r.CatanCitiesKnights)
		if err != nil || setup != *r.CatanCitiesKnights {
			return errors.New("渔夫与城市骑士布局或规则版本无效")
		}
	}
	return nil
}

func (r *Room) setCatanScenario(scenario string) error {
	if r.Kind != "catan" || r.Status != "waiting" || (r.CatanTwoRules != "" && !publicCatanFlexibleScenario(scenario)) {
		return errors.New("只能在对应人数的卡坦开局前选择剧本")
	}
	if r.Capacity > 4 && (publicCatanFlexibleScenario(r.CatanScenario) || r.CatanScenario == "barbarian-attack") && !publicCatanExplorerExtended(scenario) && scenario != "barbarian-attack" && scenario != "transport" {
		return errors.New("五六席牌桌只能选择支持该人数的剧本")
	}
	next := *r
	next.CatanScenario = scenario
	if scenario != "" {
		next.CatanBaseConfiguration = nil
	}
	if scenario != "" && !publicCatanSeaScenario(scenario) {
		next.CatanFriendlyRobber = nil
	}
	if !publicCatanHarborsScenario(scenario) {
		next.CatanHarbors = nil
	}
	if !publicCatanSeaScenario(scenario) {
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
	if scenario == "cities-knights" {
		setup, err := game.NormalizeCatanCitiesKnightsSetup(r.Capacity, game.CatanCitiesKnightsSetup{})
		if err != nil {
			return err
		}
		next.CatanCitiesKnights = &setup
	} else if (publicCatanExplorerExtended(r.CatanScenario) && !publicCatanExplorerExtended(scenario)) || (r.CatanScenario == "fishing" && scenario != "fishing") || r.CatanScenario == "cities-knights" || (publicCatanSeaScenario(r.CatanScenario) && !publicCatanSeaScenario(scenario)) {
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
	r.CatanScenario = scenario
	r.CatanEvents = next.CatanEvents
	r.CatanBaseConfiguration = next.CatanBaseConfiguration
	r.CatanHarbors = next.CatanHarbors
	r.CatanFriendlyRobber = next.CatanFriendlyRobber
	r.CatanFishing = next.CatanFishing
	r.CatanTwoRules, r.CatanTwoScenario = next.CatanTwoRules, next.CatanTwoScenario
	r.CatanCitiesKnights = next.CatanCitiesKnights
	r.CatanSeafarers = next.CatanSeafarers
	r.CatanNewWorldMap = next.CatanNewWorldMap
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
