package game

import "fmt"

// Internal constructor until scenario controls, art and full UI acceptance
// are complete. Three/four and five/six each share an official fixed layout.
func NewCatanWonders(n int, options CatanOptions) (*State, error) {
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	if n > 4 {
		err = s.Catan.makeSeafarersWondersSix()
	} else {
		err = s.Catan.makeSeafarersWondersFour()
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Seafarers (2025), pages 18–19. The two small islands are fixed, as are
// the three deserts on the eastern side of the main island.
func (g *Catan) makeSeafarersWondersFour() error {
	if len(g.Players) < 3 || len(g.Players) > 4 {
		return fmt.Errorf("此卡坦奇迹地图需要3至4位玩家")
	}
	rows := [][]seaTerrain{
		{{7, 8}, {1, 2}, {6, 0}, {4, 10}, {6, 0}, {6, 0}},
		{{6, 0}, {6, 0}, {6, 0}, {0, 11}, {6, 0}, {5, 0}, {6, 0}},
		{{4, 12}, {3, 6}, {1, 11}, {3, 10}, {2, 3}, {0, 9}, {5, 0}, {6, 0}},
		{{4, 3}, {3, 4}, {4, 6}, {2, 5}, {1, 4}, {5, 0}, {6, 0}},
		{{1, 8}, {2, 9}, {6, 0}, {6, 0}, {1, 10}, {6, 0}, {6, 0}, {7, 6}},
		{{6, 0}, {0, 3}, {6, 0}, {0, 8}, {3, 9}, {6, 0}, {0, 4}},
		{{4, 5}, {6, 0}, {2, 11}, {2, 2}, {6, 0}, {3, 5}},
	}
	ports := []seaPort{{2, 1, 3, 0}, {2, 3, 3, 1}, {2, 4, 4, 2}, {3, 0, 2, 3}, {3, 1, 0, 4}, {5, 1, 2, -1}, {5, 4, 4, -1}, {5, 4, 0, -1}, {6, 2, 0, -1}}
	// Each marker is wonder ID, row, column, corner.
	markers := [][4]int{
		{3, 5, 1, 0}, {3, 6, 2, 3},
		{4, 2, 5, 4}, {4, 2, 5, 5}, {4, 2, 5, 0}, {4, 2, 5, 1}, {4, 3, 4, 0},
	}
	xs := [][3]int{{5, 1, 5}, {5, 1, 1}, {6, 2, 4}, {6, 2, 2}}
	return g.makeSeafarersWondersFixed(rows, ports, markers, xs, [2]int{2, 6})
}

// Seafarers 5–6 (2025), page 11. The northern pair of markers are
// lighthouses (PDF vector artwork overlays the older raster bridge icons).
func (g *Catan) makeSeafarersWondersSix() error {
	if len(g.Players) < 5 || len(g.Players) > 6 {
		return fmt.Errorf("此卡坦奇迹地图需要5至6位玩家")
	}
	rows := [][]seaTerrain{
		{{7, 8}, {6, 0}, {2, 10}, {6, 0}, {3, 12}, {6, 0}, {6, 0}, {6, 0}},
		{{6, 0}, {6, 0}, {1, 11}, {6, 0}, {0, 11}, {6, 0}, {6, 0}, {5, 0}, {5, 0}},
		{{6, 0}, {1, 2}, {2, 5}, {3, 6}, {1, 5}, {3, 3}, {2, 8}, {0, 11}, {5, 0}, {6, 0}},
		{{6, 0}, {3, 9}, {4, 3}, {0, 4}, {4, 6}, {3, 10}, {2, 9}, {1, 10}, {5, 0}},
		{{6, 0}, {6, 0}, {1, 8}, {2, 5}, {6, 0}, {1, 4}, {0, 8}, {4, 12}, {6, 0}, {6, 0}},
		{{7, 6}, {6, 0}, {0, 10}, {4, 3}, {6, 0}, {4, 9}, {3, 4}, {6, 0}, {0, 4}},
		{{6, 0}, {4, 5}, {0, 9}, {6, 0}, {2, 2}, {2, 11}, {6, 0}, {7, 6}},
	}
	ports := []seaPort{{2, 1, 2, 0}, {2, 2, 3, 1}, {2, 3, 4, 2}, {2, 5, 4, 2}, {3, 3, 0, 3}, {4, 2, 2, 4}, {5, 2, 2, -1}, {4, 7, 0, -1}, {6, 1, 0, -1}, {6, 4, 0, -1}, {5, 6, 0, -1}}
	markers := [][4]int{
		{3, 5, 3, 0}, {3, 6, 4, 3},
		{4, 2, 7, 4}, {4, 2, 7, 5}, {4, 2, 7, 0}, {4, 3, 7, 5}, {4, 3, 7, 0},
		{5, 0, 2, 0}, {5, 1, 4, 3},
	}
	xs := [][3]int{{5, 3, 5}, {5, 3, 1}, {6, 4, 4}, {6, 4, 2}, {0, 2, 5}, {0, 2, 1}, {1, 4, 4}, {1, 4, 2}}
	return g.makeSeafarersWondersFixed(rows, ports, markers, xs, [2]int{2, 8})
}

func (g *Catan) makeSeafarersWondersFixed(rows [][]seaTerrain, ports []seaPort, markers [][4]int, xs [][3]int, robber [2]int) error {
	if err := g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "wonders", 10, robber, [][2]int{robber}); err != nil {
		return err
	}
	g.Seafarers.IslandBonus = 1
	g.shuffleSeafarersPorts()
	w := &CatanWonders{Cards: []CatanWonder{}, Markers: []CatanWonderMarker{}, SetupBlocked: []int{}}
	g.Seafarers.Wonders = w
	count := 5
	if len(g.Players) > 4 {
		count = 7
	}
	for id := range count {
		w.Cards = append(w.Cards, CatanWonder{ID: id, Owner: -1})
	}
	vertex := func(row, col, corner int) int {
		for r := 0; r < row; r++ {
			col += len(rows[r])
		}
		return g.Tiles[col].Vertices[corner]
	}
	for _, m := range markers {
		v := vertex(m[1], m[2], m[3])
		w.Markers = append(w.Markers, CatanWonderMarker{Card: m[0], Vertex: v})
		w.SetupBlocked = append(w.SetupBlocked, v)
	}
	for _, x := range xs {
		w.SetupBlocked = append(w.SetupBlocked, vertex(x[0], x[1], x[2]))
	}
	return nil
}

// The three/four-player variable recipe shuffles only the blue-outlined
// productive mainland. Deserts, islets, sea and all marker positions stay put.
// Only the two mainland hexes touching the desert are forbidden red tokens;
// the scenario does not prohibit adjacent reds elsewhere.
func (g *Catan) randomizeWondersMap() error {
	if g.wonders() == nil || len(g.Players) > 4 || len(g.Tiles) != 49 || len(g.Seafarers.StartIslands) != 1 {
		return fmt.Errorf("该卡坦奇迹地图尚未支持可变布局")
	}
	mainland, terrain, numbers := []int{}, []int{}, []int{}
	for _, tile := range g.Tiles {
		if tile.Resource < CatanDesert && g.Seafarers.Islands[tile.ID] == g.Seafarers.StartIslands[0] {
			mainland = append(mainland, tile.ID)
			terrain = append(terrain, tile.Resource)
			numbers = append(numbers, tile.Number)
		}
	}
	if len(mainland) != 22 {
		return fmt.Errorf("卡坦奇迹主岛地形数量不符")
	}
	border := map[int]bool{}
	for _, edge := range g.Edges {
		ids := g.edgeTiles(edge.ID)
		if len(ids) == 2 {
			for _, pair := range [][2]int{{ids[0], ids[1]}, {ids[1], ids[0]}} {
				if g.Tiles[pair[0]].Resource < CatanDesert && g.Tiles[pair[1]].Resource == CatanDesert {
					border[pair[0]] = true
				}
			}
		}
	}
	if len(border) != 2 {
		return fmt.Errorf("卡坦奇迹沙漠边界不符")
	}
	shuffle(terrain)
	shuffle(numbers)
	nonred, red := []int{}, []int{}
	for _, n := range numbers {
		if n == 6 || n == 8 {
			red = append(red, n)
		} else {
			nonred = append(nonred, n)
		}
	}
	if len(nonred) < len(border) {
		return fmt.Errorf("卡坦奇迹沙漠边界缺少非红色数字")
	}
	tiles := append([]CatanTile{}, g.Tiles...)
	for i, id := range mainland {
		tiles[id].Resource, tiles[id].Number = terrain[i], 0
		if border[id] {
			tiles[id].Number, nonred = nonred[0], nonred[1:]
		}
	}
	remaining := append(nonred, red...)
	shuffle(remaining)
	for _, id := range mainland {
		if tiles[id].Number == 0 {
			tiles[id].Number, remaining = remaining[0], remaining[1:]
		}
	}
	g.Tiles = tiles
	g.shuffleSeafarersPorts()
	g.Seafarers.Variable = true
	return nil
}
