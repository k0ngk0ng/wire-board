package game

import "fmt"

type seaTerrain struct{ resource, number int }

// Side indices run SE, SW, W, NW, NE, E (corner k to corner k+1).
type seaPort struct{ row, col, side, resource int }

// Fixed four-player Heading for New Shores, 2025 rulebook page 5.
// The staggered six-hex middle row lies between two seven-hex rows; the sea
// frame fills the two indents, so those areas are not additional sea hexes.
func (g *Catan) makeSeafarersShoresFour() error {
	rows := [][]seaTerrain{
		{{4, 8}, {2, 11}, {CatanSea, 0}, {CatanGold, 4}, {CatanSea, 0}},
		{{CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {1, 5}, {4, 2}},
		{{CatanSea, 0}, {2, 5}, {0, 6}, {4, 4}, {CatanSea, 0}, {0, 9}, {CatanSea, 0}},
		{{3, 12}, {1, 11}, {3, 3}, {2, 9}, {CatanSea, 0}, {CatanGold, 10}},
		{{1, 6}, {0, 10}, {CatanDesert, 0}, {3, 11}, {0, 5}, {CatanSea, 0}, {CatanSea, 0}},
		{{4, 3}, {2, 4}, {1, 9}, {2, 8}, {CatanSea, 0}, {1, 3}},
		{{3, 8}, {0, 2}, {4, 10}, {CatanSea, 0}, {3, 6}},
	}
	ports := []seaPort{
		{2, 1, 2, -1}, // west of pasture 5
		{2, 3, 3, 4},  // northwest of mountains 4
		{3, 3, 4, -1}, // northeast of pasture 9
		{3, 0, 2, 2},  // west of fields 12, on the frame
		{4, 0, 1, 1},  // southwest of hills 6
		{4, 4, 0, 3},  // southeast of forest 5
		{6, 0, 2, -1}, // west of fields 8
		{6, 1, 1, 0},  // southwest of forest 2
		{6, 2, 5, -1}, // east of mountains 10
	}
	return g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "shores", 14, [2]int{4, 2}, [][2]int{{4, 2}})
}

// Fixed three-player Heading for New Shores, 2025 rulebook page 4.
// The robber starts on the northern small island, NOT on the home island.
func (g *Catan) makeSeafarersShoresThree() error {
	rows := [][]seaTerrain{
		{{1, 12}, {CatanGold, 5}, {CatanSea, 0}, {CatanSea, 0}},
		{{CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {2, 4}, {4, 9}},
		{{CatanSea, 0}, {3, 4}, {2, 6}, {CatanSea, 0}, {3, 3}, {CatanSea, 0}},
		{{2, 2}, {4, 5}, {0, 10}, {CatanSea, 0}, {CatanGold, 4}},
		{{1, 8}, {2, 10}, {2, 9}, {0, 8}, {CatanSea, 0}, {CatanSea, 0}},
		{{3, 11}, {4, 3}, {1, 11}, {CatanSea, 0}, {4, 8}},
		{{3, 6}, {0, 5}, {CatanSea, 0}, {1, 10}},
	}
	ports := []seaPort{
		{2, 1, 2, 3}, {2, 2, 5, -1}, {4, 0, 3, 4}, {4, 0, 1, -1},
		{4, 3, 4, 2}, {6, 0, 2, 1}, {6, 1, 5, -1}, {6, 1, 1, 0},
	}
	return g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "shores", 14, [2]int{0, 0}, [][2]int{{3, 2}})
}

// Fixed three-player Four Islands, 2025 rulebook page 6.
func (g *Catan) makeSeafarersIslandsThree() error {
	rows := [][]seaTerrain{
		{{CatanSea, 0}, {CatanSea, 0}, {3, 4}, {2, 3}},
		{{4, 4}, {0, 9}, {CatanSea, 0}, {3, 9}, {1, 5}},
		{{2, 6}, {4, 10}, {CatanSea, 0}, {0, 8}, {1, 11}, {CatanSea, 0}},
		{{CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}},
		{{3, 11}, {4, 8}, {0, 3}, {CatanSea, 0}, {1, 10}, {1, 6}},
		{{0, 5}, {2, 9}, {CatanSea, 0}, {4, 2}, {3, 5}},
		{{2, 12}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}},
	}
	ports := []seaPort{
		{1, 4, 4, -1}, {1, 1, 0, 4}, {2, 0, 3, -1}, {2, 3, 0, 1},
		{4, 0, 4, 0}, {4, 0, 1, -1}, {5, 1, 0, 2}, {5, 3, 3, -1}, {4, 5, 0, 3},
	}
	return g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "islands", 13, [2]int{6, 0}, nil)
}

// Fixed four-player Four Islands, 2025 rulebook page 7.
func (g *Catan) makeSeafarersIslandsFour() error {
	rows := [][]seaTerrain{
		{{2, 8}, {CatanSea, 0}, {0, 9}, {0, 11}},
		{{1, 10}, {CatanSea, 0}, {4, 3}, {3, 12}, {2, 5}},
		{{3, 5}, {0, 3}, {CatanSea, 0}, {1, 5}, {4, 10}, {CatanSea, 0}},
		{{CatanSea, 0}, {CatanSea, 0}, {0, 6}, {CatanSea, 0}, {CatanSea, 0}},
		{{1, 4}, {2, 9}, {CatanSea, 0}, {CatanSea, 0}, {0, 9}, {2, 11}},
		{{3, 6}, {4, 4}, {1, 2}, {CatanSea, 0}, {4, 8}},
		{{2, 10}, {3, 11}, {CatanSea, 0}, {3, 4}},
	}
	ports := []seaPort{
		{1, 0, 2, 3}, {2, 0, 0, -1}, {3, 2, 3, -1}, {2, 4, 5, 0},
		{4, 5, 3, 4}, {4, 5, 0, -1}, {5, 1, 4, -1}, {4, 0, 1, 1}, {6, 0, 2, 2},
	}
	return g.makeSeafarersFixed(rows, []int{0, -1, -2, -2, -3, -3, -3}, ports, "islands", 13, [2]int{1, 3}, nil)
}

// Coordinate-based ports and markers keep the printed scenarios independent
// of generated edge IDs. Empty start coordinates mean each player's two
// starting settlements establish their personal home islands.
func (g *Catan) makeSeafarersFixed(rows [][]seaTerrain, starts []int, ports []seaPort, scenario string, victory int, robber [2]int, homes [][2]int) error {
	if len(rows) != len(starts) {
		return fmt.Errorf("scenario row coordinates mismatch")
	}
	specs := []CatanHexSpec{}
	indices := map[[2]int]int{}
	for r, row := range rows {
		for col, t := range row {
			indices[[2]int{r, col}] = len(specs)
			specs = append(specs, CatanHexSpec{Q: starts[r] + col, R: r, Resource: t.resource, Number: t.number})
		}
	}
	if err := g.makeScenarioMap(specs); err != nil {
		return err
	}
	g.Seafarers = &CatanSeafarers{Pirate: -1, Scenario: scenario, VictoryPoints: victory, IslandBonus: 2, Seats: make([]CatanSeafarerSeat, len(g.Players))}
	for _, p := range ports {
		id, ok := indices[[2]int{p.row, p.col}]
		if !ok || p.side < 0 || p.side > 5 || p.resource < -1 || p.resource > 4 {
			return fmt.Errorf("invalid scenario port")
		}
		tile := g.Tiles[id]
		a, b := tile.Vertices[p.side], tile.Vertices[(p.side+1)%6]
		edge := -1
		for _, e := range g.Edges {
			if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
				edge = e.ID
				break
			}
		}
		if edge < 0 || !g.edgeTerrain(edge, true) || !g.edgeTerrain(edge, false) {
			return fmt.Errorf("missing coastal port edge at %d,%d", p.row, p.col)
		}
		g.Ports = append(g.Ports, CatanPort{Edge: edge, Resource: p.resource})
	}
	id, ok := indices[robber]
	if !ok || g.Tiles[id].Resource == CatanSea {
		return fmt.Errorf("invalid starting robber position")
	}
	g.Robber = id
	g.Seafarers.Islands = g.findIslands()
	for _, coord := range homes {
		id, ok := indices[coord]
		if !ok || g.Seafarers.Islands[id] < 0 {
			return fmt.Errorf("invalid starting island")
		}
		g.Seafarers.StartIslands = append(g.Seafarers.StartIslands, g.Seafarers.Islands[id])
	}
	return nil
}
