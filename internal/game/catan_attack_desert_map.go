package game

// Printed Barbarian Attack + Seafarers 2025, p2, three-player Desert.
// This is map geometry only; landing order and complete play are separate.
func newCatanAttackDesertThreeGeometry() (*Catan, error) {
	s, err := NewCatanSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	for id, t := range map[int]seaTerrain{
		6: {0, 12}, 11: {1, 8}, 12: {2, 9}, 16: {4, 4}, 17: {0, 3}, 20: {catanCastle, 0}, 21: {1, 6}, 22: {3, 11}, 23: {1, 8}, 26: {4, 2}, 27: {3, 9}, 28: {0, 10}, 31: {2, 5}, 32: {2, 11},
	} {
		g.Tiles[id].Resource, g.Tiles[id].Number = t.resource, t.number
	}
	// The grain port moves to the SW-facing side of the castle, while the
	// generic port at forest 3 faces east; the remaining frame ports are unchanged.
	g.Ports[1].Edge = catanFishingSide(g, 20, 1)
	g.Ports[2].Edge = catanFishingSide(g, 16, 2)
	g.Ports[3].Edge = catanFishingSide(g, 23, 0)
	g.Ports[4].Edge = catanFishingSide(g, 17, 5)
	g.Robber, g.Seafarers.Pirate = -1, -1
	g.Seafarers.Islands = g.findLandRegions(true)
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[22]}
	g.Seafarers.VictoryPoints = 14
	return g, nil
}
