package game

import (
	"errors"
	"slices"
)

// Only the initially faceup islands may carry grounds (2025 combination p.2).
// Reconstruct their coastline from the same printed recipe as setup, without
// generating a board or consulting the hidden stacks. Discovered sea must not
// create new legal ground positions; discovered land cannot move old ones.
func (g *Catan) fishingFogCoasts() ([]CatanFishingCoast, error) {
	if g.Seafarers == nil || g.Seafarers.Scenario != "fog" || g.Seafarers.Fog == nil || len(g.Players) < 3 || len(g.Players) > 4 {
		return nil, errors.New("仅已核对的三/四人迷雾群岛捕鱼位置")
	}
	rows := catanFogThreeRows()
	if len(g.Players) == 4 {
		rows = catanFogFourRows()
	}
	board := *g
	board.Tiles = slices.Clone(g.Tiles)
	initial, id := []int{}, 0
	for _, row := range rows {
		for _, tile := range row {
			if id >= len(board.Tiles) {
				return nil, errors.New("迷雾捕鱼地图大小不符")
			}
			board.Tiles[id].Resource = tile.resource
			if tile.resource != CatanSea && tile.resource != CatanFog {
				initial = append(initial, id)
			}
			id++
		}
	}
	if id != len(board.Tiles) || !slices.Equal(initial, g.Seafarers.Fog.StartTiles) {
		return nil, errors.New("迷雾捕鱼起始岛屿不符")
	}
	return board.fishingCoasts(board.findIslands()), nil
}

func (g *Catan) makeFishingFog(placements []CatanFishingGroundPlacement) (*catanFishingMap, error) {
	coasts, err := g.fishingFogCoasts()
	if err != nil {
		return nil, err
	}
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
			return nil, errors.New("起始岛屿没有六个不重叠且不占港口的渔场位置")
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
			return nil, errors.New("渔场必须位于起始岛屿合法海岸凹角且不占港口")
		}
		f.Grounds = append(f.Grounds, fishingSeaGround(p.Number, c))
	}
	if err := f.validateFog(g); err != nil {
		return nil, err
	}
	return f, nil
}

func (f catanFishingMap) validateFog(g *Catan) error {
	coasts, err := g.fishingFogCoasts()
	if err != nil {
		return err
	}
	if len(f.Lakes) != 0 || len(f.Grounds) != 6 {
		return errors.New("迷雾捕鱼不使用湖泊，必须有六个渔场")
	}
	byEdges := map[[2]int]CatanFishingCoast{}
	for _, c := range coasts {
		byEdges[c.Edges] = c
	}
	used, numbers := map[int]bool{}, []int{}
	for _, ground := range f.Grounds {
		c, ok := byEdges[ground.Edges]
		if !ok || ground.Vertices != c.Vertices || (ground.SeaTile == nil) != (c.SeaTile < 0) || ground.SeaTile != nil && *ground.SeaTile != c.SeaTile {
			return errors.New("渔场与起始海岸或封锁海格不匹配")
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
		return errors.New("迷雾渔场点数不符")
	}
	for _, tile := range g.Tiles {
		if tile.Resource == catanLake {
			return errors.New("迷雾捕鱼不能放置湖泊")
		}
	}
	if slices.Contains(g.Seafarers.Fog.Terrain, catanLake) {
		return errors.New("迷雾探索堆不能包含湖泊")
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && (g.Tiles[g.Robber].Resource == CatanSea || g.Tiles[g.Robber].Resource == CatanFog) {
		return errors.New("非法强盗位置")
	}
	return nil
}
