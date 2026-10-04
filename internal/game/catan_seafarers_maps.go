package game

import "fmt"

// Fixed four-player Heading for New Shores, 2025 rulebook page 5.
// The staggered six-hex middle row lies between two seven-hex rows; the sea
// frame fills the two indents, so those areas are not additional sea hexes.
func (g *Catan) makeSeafarersShoresFour() error {
	type terrain struct{ resource, number int }
	rows := [][]terrain{
		{{4, 8}, {2, 11}, {CatanSea, 0}, {CatanGold, 4}, {CatanSea, 0}},
		{{CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {CatanSea, 0}, {1, 5}, {4, 2}},
		{{CatanSea, 0}, {2, 5}, {0, 6}, {4, 4}, {CatanSea, 0}, {0, 9}, {CatanSea, 0}},
		{{3, 12}, {1, 11}, {3, 3}, {2, 9}, {CatanSea, 0}, {CatanGold, 10}},
		{{1, 6}, {0, 10}, {CatanDesert, 0}, {3, 11}, {0, 5}, {CatanSea, 0}, {CatanSea, 0}},
		{{4, 3}, {2, 4}, {1, 9}, {2, 8}, {CatanSea, 0}, {1, 3}},
		{{3, 8}, {0, 2}, {4, 10}, {CatanSea, 0}, {3, 6}},
	}
	starts := []int{0, -1, -2, -2, -3, -3, -3}
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
	// Hex sides run clockwise from the southeast edge: SE,SW,W,NW,NE,E
	// in corner-index notation (0-1,1-2,...). The vertex orientation matches
	// the existing board, making ports independent of generated edge IDs.
	ports := []struct{ row, col, side, resource int }{
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
	for _, p := range ports {
		tile := g.Tiles[indices[[2]int{p.row, p.col}]]
		a, b := tile.Vertices[p.side], tile.Vertices[(p.side+1)%6]
		edge := -1
		for _, e := range g.Edges {
			if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
				edge = e.ID
				break
			}
		}
		if edge < 0 {
			return fmt.Errorf("missing scenario port edge at %d,%d", p.row, p.col)
		}
		g.Ports = append(g.Ports, CatanPort{Edge: edge, Resource: p.resource})
	}
	g.Seafarers = &CatanSeafarers{Pirate: -1, Scenario: "shores", VictoryPoints: 14, IslandBonus: 2, Seats: make([]CatanSeafarerSeat, len(g.Players))}
	g.Seafarers.Islands = g.findIslands()
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[g.Robber]}
	return nil
}
