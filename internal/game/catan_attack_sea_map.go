package game

import "errors"

const CatanAttackSeafarersRules = "catan-attack-seafarers-2025"

// Official combination p2: install the ordinary Barbarian Attack island
// into the four-player New Shores frame, preserving the printed outer islands.
func newCatanAttackShoresBoard(n int) (*Catan, *catanAttackMap, error) {
	if n != 4 {
		return nil, nil, errors.New("此蛮族新海岸配方需要四人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "shores", Layout: "fixed"}, nil)
	if err != nil {
		return nil, nil, err
	}
	base, m, err := newCatanAttackBoard(n)
	if err != nil {
		return nil, nil, err
	}
	g := s.Catan
	ids := []int{12, 13, 14, 18, 19, 20, 21, 24, 25, 26, 27, 28, 31, 32, 33, 34, 37, 38, 39}
	for i, id := range ids {
		g.Tiles[id].Resource, g.Tiles[id].Number = base.Tiles[i].Resource, base.Tiles[i].Number
	}
	remap := func(values []int) []int {
		out := make([]int, len(values))
		for i, id := range values {
			out[i] = ids[id]
		}
		return out
	}
	mapped := &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: remap(m.Castles), Coast: remap(m.Coast), Barbarians: m.Barbarians, Gold: m.Gold}
	// Transfer printed attack ports by hex-side identity, not vertex numbering.
	g.Ports = nil
	for _, p := range base.Ports {
		found := false
		for _, tile := range base.Edges[p.Edge].Tiles {
			for side := 0; side < 6; side++ {
				if catanFishingSide(base, tile, side) == p.Edge {
					edge := catanFishingSide(g, ids[tile], side)
					if edge < 0 {
						return nil, nil, errors.New("蛮族海岸港口映射失败")
					}
					g.Ports = append(g.Ports, CatanPort{Edge: edge, Resource: p.Resource})
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return nil, nil, errors.New("蛮族海岸原港口缺失")
		}
	}
	g.Robber, g.Seafarers.Pirate = -1, -1
	g.Seafarers.Islands = g.findIslands()
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[26]}
	g.Seafarers.VictoryPoints = 14
	return g, mapped, nil
}
