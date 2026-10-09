package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
)

func (r *Room) isCatanStandaloneKnightsRecipe() bool {
	return r.Kind == "catan" && r.CatanScenario == "cities-knights" && r.CatanCitiesKnights != nil && r.CatanSeafarers == nil && r.CatanNewWorldMap == nil && r.CatanBaseConfiguration == nil && r.CatanTwoRules == "" && !r.CatanFishing
}

// Shared by public standalone three-to-six-player rooms and internal combinations.
func (r *Room) setCatanCitiesKnights(setup game.CatanCitiesKnightsSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" {
		return fmt.Errorf("只能在城市与骑士开局前调整设置")
	}
	if err := r.validateCatanCitiesKnightsMap(); err != nil {
		return err
	}
	if (r.Capacity > 4) != r.CatanOptions.FiveSix {
		return fmt.Errorf("城市与骑士人数与扩充不一致")
	}
	normalized, err := game.NormalizeCatanCitiesKnightsSetup(max(3, r.Capacity), setup)
	if err != nil {
		return err
	}
	if r.CatanCitiesKnights != nil && *r.CatanCitiesKnights == normalized {
		return nil
	}
	r.CatanCitiesKnights = &normalized
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

// Both expansion fields are provisioned internally. HTTP commands only edit
// an existing field, so this does not enable a public combination entry point.
func (r *Room) validateCatanCitiesKnightsMap() error {
	if r.CatanBaseConfiguration != nil {
		return fmt.Errorf("城市与骑士不能使用基础地图配置")
	}
	if r.CatanSeafarers != nil {
		if !game.CatanCitiesKnightsSeafarersSupported(r.CatanSeafarers.Scenario) {
			return fmt.Errorf("该航海家剧本的城市骑士组合规则尚未接入")
		}
		if r.CatanSeafarers.Scenario == "new_world" {
			if r.CatanNewWorldMap == nil {
				return fmt.Errorf("请先确认新世界地图")
			}
			return nil
		}
	}
	if r.CatanNewWorldMap != nil {
		return fmt.Errorf("只有新世界组合可以使用预设地图")
	}
	return nil
}

func (r *Room) catanAttackRecipe() bool {
	return r.CatanScenario == "barbarian-attack" || r.CatanTwoRules == game.CatanTwoRules && r.CatanTwoScenario == "barbarian-attack"
}

func (r *Room) catanRiverRecipe() bool {
	return r.CatanScenario == "rivers" || r.CatanTwoRules == game.CatanTwoRules && r.CatanTwoScenario == "rivers"
}

func (r *Room) catanCaravanRecipe() bool {
	return r.CatanScenario == "caravans" || r.CatanTwoRules == game.CatanTwoRules && r.CatanTwoScenario == "caravans"
}

// Public sea, Fishing, Rivers, Caravans and Explorer rooms may toggle while waiting. Every
// other recipe keeps its own configuration path; nil means remove only here.
func (r *Room) setPublicCatanCombinationKnights(setup *game.CatanCitiesKnightsSetup) error {
	if r.Kind != "catan" || r.Status != "waiting" || (!publicCatanSeaScenario(r.CatanScenario) && r.CatanScenario != "fishing" && !publicCatanExplorerScenario(r.CatanScenario) && !r.twoCatanSeafarers() && !r.catanCaravanRecipe() && !r.catanRiverRecipe() && r.CatanScenario != "transport" && (r.CatanScenario != "rivers-caravans" && (r.CatanScenario != "rivers-attack" && r.CatanScenario != "rivers-transport")) && !r.catanAttackRecipe()) {
		return fmt.Errorf("只能在航海家、渔夫、河流、商队、运输、蛮族进攻或探索任务等待房间切换城市与骑士组合")
	}
	next := *r
	next.CatanCitiesKnights = nil
	if setup != nil {
		normalized, err := r.normalizeCatanCombinationKnights(*setup)
		if err != nil {
			return err
		}
		next.CatanCitiesKnights = &normalized
	}
	if err := next.validateCatanScenario(); err != nil {
		return err
	}
	if err := next.validateCatanTwoSetup(); err != nil {
		return err
	}
	if (r.CatanCitiesKnights == nil && next.CatanCitiesKnights == nil) || (r.CatanCitiesKnights != nil && next.CatanCitiesKnights != nil && *r.CatanCitiesKnights == *next.CatanCitiesKnights) {
		return nil
	}
	r.CatanCitiesKnights = next.CatanCitiesKnights
	for i := range r.Seats {
		r.Seats[i].Ready = r.Seats[i].Bot
	}
	return nil
}

// Native Explorer and marked two-player combination recipes share the standard city deck.
func (r *Room) normalizeCatanCombinationKnights(setup game.CatanCitiesKnightsSetup) (game.CatanCitiesKnightsSetup, error) {
	n := r.Capacity
	if n == 2 && (publicCatanExplorerScenario(r.CatanScenario) || r.twoCatanSeafarers() || r.catanCaravanRecipe() || r.catanRiverRecipe() || r.CatanScenario == "transport" || (r.CatanScenario == "rivers-caravans" || (r.CatanScenario == "rivers-attack" || r.CatanScenario == "rivers-transport")) || r.catanAttackRecipe()) {
		n = 3
	}
	return game.NormalizeCatanCitiesKnightsSetup(n, setup)
}
