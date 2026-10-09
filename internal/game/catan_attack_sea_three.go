package game

// Official combination p1, row-major three-player New Shores frame.
// Coastal 2/12 is one landing tile and produces on either face.
func newCatanAttackShoresThreeBoard() (*Catan, *catanAttackMap, error) {
	s, err := NewCatanSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "shores", Layout: "fixed"}, nil)
	if err != nil {
		return nil, nil, err
	}
	g := s.Catan
	coast := []int{10, 11, 17, 23, 28, 32, 26, 20, 15}
	inner := []int{16, 21, 22, 27}
	resources := []int{0, 0, 1, 1, 2, 2, 3, 3, 4}
	shuffle(resources)
	numbers := []int{8, 10, 11, 5, 2, 6, 4, 9, 3}
	for i, id := range coast {
		g.Tiles[id].Resource, g.Tiles[id].Number = resources[i], numbers[i]
	}
	resources = []int{0, 2, 3, 4}
	shuffle(resources)
	for i, id := range inner {
		g.Tiles[id].Resource = resources[i]
		g.Tiles[id].Number = []int{4, 5, 9, 10}[i]
	}
	g.Tiles[31].Resource, g.Tiles[31].Number = catanCastle, 0
	g.Robber, g.Seafarers.Pirate = -1, -1
	g.Seafarers.Islands = g.findIslands()
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[16]}
	g.Seafarers.VictoryPoints = 14
	return g, &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: []int{31}, Coast: coast, Barbarians: 36, Gold: 100, ExtraNumbers: []catanFishingExtraNumber{{Tile: 28, Number: 12}}}, nil
}
