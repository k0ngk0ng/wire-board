package game

import "fmt"

// The Forgotten Tribe, 2025 Seafarers pages 12–13. Three and four players
// share this map. Outer islands intentionally have no number discs.
func (g *Catan) makeSeafarersTribeFour() error {
	if (len(g.Players) < 3 && !g.twoSeaRecipe()) || len(g.Players) > 4 {
		return fmt.Errorf("此部族地图需要3至4位玩家")
	}
	rows := [][]seaTerrain{
		{{7, 0}, {4, 0}, {6, 0}, {5, 0}, {4, 0}, {3, 0}},
		{{6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}},
		{{3, 6}, {0, 9}, {4, 11}, {1, 5}, {0, 6}, {4, 4}, {6, 0}, {2, 0}},
		{{2, 10}, {1, 8}, {2, 4}, {3, 12}, {0, 5}, {2, 2}, {6, 0}},
		{{0, 11}, {3, 9}, {4, 3}, {2, 8}, {1, 10}, {3, 3}, {6, 0}, {0, 0}},
		{{6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}},
		{{5, 0}, {1, 0}, {6, 0}, {1, 0}, {5, 0}, {7, 0}},
	}
	ports := []seaPort{{0, 0, 3, -1}, {0, 3, 3, 0}, {2, 7, 4, 1}, {4, 7, 0, 2}, {6, 0, 1, 3}, {6, 3, 0, 4}}
	tokens := [][3]int{{0, 0, 4}, {0, 3, 4}, {0, 5, 4}, {2, 7, 5}, {4, 7, 5}, {6, 0, 0}, {6, 3, 1}, {6, 5, 0}}
	cards := [][3]int{{0, 0, 2}, {0, 5, 5}, {6, 0, 2}, {6, 5, 5}}
	return g.makeSeafarersTribeFixed(rows, ports, tokens, cards, [2]int{6, 4}, [2]int{0, 2})
}

// Five/six-player extension, 2025 rulebook page 8. Ports are randomized,
// including the three generic ports shown in the component inventory.
func (g *Catan) makeSeafarersTribeSix() error {
	if len(g.Players) < 5 || len(g.Players) > 6 {
		return fmt.Errorf("此部族地图需要5至6位玩家")
	}
	rows := [][]seaTerrain{
		{{5, 0}, {5, 0}, {6, 0}, {7, 0}, {7, 0}, {6, 0}, {2, 0}, {2, 0}},
		{{6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}},
		{{3, 3}, {1, 5}, {4, 11}, {2, 8}, {1, 4}, {0, 10}, {3, 4}, {1, 3}, {0, 8}, {4, 5}},
		{{0, 11}, {2, 12}, {0, 5}, {3, 10}, {2, 5}, {1, 9}, {3, 2}, {2, 9}, {1, 10}},
		{{0, 6}, {4, 4}, {3, 9}, {1, 8}, {0, 11}, {4, 3}, {2, 4}, {3, 6}, {0, 3}, {4, 6}},
		{{6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}, {6, 0}},
		{{5, 0}, {5, 0}, {6, 0}, {4, 0}, {1, 0}, {6, 0}, {3, 0}, {7, 0}},
	}
	ports := []seaPort{{0, 1, 4, 3}, {0, 3, 3, -1}, {0, 6, 4, 0}, {0, 7, 4, -1}, {6, 0, 1, 1}, {6, 1, 0, 2}, {6, 4, 0, 4}, {6, 6, 1, -1}}
	tokens := [][3]int{{0, 0, 3}, {0, 1, 3}, {0, 3, 4}, {0, 4, 3}, {0, 6, 3}, {6, 1, 1}, {6, 3, 0}, {6, 4, 1}, {6, 6, 0}, {6, 7, 0}}
	cards := [][3]int{{0, 0, 2}, {0, 4, 4}, {0, 7, 5}, {6, 0, 2}, {6, 3, 1}, {6, 7, 5}}
	return g.makeSeafarersTribeFixed(rows, ports, tokens, cards, [2]int{0, 0}, [2]int{6, 2})
}

func (g *Catan) makeSeafarersTribeFixed(rows [][]seaTerrain, ports []seaPort, tokens, cards [][3]int, robber, pirate [2]int) error {
	if len(g.DevDeck) < len(cards) {
		return fmt.Errorf("部族发展卡数量不足")
	}
	if err := g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "tribe", 13, robber, [][2]int{{3, 2}}); err != nil {
		return err
	}
	g.shuffleSeafarersPorts()
	t := &CatanTribeState{Ports: g.Ports, Points: make([]int, len(g.Players)), HeldPorts: make([][]int, len(g.Players))}
	g.Ports = []CatanPort{}
	g.Seafarers.Tribe = t
	g.Seafarers.IslandBonus = 0
	tileID := func(row, col int) int {
		for r := 0; r < row; r++ {
			col += len(rows[r])
		}
		return col
	}
	g.Seafarers.Pirate = tileID(pirate[0], pirate[1])
	edgeID := func(coord [3]int) (int, error) {
		if coord[0] < 0 || coord[0] >= len(rows) || coord[1] < 0 || coord[1] >= len(rows[coord[0]]) || coord[2] < 0 || coord[2] > 5 {
			return -1, fmt.Errorf("部族奖励坐标不合法")
		}
		v := g.Tiles[tileID(coord[0], coord[1])].Vertices
		a, b := v[coord[2]], v[(coord[2]+1)%6]
		for _, e := range g.Edges {
			if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
				if g.edgeTerrain(e.ID, true) {
					return e.ID, nil
				}
			}
		}
		return -1, fmt.Errorf("部族奖励必须位于可航行的边")
	}
	for _, coord := range tokens {
		id, err := edgeID(coord)
		if err != nil {
			return err
		}
		t.Tokens = append(t.Tokens, id)
	}
	for _, coord := range cards {
		id, err := edgeID(coord)
		if err != nil {
			return err
		}
		last := len(g.DevDeck) - 1
		t.Development = append(t.Development, CatanTribeDevelopment{Edge: id, Card: g.DevDeck[last]})
		g.DevDeck = g.DevDeck[:last]
	}
	return nil
}

func (g *Catan) randomizeTribeMap() error {
	if len(g.Players) > 4 || len(g.Tiles) != 49 {
		return fmt.Errorf("该部族剧本尚未支持可变布局")
	}
	// The three blue-bordered east coast hexes must not receive 5/6/8/9.
	restricted := map[int]bool{18: true, 26: true, 33: true}
	land, terrain, low, other := []int{}, []int{}, []int{}, []int{}
	for _, tile := range g.Tiles {
		if g.Seafarers.Islands[tile.ID] != g.Seafarers.StartIslands[0] {
			continue
		}
		land = append(land, tile.ID)
		terrain = append(terrain, tile.Resource)
		if tile.Number == 5 || tile.Number == 6 || tile.Number == 8 || tile.Number == 9 {
			other = append(other, tile.Number)
		} else {
			low = append(low, tile.Number)
		}
	}
	if len(land) != 18 || len(low) < len(restricted) {
		return fmt.Errorf("部族主岛组件不完整")
	}
	shuffle(terrain)
	shuffle(low)
	tiles := append([]CatanTile(nil), g.Tiles...)
	for i, id := range land {
		tiles[id].Resource = terrain[i]
		if restricted[id] {
			tiles[id].Number = low[0]
			low = low[1:]
		}
	}
	numbers := append(other, low...)
	shuffle(numbers)
	for _, id := range land {
		if !restricted[id] {
			tiles[id].Number = numbers[0]
			numbers = numbers[1:]
		}
	}
	ports := g.tribe().Ports
	resources := make([]int, len(ports))
	for i, p := range ports {
		resources[i] = p.Resource
	}
	shuffle(resources)
	for i := range ports {
		ports[i].Resource = resources[i]
	}
	g.Tiles = tiles
	g.Seafarers.Variable = true
	return nil
}
