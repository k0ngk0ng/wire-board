package game

import "fmt"

// Heading for New Shores, 2025 Seafarers 5–6 rulebook page 4. Only the main
// island is variable; the ten outer land hexes/numbers stay exactly as printed.
func (g *Catan) makeSeafarersShoresSix() error {
	if len(g.Players) < 5 || len(g.Players) > 6 {
		return fmt.Errorf("此航海家地图需要5至6位玩家")
	}
	// Use the base extension's numbered spiral for the 30-hex main island.
	// Do not replace its A–Zc sequence with unrestricted number shuffling.
	main := &Catan{Players: g.Players}
	main.makeMap()
	rows := [][]seaTerrain{
		{{CatanGold, 9}, {CatanSea, 0}, {0, 0}, {0, 0}, {0, 0}, {CatanSea, 0}, {4, 6}},
		{{4, 11}, {CatanSea, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {CatanSea, 0}, {CatanSea, 0}},
		{{2, 8}, {CatanSea, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {CatanSea, 0}, {1, 12}},
		{{CatanSea, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {CatanSea, 0}},
		{{1, 4}, {CatanSea, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {CatanSea, 0}, {3, 3}},
		{{0, 2}, {CatanSea, 0}, {0, 0}, {0, 0}, {0, 0}, {0, 0}, {CatanSea, 0}, {CatanSea, 0}},
		{{CatanGold, 5}, {CatanSea, 0}, {0, 0}, {0, 0}, {0, 0}, {CatanSea, 0}, {CatanGold, 10}},
	}
	at := 0
	robber := [2]int{}
	for row, count := range []int{3, 4, 5, 6, 5, 4, 3} {
		start := 2
		if row == 3 {
			start = 1
		}
		for col := start; col < start+count; col++ {
			t := main.Tiles[at]
			rows[row][col] = seaTerrain{t.Resource, t.Number}
			if at == main.Robber {
				robber = [2]int{row, col}
			}
			at++
		}
	}
	ports := []seaPort{
		{0, 3, 4, -1}, {1, 2, 3, -1}, {1, 5, 4, -1}, {2, 2, 2, -1}, {2, 6, 5, -1},
		{3, 1, 1, 0}, {3, 6, 0, 1}, {5, 5, 5, 2}, {5, 2, 1, 3}, {6, 3, 1, 4}, {6, 4, 0, 2},
	}
	if err := g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "shores", 14, robber, [][2]int{robber}); err != nil {
		return err
	}
	// The blue outlines explicitly join the two northeastern land hexes into
	// one exploration region, and likewise the two southeastern land hexes.
	// Each has a sea hex inside its outline. Sea stays -1 so it cannot become
	// a settlement site or a home island. 48 VP tokens = 6 players × 4 regions × 2.
	for _, pair := range [][2]int{{6, 23}, {40, 55}} {
		from, to := g.Seafarers.Islands[pair[1]], g.Seafarers.Islands[pair[0]]
		for id, region := range g.Seafarers.Islands {
			if region == from {
				g.Seafarers.Islands[id] = to
			}
		}
	}
	g.Seafarers.Variable = true
	g.shuffleSeafarersPorts()
	return nil
}

// Six Islands, 2025 Seafarers 5–6 rulebook page 5. The six island areas are
// fixed, ports are shuffled, and each player has one or two personal homes.
func (g *Catan) makeSeafarersIslandsSix() error {
	if len(g.Players) < 5 || len(g.Players) > 6 {
		return fmt.Errorf("此航海家地图需要5至6位玩家")
	}
	rows := [][]seaTerrain{
		{{2, 12}, {4, 4}, {CatanSea, 0}, {0, 6}, {CatanSea, 0}, {4, 9}, {2, 6}},
		{{0, 8}, {4, 10}, {CatanSea, 0}, {4, 11}, {1, 10}, {CatanSea, 0}, {3, 3}, {1, 10}},
		{{1, 11}, {3, 9}, {CatanSea, 0}, {2, 3}, {0, 4}, {CatanSea, 0}, {CatanSea, 0}, {0, 4}, {CatanSea, 0}},
		{{CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}},
		{{CatanSea, 0}, {1, 4}, {CatanSea, 0}, {CatanSea, 0}, {2, 8}, {0, 10}, {CatanSea, 0}, {0, 5}, {3, 6}},
		{{0, 9}, {3, 12}, {CatanSea, 0}, {3, 5}, {4, 3}, {CatanSea, 0}, {1, 8}, {2, 9}},
		{{2, 5}, {4, 6}, {CatanSea, 0}, {2, 2}, {CatanSea, 0}, {1, 5}, {3, 2}},
	}
	ports := []seaPort{
		{0, 1, 3, -1}, {0, 3, 4, -1}, {0, 6, 4, -1}, {1, 1, 0, -1}, {2, 1, 1, -1},
		{1, 6, 1, 0}, {5, 0, 2, 1}, {6, 1, 4, 2}, {5, 3, 1, 3}, {5, 7, 5, 4}, {6, 5, 1, 2},
	}
	if err := g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "six_islands", 13, [2]int{0, 0}, nil); err != nil {
		return err
	}
	g.shuffleSeafarersPorts()
	return nil
}

func (g *Catan) shuffleSeafarersPorts() {
	resources := []int{}
	for _, p := range g.Ports {
		resources = append(resources, p.Resource)
	}
	shuffle(resources)
	for i := range g.Ports {
		g.Ports[i].Resource = resources[i]
	}
}
