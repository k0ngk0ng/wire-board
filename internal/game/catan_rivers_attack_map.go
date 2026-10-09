package game

import (
	"errors"
	"slices"
)

const CatanRiversAttackRules = "catan-rivers-attack-2025"

// The small board transcribes the official 2025 combination diagram. The
// extended board is a labelled site recipe retaining all three printed rivers.
func catanRiversAttackRecipe(extended bool) (catanAttackRecipe, [][]int, [][]int, []int) {
	if extended {
		paths, terrain, outlets := catanRiverRecipe(true)
		return catanAttackRecipe{
			castles:          []int{12, 18},
			coast:            []int{7, 3, 0, 2, 6, 11, 17, 22, 26, 29, 27, 23},
			numbers:          []int{12, 0, 3, 4, 5, 10, 8, 5, 11, 6, 3, 5, 0, 8, 10, 4, 6, 9, 0, 3, 8, 11, 9, 6, 4, 9, 10, 2, 0, 11},
			coastalResources: [5]int{3, 3, 1, 2, 2}, innerResources: [5]int{1, 0, 2, 4, 0},
		}, paths, terrain, outlets
	}
	return catanAttackRecipe{
		castles:          []int{16},
		coast:            []int{12, 7, 3, 0, 1, 6, 11, 15, 17},
		numbers:          []int{3, 8, 0, 9, 4, 5, 10, 12, 6, 10, 8, 11, 4, 9, 3, 5, 0, 6, 0},
		coastalResources: [5]int{2, 1, 2, 2, 0}, innerResources: [5]int{1, 0, 0, 2, 1},
	}, [][]int{{13, 9, 5, 2}, {11, 15, 18}}, [][]int{{4, 1, 2, catanSwamp}, {4, 1, catanSwamp}}, []int{3, 0}
}

func riversAttackFixedTerrain(extended bool) map[int]int {
	_, paths, terrain, _ := catanRiversAttackRecipe(extended)
	fixed := map[int]int{}
	for i, path := range paths {
		for j, id := range path {
			fixed[id] = terrain[i][j]
		}
	}
	return fixed
}

func riversAttackChannels(g *Catan) (*catanRiversMap, error) {
	extended := len(g.Players) > 4
	_, paths, _, outlets := catanRiversAttackRecipe(extended)
	f := &catanRiversMap{DoubleNumberTile: 7}
	if extended {
		f.DoubleNumberTile = -1
	}
	for i, path := range paths {
		c := catanRiverChannel{Tiles: slices.Clone(path), Outlet: catanFishingSide(g, path[len(path)-1], outlets[i])}
		if c.Outlet < 0 || len(g.Edges[c.Outlet].Tiles) != 1 {
			return nil, errors.New("河流蛮族河口必须临海")
		}
		f.Channels = append(f.Channels, c)
		for j, id := range path {
			if g.Tiles[id].Resource == catanSwamp {
				f.Swamps = append(f.Swamps, id)
			}
			if j > 0 {
				edge := -1
				for _, e := range g.Edges {
					if slices.Contains(e.Tiles, id) && slices.Contains(e.Tiles, path[j-1]) {
						edge = e.ID
						break
					}
				}
				if edge < 0 {
					return nil, errors.New("河流蛮族河道不连续")
				}
				f.Bridges = append(f.Bridges, edge)
			}
		}
		f.Bridges = append(f.Bridges, c.Outlet)
	}
	return f, nil
}

// Board and components only; public sessions remain disabled until combined
// gold, combat, two-player rules and the complete action flow are integrated.
func newCatanRiversAttackBoard(n int) (*Catan, *catanAttackMap, *catanRiversMap, error) {
	g, m, err := newCatanAttackBoard(n)
	if err != nil {
		return nil, nil, nil, err
	}
	recipe, _, _, _ := catanRiversAttackRecipe(n > 4)
	fixed := riversAttackFixedTerrain(n > 4)
	resources := func(counts [5]int) []int {
		result := []int{}
		for color, count := range counts {
			for range count {
				result = append(result, color)
			}
		}
		shuffle(result)
		return result
	}
	coast, inner := resources(recipe.coastalResources), resources(recipe.innerResources)
	for id := range g.Tiles {
		t := &g.Tiles[id]
		t.Number = recipe.numbers[id]
		if terrain, ok := fixed[id]; ok {
			t.Resource = terrain
		} else if slices.Contains(recipe.castles, id) {
			t.Resource = catanCastle
		} else if slices.Contains(recipe.coast, id) {
			if len(coast) == 0 {
				return nil, nil, nil, errors.New("河流蛮族沿海库存不足")
			}
			t.Resource = coast[0]
			coast = coast[1:]
		} else {
			if len(inner) == 0 {
				return nil, nil, nil, errors.New("河流蛮族内陆库存不足")
			}
			t.Resource = inner[0]
			inner = inner[1:]
		}
	}
	if len(coast)+len(inner) != 0 {
		return nil, nil, nil, errors.New("河流蛮族地形库存多余")
	}
	m.Rivers = CatanRiversAttackRules
	m.Castles = recipe.castles
	m.Coast = recipe.coast
	f, err := riversAttackChannels(g)
	if err != nil {
		return nil, nil, nil, err
	}
	if err = m.validate(g); err != nil {
		return nil, nil, nil, err
	}
	return g, m, f, nil
}

// Landing matches printed numbers even when production is already conquered.
// The small combination's western coastal hex carries both 2 and 12.
func (m catanAttackMap) landingNumber(g *Catan, tile, number int) bool {
	if tile < 0 || tile >= len(g.Tiles) {
		return false
	}
	return g.Tiles[tile].Number == number || m.Rivers == CatanRiversAttackRules && len(g.Players) <= 4 && tile == 7 && number == 2
}
