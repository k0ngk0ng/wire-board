package game

import "errors"

// Site recipe: physical six/eight token faces could not be independently
// verified. Symmetric values avoid biasing gold mines toward low or high rolls.
const CatanExplorerLairRecipe = "wire-board-lair-numbers-v1"
const catanExplorerLairRecipeNotice = "本站巢穴数字配置：二至四人使用3、4、5、9、10、11，五六人再加入6、8；随机分配，攻陷前隐藏"

func catanExplorerLairNumbers(players int) []int {
	numbers := []int{3, 4, 5, 9, 10, 11}
	if players > 4 {
		numbers = append(numbers, 6, 8)
	}
	return numbers
}

// NewCatanExplorerMission starts one of the three missions with pirate lairs.
// Existing Land Ho and Spices recipes remain unchanged.
func NewCatanExplorerMission(players int, scenario string) (*State, error) {
	if !catanExplorerMissionScenario(scenario) {
		return nil, errors.New("请选择海盗巢穴、卡坦鱼群或完整三任务")
	}
	s, err := newCatanExplorerMissionState(players, scenario, "variable", catanExplorerLairNumbers(players))
	if err != nil {
		return nil, err
	}
	s.Catan.Explorer.Lairs.NumberRecipe = CatanExplorerLairRecipe
	s.Log = append(s.Log, catanExplorerLairRecipeNotice)
	return s, s.validateCatanExplorer()
}
