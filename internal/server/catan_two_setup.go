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
	if game.CatanTwoSeafarersScenario(scenario) {
		if r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != scenario {
			setup, err := game.NormalizeCatanTwoSeafarersSetup(game.CatanSeafarersSetup{Scenario: scenario})
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
	} else {
		next.CatanSeafarers, next.CatanNewWorldMap = nil, nil
		next.CatanCitiesKnights = nil
		if scenario != "cities-knights" && scenario != "rivers" && scenario != "caravans" && scenario != "barbarian-attack" {
			next.CatanFishing = false
		}
	}
	if r.Capacity == 2 && publicCatanFlexibleScenario(r.CatanScenario) {
		next.CatanScenario = ""
		next.CatanRiversWorldMap = nil
		next.CatanOptions = game.CatanOptions{}
		next.CatanFishing, next.CatanFishingLakes = false, false
	}
	if err := next.validateCatanTwoSetup(); err != nil {
		return err
	}
	if r.Capacity == next.Capacity && r.CatanTwoRules == next.CatanTwoRules && r.CatanTwoScenario == next.CatanTwoScenario {
		return nil
	}
	r.Capacity, r.CatanTwoRules = next.Capacity, next.CatanTwoRules
	r.CatanTwoScenario = next.CatanTwoScenario
	r.CatanRiversWorldMap = next.CatanRiversWorldMap
	r.CatanScenario = next.CatanScenario
	r.CatanSeafarers, r.CatanNewWorldMap = next.CatanSeafarers, next.CatanNewWorldMap
	r.CatanOptions = next.CatanOptions
	r.CatanCitiesKnights = next.CatanCitiesKnights
	r.CatanFishing, r.CatanFishingLakes = next.CatanFishing, next.CatanFishingLakes
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

func (r *Room) validateCatanTwoSetup() error {
	if r.CatanTwoRules == "" && r.CatanTwoScenario == "" {
		return nil
	}
	if r.CatanTwoScenario != "" && r.CatanTwoScenario != "rivers" && r.CatanTwoScenario != "caravans" && r.CatanTwoScenario != "fishing" && r.CatanTwoScenario != "cities-knights" && r.CatanTwoScenario != "barbarian-attack" && !game.CatanTwoSeafarersScenario(r.CatanTwoScenario) {
		return errors.New("双人剧本尚未接入")
	}
	if r.Kind != "catan" || r.CatanTwoRules != game.CatanTwoRules || r.Capacity != 2 || len(r.Seats) > 2 || !game.CatanTwoHelpersOptions(r.CatanTwoScenario, r.CatanOptions) || r.CatanFishing && !r.catanRiverRecipe() && !r.catanCaravanRecipe() && !r.catanAttackRecipe() && !r.twoCatanSeafarers() && r.CatanTwoScenario != "cities-knights" || r.CatanFishingLakes || r.CatanScenario != "" || r.CatanCitiesKnights != nil && !r.twoCatanSeafarers() && !r.catanCaravanRecipe() && !r.catanRiverRecipe() && !r.catanAttackRecipe() || r.CatanBaseConfiguration != nil || !r.twoCatanSeafarers() && (r.CatanSeafarers != nil || r.CatanNewWorldMap != nil) {
		return errors.New("双人卡坦人数、版本或尚未核对的组合无效")
	}
	if r.CatanCitiesKnights != nil {
		setup, err := r.normalizeCatanCombinationKnights(*r.CatanCitiesKnights)
		if err != nil || setup != *r.CatanCitiesKnights {
			return errors.New("双人海图骑士配置无效")
		}
	}
	if r.twoCatanSeafarers() {
		if r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != r.CatanTwoScenario {
			return errors.New("双人海图配置缺失")
		}
		setup, err := game.NormalizeCatanTwoSeafarersSetup(*r.CatanSeafarers)
		if err != nil || setup != *r.CatanSeafarers {
			return errors.New("双人海图配置无效")
		}
		if setup.Scenario == "new_world" {
			if err = game.ValidateCatanNewWorldMap(4, r.CatanNewWorldMap); err != nil {
				return err
			}
		} else if r.CatanNewWorldMap != nil {
			return errors.New("此双人海图不能包含新世界地形")
		}
	}
	if r.CatanFishing {
		if err := r.validateCatanTwoFishing(); err != nil {
			return err
		}
	}
	if (r.friendlyRobberEnabled() || r.CatanHarbors != nil && r.CatanHarbors.Enabled) && !r.publicCatanTwoVariantsAvailable() {
		return errors.New("此双人剧本的友善／港口组合尚未接通，请先关闭变体")
	}
	return nil
}

func (r *Room) publicCatanTwoVariantsAvailable() bool {
	return r.Kind == "catan" && r.Capacity == 2 && r.CatanTwoRules == game.CatanTwoRules && r.CatanScenario == "" && (r.CatanTwoScenario == "" || r.CatanTwoScenario == "fishing" || r.CatanTwoScenario == "cities-knights" || game.CatanTwoSeafarersScenario(r.CatanTwoScenario)) && game.CatanTwoHelpersOptions(r.CatanTwoScenario, r.CatanOptions)
}
