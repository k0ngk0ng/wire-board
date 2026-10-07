package server

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func publicCatanSeaScenario(scenario string) bool {
	switch scenario {
	case "shores", "islands", "fog", "desert":
		return true
	}
	return false
}

// Public three/four-player recipes. The other scenarios/combinations retain
// their separate acceptance gates; public sea rooms carry both the selected
// scenario and its normalized map configuration.
func (r *Room) validateCatanScenario() error {
	if r.CatanScenario == "" {
		return nil
	}
	if publicCatanSeaScenario(r.CatanScenario) {
		if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 4 || len(r.Seats) > r.Capacity || r.CatanOptions.FiveSix {
			return errors.New("这些航海家剧本当前支持三至四人牌桌")
		}
		if _, err := game.NormalizeCatanOptions(r.CatanOptions); err != nil {
			return err
		}
		if r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanCitiesKnights != nil || r.CatanBaseConfiguration != nil || r.CatanNewWorldMap != nil {
			return errors.New("所选航海家剧本与该扩展的组合尚未开放")
		}
		if r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != r.CatanScenario {
			return errors.New("航海家剧本与地图配置不一致")
		}
		normalized, err := game.NormalizeCatanSeafarersSetup(r.Capacity, *r.CatanSeafarers)
		if err != nil || normalized != *r.CatanSeafarers {
			return errors.New("航海家地图布局或规则版本无效")
		}
		return nil
	}
	if r.CatanScenario != "rivers" && r.CatanScenario != "caravans" {
		return errors.New("未知卡坦剧本")
	}
	if r.Kind != "catan" || r.Capacity < 3 || r.Capacity > 4 || len(r.Seats) > r.Capacity {
		return errors.New("河流与商队当前支持三至四人牌桌；双人请使用双人变体")
	}
	if r.CatanOptions != (game.CatanOptions{}) || r.CatanTwoRules != "" || r.CatanTwoScenario != "" || r.CatanFriendlyRobber != nil || r.CatanHarbors != nil || r.CatanCitiesKnights != nil || r.CatanBaseConfiguration != nil || r.CatanSeafarers != nil || r.CatanNewWorldMap != nil {
		return errors.New("所选剧本与其他扩展的组合尚未开放")
	}
	return nil
}

func (r *Room) setCatanScenario(scenario string) error {
	if r.Kind != "catan" || r.Status != "waiting" || r.CatanTwoRules != "" {
		return errors.New("只能在三至四人卡坦开局前选择剧本")
	}
	next := *r
	next.CatanScenario = scenario
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
		}
	} else if publicCatanSeaScenario(r.CatanScenario) {
		next.CatanSeafarers = nil
	}
	if err := next.validateCatanScenario(); err != nil {
		return err
	}
	if r.CatanScenario == scenario {
		return nil
	}
	r.CatanScenario = scenario
	r.CatanSeafarers = next.CatanSeafarers
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}
