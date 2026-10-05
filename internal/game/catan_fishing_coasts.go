package game

import (
	"errors"
	"slices"
)

// Build the six freely placed coastal grounds used by several scenario
// recipes. Scenario-specific lake/number changes are validated by the caller.
func makeFishingCoastalMap(coasts []CatanFishingCoast, placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	if placements == nil {
		numbers := []int{4, 5, 6, 8, 9, 10}
		used := map[int]bool{}
		var choose func(int) bool
		choose = func(next int) bool {
			if len(placements) == len(numbers) {
				return true
			}
			for i := next; i < len(coasts); i++ {
				c := coasts[i]
				if used[c.Edges[0]] || used[c.Edges[1]] {
					continue
				}
				used[c.Edges[0]], used[c.Edges[1]] = true, true
				placements = append(placements, CatanFishingGroundPlacement{numbers[len(placements)], c.Edges})
				if choose(i + 1) {
					return true
				}
				placements = placements[:len(placements)-1]
				delete(used, c.Edges[0])
				delete(used, c.Edges[1])
			}
			return false
		}
		if !choose(0) {
			return nil, errors.New("海岸没有六个不重叠且不占港口的渔场位置")
		}
	}
	byEdges := map[[2]int]CatanFishingCoast{}
	for _, c := range coasts {
		byEdges[c.Edges] = c
	}
	f := &catanFishingMap{Lakes: []catanFishingLake{}, Grounds: []catanFishingGround{}}
	for _, p := range placements {
		c, ok := byEdges[fishingEdgeKey(p.Edges)]
		if !ok {
			return nil, errors.New("渔场必须位于合法海岸凹角且不占港口")
		}
		f.Grounds = append(f.Grounds, fishingSeaGround(p.Number, c))
	}
	if err := f.validateCoastalGrounds(coasts); err != nil {
		return nil, err
	}
	return f, nil
}

func (f catanFishingMap) validateCoastalGrounds(coasts []CatanFishingCoast) error {
	byEdges := map[[2]int]CatanFishingCoast{}
	for _, c := range coasts {
		byEdges[c.Edges] = c
	}
	used, numbers := map[int]bool{}, []int{}
	for _, ground := range f.Grounds {
		c, ok := byEdges[ground.Edges]
		if !ok || ground.Vertices != c.Vertices || (ground.SeaTile == nil) != (c.SeaTile < 0) || ground.SeaTile != nil && *ground.SeaTile != c.SeaTile {
			return errors.New("渔场与海岸或封锁海格不匹配")
		}
		for _, edge := range ground.Edges {
			if used[edge] {
				return errors.New("渔场相互重叠")
			}
			used[edge] = true
		}
		numbers = append(numbers, ground.Number)
	}
	slices.Sort(numbers)
	if !slices.Equal(numbers, []int{4, 5, 6, 8, 9, 10}) {
		return errors.New("海岸渔场点数不符")
	}
	return nil
}
