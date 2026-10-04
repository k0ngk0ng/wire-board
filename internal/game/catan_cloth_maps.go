package game

import "fmt"

// Cloth for CATAN, 2025 Seafarers pages 14–15. Three/four players share
// this map; the numbers on the four islets belong to vertex villages.
func (g *Catan) makeSeafarersClothFour() error {
	if len(g.Players) < 3 || len(g.Players) > 4 {
		return fmt.Errorf("此布匹地图需要3至4位玩家")
	}
	rows := [][]seaTerrain{
		{{0, 4}, {2, 6}, {1, 5}, {2, 11}, {3, 8}},
		{{3, 3}, {0, 12}, {6, 0}, {6, 0}, {0, 3}, {4, 9}},
		{{3, 12}, {6, 0}, {6, 0}, {7, 0}, {6, 0}, {6, 0}, {6, 0}},
		{{6, 0}, {5, 0}, {6, 0}, {6, 0}, {5, 0}, {6, 0}},
		{{6, 0}, {6, 0}, {6, 0}, {7, 0}, {6, 0}, {6, 0}, {4, 2}},
		{{1, 9}, {3, 2}, {6, 0}, {6, 0}, {2, 11}, {4, 4}},
		{{2, 10}, {0, 6}, {4, 5}, {3, 10}, {1, 8}},
	}
	ports := []seaPort{{0, 0, 4, 0}, {0, 3, 3, 1}, {1, 0, 2, 2}, {1, 5, 5, 3}, {4, 6, 5, 4}, {5, 0, 2, -1}, {5, 5, 0, -1}, {6, 1, 1, -1}, {6, 3, 0, -1}}
	// row, column, north village number, south village number.
	villages := [][4]int{{2, 3, 11, 8}, {3, 1, 10, 9}, {3, 4, 4, 5}, {4, 3, 6, 3}}
	return g.makeSeafarersClothFixed(rows, ports, villages, [2]int{2, 0})
}

// 2025 Seafarers 5–6 extension page 9: twelve villages, seventy cloth,
// eleven ports (the additional specialized port is wool).
func (g *Catan) makeSeafarersClothSix() error {
	if len(g.Players) < 5 || len(g.Players) > 6 {
		return fmt.Errorf("此布匹地图需要5至6位玩家")
	}
	rows := [][]seaTerrain{
		{{0, 4}, {2, 6}, {1, 5}, {3, 12}, {1, 3}, {2, 11}, {3, 8}},
		{{3, 3}, {0, 12}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {0, 3}, {4, 9}},
		{{3, 11}, {6, 0}, {6, 0}, {5, 0}, {6, 0}, {5, 0}, {6, 0}, {6, 0}, {2, 3}},
		{{6, 0}, {7, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {7, 0}, {6, 0}},
		{{4, 8}, {6, 0}, {6, 0}, {5, 0}, {6, 0}, {5, 0}, {6, 0}, {6, 0}, {4, 2}},
		{{1, 9}, {3, 2}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {2, 11}, {4, 4}},
		{{2, 10}, {0, 6}, {4, 5}, {0, 11}, {0, 8}, {3, 10}, {1, 6}},
	}
	ports := []seaPort{{0, 0, 4, 0}, {0, 3, 3, 1}, {0, 5, 3, 2}, {1, 0, 2, 2}, {1, 7, 5, 3}, {4, 8, 5, 4}, {5, 0, 2, -1}, {5, 7, 0, -1}, {6, 1, 1, -1}, {6, 2, 0, -1}, {6, 5, 0, -1}}
	villages := [][4]int{{2, 3, 2, 5}, {2, 5, 10, 8}, {3, 1, 4, 9}, {3, 6, 4, 5}, {4, 3, 6, 12}, {4, 5, 9, 10}}
	return g.makeSeafarersClothFixed(rows, ports, villages, [2]int{2, 0})
}

func (g *Catan) makeSeafarersClothFixed(rows [][]seaTerrain, ports []seaPort, villages [][4]int, robber [2]int) error {
	if err := g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "cloth", 14, robber, [][2]int{{0, 0}, {6, 0}}); err != nil {
		return err
	}
	g.shuffleSeafarersPorts()
	g.Seafarers.IslandBonus = 0
	// The extension changes setup/components only; it does not replace the
	// base rulebook's five-empty-village ending with a majority formula.
	c := &CatanClothState{Stock: 10, Held: make([]int, len(g.Players)), EmptyLimit: 5}
	g.Seafarers.Cloth = c
	tileID := func(row, col int) int {
		for r := 0; r < row; r++ {
			col += len(rows[r])
		}
		return col
	}
	// The illustrated pirate sits in the right-hand sea frame indent,
	// not on the adjacent playable hex. makeSeafarersFixed initializes -1.
	for _, tile := range g.Tiles {
		if tile.Number > 0 {
			c.HomeTiles = append(c.HomeTiles, tile.ID)
		}
	}
	for _, v := range villages {
		tile := g.Tiles[tileID(v[0], v[1])]
		c.Villages = append(c.Villages, CatanClothVillage{Vertex: tile.Vertices[4], Number: v[2], Stock: 5, Traders: []int{}}, CatanClothVillage{Vertex: tile.Vertices[1], Number: v[3], Stock: 5, Traders: []int{}})
	}
	return nil
}

func (g *Catan) randomizeClothMap() error {
	c := g.cloth()
	if c == nil || len(g.Players) > 4 || len(g.Tiles) != 42 || len(c.HomeTiles) != 20 {
		return fmt.Errorf("该布匹剧本尚未支持可变布局")
	}
	terrain, numbers := []int{}, []int{}
	for _, id := range c.HomeTiles {
		terrain = append(terrain, g.Tiles[id].Resource)
		numbers = append(numbers, g.Tiles[id].Number)
	}
	shuffle(terrain)
	shuffle(numbers)
	tiles := append([]CatanTile{}, g.Tiles...)
	candidates := []int{}
	for i, id := range c.HomeTiles {
		tiles[id].Resource, tiles[id].Number = terrain[i], numbers[i]
		if numbers[i] == 12 {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return fmt.Errorf("布匹地图缺少起始强盗所需的12号地块")
	}
	g.Tiles = tiles
	g.Robber = candidates[catanRandom(len(candidates))]
	g.shuffleSeafarersPorts()
	g.Seafarers.Variable = true
	return nil
}
