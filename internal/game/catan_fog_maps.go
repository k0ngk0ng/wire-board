package game

import "errors"

// 2025 Seafarers, pages 8–9. The white hexes are locations to reveal, not
// preassigned hidden tiles: their contents come from the separate shuffled pile.
func catanFogThreeRows() [][]seaTerrain {
	return [][]seaTerrain{
		{{CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {1, 6}, {0, 11}},
		{{CatanFog, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {0, 5}, {3, 3}},
		{{CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanFog, 0}, {CatanSea, 0}, {2, 8}, {2, 9}},
		{{0, 6}, {2, 5}, {CatanSea, 0}, {CatanFog, 0}, {CatanSea, 0}, {4, 4}},
		{{CatanSea, 0}, {1, 11}, {0, 9}, {CatanSea, 0}, {CatanFog, 0}, {CatanSea, 0}, {CatanSea, 0}},
		{{CatanSea, 0}, {4, 8}, {3, 10}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}},
		{{CatanSea, 0}, {2, 12}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}},
	}
}

func (g *Catan) makeSeafarersFogThree() error {
	rows := catanFogThreeRows()
	ports := []seaPort{{0, 3, 4, -1}, {0, 4, 5, 2}, {2, 6, 4, 3}, {3, 5, 5, -1}, {3, 0, 1, 4}, {4, 1, 1, 0}, {5, 2, 0, 1}, {6, 1, 2, -1}}
	return g.makeFogMap(rows, ports, [2]int{6, 1}, []int{0, 1, 1, 2, 3, 3, 4, 4, 6, 6, 7, 7}, []int{3, 3, 4, 5, 6, 8, 9, 10, 11, 12})
}

func catanFogFourRows() [][]seaTerrain {
	return [][]seaTerrain{
		{{CatanFog, 0}, {CatanSea, 0}, {1, 4}, {3, 10}, {4, 3}},
		{{CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {2, 9}, {0, 6}, {1, 12}},
		{{CatanSea, 0}, {CatanSea, 0}, {CatanFog, 0}, {CatanSea, 0}, {CatanSea, 0}, {2, 10}, {4, 8}},
		{{4, 3}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {3, 11}},
		{{3, 6}, {0, 4}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {0, 5}},
		{{1, 9}, {2, 8}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}},
		{{2, 2}, {0, 5}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}},
	}
}

func (g *Catan) makeSeafarersFogFour() error {
	rows := catanFogFourRows()
	ports := []seaPort{{0, 2, 4, 2}, {0, 3, 4, 3}, {0, 4, 5, -1}, {2, 6, 4, 1}, {4, 6, 4, -1}, {3, 0, 2, 0}, {4, 0, 1, 4}, {6, 0, 2, -1}, {6, 0, 0, -1}}
	return g.makeFogMap(rows, ports, [2]int{1, 5}, []int{0, 1, 1, 2, 3, 3, 4, 4, 6, 6, 7, 7}, []int{3, 4, 5, 6, 8, 9, 10, 11, 11, 12})
}

// 2025 Seafarers 5–6, page 6. This recipe includes a desert in the hidden
// pile; revealing it must not consume one of the fourteen number discs.
func (g *Catan) makeSeafarersFogSix() error {
	if len(g.Players) < 5 || len(g.Players) > 6 {
		return errors.New("此迷雾岛地图需要5至6位玩家")
	}
	rows := [][]seaTerrain{
		{{0, 10}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {0, 9}, {3, 2}},
		{{2, 6}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {3, 5}, {4, 12}},
		{{3, 12}, {1, 4}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {2, 4}, {1, 8}},
		{{4, 11}, {2, 5}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {0, 6}, {2, 3}},
		{{3, 6}, {0, 3}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {4, 9}, {3, 4}},
		{{1, 9}, {2, 8}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {1, 10}},
		{{1, 11}, {4, 10}, {CatanSea, 0}, {CatanFog, 0}, {CatanFog, 0}, {CatanSea, 0}, {0, 8}},
	}
	ports := []seaPort{{1, 0, 3, -1}, {0, 5, 4, 0}, {2, 8, 4, -1}, {1, 6, 1, 2}, {2, 1, 5, 2}, {3, 0, 2, 4}, {4, 8, 4, 1}, {4, 8, 0, 3}, {5, 0, 2, -1}, {5, 7, 2, -1}, {6, 1, 4, -1}}
	return g.makeFogMap(rows, ports, [2]int{2, 0}, []int{0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4, 5, 6, 6, 6, 7, 7, 7}, []int{2, 2, 3, 3, 4, 5, 5, 6, 8, 9, 10, 11, 11, 12})
}

func (g *Catan) makeFogMap(rows [][]seaTerrain, ports []seaPort, robber [2]int, terrain, numbers []int) error {
	if err := g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "fog", 12, robber, nil); err != nil {
		return err
	}
	fog := &CatanFogState{Terrain: append([]int{}, terrain...), Numbers: append([]int{}, numbers...), StartTiles: []int{}}
	missing, producing := 0, 0
	for _, t := range g.Tiles {
		if t.Resource == CatanFog {
			missing++
		} else if t.Resource != CatanSea {
			fog.StartTiles = append(fog.StartTiles, t.ID)
		}
	}
	for _, resource := range terrain {
		if resource != CatanSea && resource != CatanDesert {
			producing++
		}
	}
	if missing != len(terrain) || producing != len(numbers) {
		return errors.New("迷雾地图与探索堆数量不符")
	}
	shuffle(fog.Terrain)
	shuffle(fog.Numbers)
	g.Seafarers.Fog = fog
	g.Seafarers.IslandBonus = 0
	return nil
}

func (g *Catan) randomizeFogMap() error {
	if len(g.Players) > 4 || g.Seafarers == nil || g.Seafarers.Fog == nil {
		return errors.New("此迷雾岛地图尚未支持可变布局")
	}
	fog := g.Seafarers.Fog
	terrain, numbers := []int{}, []int{}
	for _, id := range fog.StartTiles {
		terrain = append(terrain, g.Tiles[id].Resource)
		numbers = append(numbers, g.Tiles[id].Number)
	}
	shuffle(terrain)
	shuffle(numbers)
	for i, id := range fog.StartTiles {
		g.Tiles[id].Resource = terrain[i]
		g.Tiles[id].Number = numbers[i]
		if numbers[i] == 12 {
			g.Robber = id
		}
	}
	// In this scenario red numbers are explicitly allowed to touch.
	g.shuffleSeafarersPorts()
	g.Seafarers.Variable = true
	return nil
}
