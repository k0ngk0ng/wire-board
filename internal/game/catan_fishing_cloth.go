package game

import (
	"errors"
	"slices"
)

// July 2025 Fishing + Seafarers p.2: no lake; three grounds on each
// large island, with freely chosen coastal positions and number allocation.
func (g *Catan) fishingClothCoasts() ([]CatanFishingCoast, []int, error) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "cloth" || g.cloth() == nil ||
		len(g.Players) < 3 || len(g.Players) > 4 || len(g.Tiles) != 42 || len(g.cloth().HomeTiles) != 20 || len(g.Seafarers.StartIslands) != 2 {
		return nil, nil, errors.New("仅已核对的三/四人布匹捕鱼布局")
	}
	islands := g.findIslands()
	if !slices.Equal(islands, g.Seafarers.Islands) {
		return nil, nil, errors.New("布匹捕鱼岛屿不符")
	}
	groups := slices.Clone(g.Seafarers.StartIslands)
	if groups[0] == groups[1] || groups[0] < 0 || groups[1] < 0 {
		return nil, nil, errors.New("布匹捕鱼必须有两座大岛")
	}
	counts, homes := map[int]int{}, []int{}
	for tile, island := range islands {
		if slices.Contains(groups, island) {
			counts[island]++
			homes = append(homes, tile)
		}
	}
	if counts[groups[0]] != 10 || counts[groups[1]] != 10 || !slices.Equal(homes, g.cloth().HomeTiles) {
		return nil, nil, errors.New("布匹捕鱼起始大岛不符")
	}
	coasts := g.fishingCoasts(islands)
	coasts = slices.DeleteFunc(coasts, func(c CatanFishingCoast) bool { return !slices.Contains(groups, c.Island) })
	return coasts, groups, nil
}

func (g *Catan) makeFishingCloth(placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	coasts, groups, err := g.fishingClothCoasts()
	if err != nil {
		return nil, err
	}
	if placements == nil {
		numbers, used, counts := []int{4, 5, 6, 8, 9, 10}, map[int]bool{}, map[int]int{}
		var choose func(int) bool
		choose = func(next int) bool {
			if len(placements) == 6 {
				return counts[groups[0]] == 3 && counts[groups[1]] == 3
			}
			for i := next; i < len(coasts); i++ {
				c := coasts[i]
				if used[c.Edges[0]] || used[c.Edges[1]] || counts[c.Island] >= 3 {
					continue
				}
				used[c.Edges[0]], used[c.Edges[1]] = true, true
				counts[c.Island]++
				placements = append(placements, CatanFishingGroundPlacement{numbers[len(placements)], c.Edges})
				if choose(i + 1) {
					return true
				}
				placements = placements[:len(placements)-1]
				counts[c.Island]--
				delete(used, c.Edges[0])
				delete(used, c.Edges[1])
			}
			return false
		}
		if !choose(0) {
			return nil, errors.New("两座大岛各需三个不重叠且不占港口的渔场位置")
		}
	}
	f, err := makeFishingCoastalMap(coasts, placements)
	if err != nil {
		return nil, err
	}
	if err = f.validateCloth(g); err != nil {
		return nil, err
	}
	return f, nil
}

func (f catanFishingMap) validateCloth(g *Catan) error {
	coasts, groups, err := g.fishingClothCoasts()
	if err != nil {
		return err
	}
	if len(f.Lakes) != 0 || len(f.Grounds) != 6 || len(f.ExtraNumbers) != 0 {
		return errors.New("布匹捕鱼不使用湖泊，必须有六个渔场")
	}
	if err = f.validateCoastalGrounds(coasts); err != nil {
		return err
	}
	counts := map[int]int{}
	for _, ground := range f.Grounds {
		for _, coast := range coasts {
			if coast.Edges == ground.Edges {
				counts[coast.Island]++
			}
		}
	}
	if counts[groups[0]] != 3 || counts[groups[1]] != 3 {
		return errors.New("布匹捕鱼必须在两座大岛各放三个渔场")
	}
	for _, tile := range g.Tiles {
		if tile.Resource == catanLake {
			return errors.New("布匹捕鱼不能放置湖泊")
		}
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && !g.clothLand(g.Robber) {
		return errors.New("布匹捕鱼强盗必须位于大岛或场外")
	}
	return nil
}
