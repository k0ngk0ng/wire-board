package game

import (
	"errors"
	"slices"
)

// The combination sheet allows Shores but does not locate a lake in the
// desert-free three-player map. Keep this adaptation explicit in saves/UI.
const CatanFishingShoresRecipe = "wire-board-fishing-shores-v1"

func (g *Catan) fishingLakeInterior(id int) bool {
	if id < 0 || id >= len(g.Tiles) {
		return false
	}
	neighbors := map[int]bool{}
	for _, edge := range g.Edges {
		if !slices.Contains(edge.Tiles, id) {
			continue
		}
		if len(edge.Tiles) != 2 {
			return false
		}
		for _, other := range edge.Tiles {
			if other == id {
				continue
			}
			if other < 0 || other >= len(g.Tiles) || g.Tiles[other].Resource == CatanSea || g.Tiles[other].Resource == CatanFog {
				return false
			}
			neighbors[other] = true
		}
	}
	return len(neighbors) == 6
}

func newCatanFishingShores(n int, options CatanOptions, setup CatanSeafarersSetup, placements []CatanFishingGroundPlacement) (*State, error) {
	// Condition variable generation on inland deserts; never relocate numbered
	// terrain after generation, which would break the extended spiral.
	for attempt := 0; attempt < 1024; attempt++ {
		s, err := NewCatanSeafarers(n, options, setup, nil)
		if err != nil {
			return nil, err
		}
		g := s.Catan
		lakes := []int{}
		if n == 3 {
			lakes = append(lakes, 16)
		} else {
			for _, tile := range g.Tiles {
				if tile.Resource == CatanDesert {
					lakes = append(lakes, tile.ID)
				}
			}
		}
		inland := true
		for _, id := range lakes {
			if !g.fishingLakeInterior(id) {
				inland = false
			}
		}
		if !inland {
			continue
		}
		f, err := makeFishingCoastalMap(g.fishingCoasts(g.findIslands()), placements, n)
		if err != nil {
			return nil, err
		}
		f.SeaRecipe = CatanFishingShoresRecipe
		for i, id := range lakes {
			numbers := []int{2, 3, 11, 12}
			if i == 1 {
				numbers = []int{4, 10}
			}
			f.Lakes = append(f.Lakes, catanFishingLake{Tile: id, Numbers: numbers})
			g.Tiles[id].Resource, g.Tiles[id].Number = catanLake, 0
		}
		// Apply Fishing setup: the robber waits outside, the sea pirate stays at
		// its existing scenario starting position.
		g.Robber = -1
		for i, line := range s.Log {
			if line == catanSeaNumberNotice {
				s.Log[i] = "本站数字配置：五六人新海岸主岛使用原28枚数列，跳过湖泊；外围岛屿保持不变"
			}
		}
		if err = f.validateShores(g); err != nil {
			return nil, err
		}
		tokens, err := newCatanFishingTokens(n)
		if err != nil {
			return nil, err
		}
		g.Fishing = &CatanFishing{Map: *f, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
		s.Log = append(s.Log, "本站新海岸捕鱼配方：三人以主岛内陆地块替换湖泊并移除原数字；四至六人以内陆沙漠替换湖泊。强盗从场外开始，原探索奖励与14分目标保留；旧靴持有者需多1分")
		return s, nil
	}
	return nil, errors.New("无法生成内陆湖泊的新海岸地图，请重新创建")
}

func (f catanFishingMap) validateShores(g *Catan) error {
	n := len(g.Players)
	if g.Seafarers == nil || g.Seafarers.Scenario != "shores" || n < 3 || n > 6 || len(g.Seafarers.StartIslands) != 1 || f.SeaRecipe != CatanFishingShoresRecipe || f.NumberRecipe != "" || len(f.ExtraNumbers) != 0 {
		return errors.New("新海岸捕鱼配置或版本无效")
	}
	wantTiles, wantMain, wantLakes := 35, 14, 1
	if n == 4 {
		wantTiles, wantMain = 42, 19
	}
	if n > 4 {
		wantTiles, wantMain, wantLakes = 56, 30, 2
	}
	if len(g.Tiles) != wantTiles || len(g.Seafarers.Islands) != wantTiles || len(f.Lakes) != wantLakes {
		return errors.New("新海岸捕鱼组件数量不符")
	}
	physical := g.findIslands()
	home, mainSize := g.Seafarers.StartIslands[0], 0
	for id, island := range physical {
		if island == home {
			mainSize++
		}
		if (island == home) != (g.Seafarers.Islands[id] == home) {
			return errors.New("新海岸捕鱼起始主岛不符")
		}
	}
	if home < 0 || mainSize != wantMain {
		return errors.New("新海岸捕鱼主岛大小不符")
	}
	seen := map[int]bool{}
	for i, lake := range f.Lakes {
		numbers := []int{2, 3, 11, 12}
		if i == 1 {
			numbers = []int{4, 10}
		}
		if lake.Tile < 0 || lake.Tile >= len(g.Tiles) || seen[lake.Tile] || !slices.Equal(lake.Numbers, numbers) || physical[lake.Tile] != home || !g.fishingLakeInterior(lake.Tile) || g.Tiles[lake.Tile].Resource != catanLake || g.Tiles[lake.Tile].Number != 0 {
			return errors.New("新海岸湖泊须位于主岛内陆，使用对应生产点数")
		}
		if n == 3 && lake.Tile != 16 || n == 4 && !g.Seafarers.Variable && lake.Tile != 26 {
			return errors.New("新海岸固定湖泊位置不符")
		}
		seen[lake.Tile] = true
	}
	for _, tile := range g.Tiles {
		if tile.Resource == CatanDesert || tile.Resource == catanLake && !seen[tile.ID] {
			return errors.New("新海岸捕鱼不能保留沙漠或额外湖泊")
		}
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && (g.Tiles[g.Robber].Resource == CatanSea || g.Tiles[g.Robber].Resource == CatanFog) {
		return errors.New("非法强盗位置")
	}
	return f.validateCoastalGrounds(g.fishingCoasts(physical), n)
}
