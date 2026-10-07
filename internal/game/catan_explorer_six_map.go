package game

import "errors"

// Official 2025 E&P 5–6 rulebook pp4–7. Twenty-one loose land plus the
// printed 6-pasture form the starting island; loose sea counts exclude the
// printed opposite sea and, when used, the council island. Land Ho has no
// five/six-player recipe in this edition.
func catanExplorerSixRecipe(scenario string) (catanExplorerMapRecipe, error) {
	r := catanExplorerMapRecipe{
		starting: []int{2, 2, 2, 3, 4, 3, 2, 2, 2},
		numbers:  []int{2, 9, 11, 8, 4, 10, 10, 3, 8, 6, 5, 4, 5, 3, 10, 6, 8, 4, 11, 6, 12, 9},
		// Only a multiset: all loose terrain is shuffled. Index 9 is the
		// printed pasture and is excluded from that shuffle.
		resources: []int{0, 0, 0, 0, 0, 1, 1, 1, 2, 2, 2, 2, 2, 2, 3, 3, 3, 4, 4, 4, 4, 4},
	}
	switch scenario {
	case "pirate-lairs":
		r.width, r.target = 7, 12
		r.parrot = [][]int{{4, 6}, {4, 5, 6, 7}, {4, 5, 6, 7, 8}, {5, 7, 9}}
	case "fish-for-catan", "spices-for-catan":
		r.width, r.target = 8, 15
		r.parrot = [][]int{{4, 6, 7}, {4, 5, 6, 7, 8}, {4, 5, 6, 9}, {5, 7, 8, 10}}
	case "explorers-and-pirates":
		r.width, r.target = 9, 17
		r.parrot = [][]int{{4, 6, 8}, {4, 5, 6, 7, 8, 9}, {4, 5, 6, 7, 8, 9, 10}, {5, 7, 9, 11}}
	default:
		return r, errors.New("五六人探险地图仅支持四种正式任务剧本")
	}
	return r, nil
}

func (m catanExplorerBoard) regionNumbers(region int) []int {
	if m.Players <= 4 {
		return catanExplorerRegionNumbers(region)
	}
	if region == 0 {
		return []int{2, 3, 4, 5, 5, 6, 9, 9, 10}
	}
	return []int{3, 4, 4, 5, 8, 9, 10, 10, 11}
}

func (m catanExplorerBoard) regionResources(region int) []int {
	if m.Players <= 4 {
		return catanExplorerScenarioResources(region, m.Scenario)
	}
	resources := []int{0, 0, 1, 2, 3, 3, 4, 4, 4}
	if region == 1 {
		resources = []int{0, 1, 1, 2, 2, 3, 3, 4, 4}
	}
	switch m.Scenario {
	case "pirate-lairs":
		resources = append(resources, CatanSea, CatanGold, CatanGold, CatanGold, CatanGold)
	case "fish-for-catan":
		resources = append(resources, CatanSea, CatanSea, CatanSea, CatanGold, CatanGold, CatanGold, CatanGold)
	case "spices-for-catan", "explorers-and-pirates":
		resources = append(resources, CatanSea, CatanSea, CatanSea, CatanSea, CatanDesert, CatanDesert, CatanDesert)
		if m.Scenario == "explorers-and-pirates" {
			resources = append(resources, CatanGold, CatanGold, CatanGold, CatanGold)
		}
	}
	return resources
}
