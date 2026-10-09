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

func newCatanAttackDesertThreeBoard() (*Catan, *catanAttackMap, error) {
	g, e := newCatanAttackDesertThreeGeometry()
	if e != nil {
		return nil, nil, e
	}
	// Clockwise coast of the starting region; the desert boundary is not sea.
	return g, &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: []int{20}, Coast: []int{6, 12, 17, 23, 28, 32, 31, 26, 16}, Barbarians: 36, Gold: 100}, nil
}
func newCatanAttackDesertThree() (*State, error) {
	board, m, e := newCatanAttackDesertThreeBoard()
	if e != nil {
		return nil, e
	}
	s, e := NewCatan(3, CatanOptions{})
	if e != nil {
		return nil, e
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize, g.Seafarers = board.Tiles, board.Vertices, board.Edges, board.Ports, board.HexSize, board.Seafarers
	g.Robber = -1
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Attack, e = newCatanAttackPieces(g, m)
	if e != nil {
		return nil, e
	}
	s.Log = []string{"蛮族穿越沙漠：骑士和蛮族仅在起始主岛活动，外区建设同样触发登陆；无强盗海盗，14分获胜。"}
	s.catanScores()
	return s, s.validateCatanAttack()
}

func newCatanAttackDesertFourBoard() (*Catan, *catanAttackMap, error) {
	s, e := NewCatanSeafarers(4, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
	if e != nil {
		return nil, nil, e
	}
	g := s.Catan
	for id, t := range map[int]seaTerrain{2: {0, 9}, 7: {1, 6}, 8: {2, 4}, 13: {4, 11}, 15: {0, 12}, 19: {catanCastle, 0}, 20: {1, 5}, 24: {1, 2}, 26: {3, 4}, 27: {0, 3}, 31: {2, 5}, 33: {4, 8}, 38: {0, 10}} {
		g.Tiles[id].Resource, g.Tiles[id].Number = t.resource, t.number
	}
	g.Robber, g.Seafarers.Pirate = -1, -1
	g.Seafarers.Islands = g.findLandRegions(true)
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[26]}
	g.Seafarers.VictoryPoints = 14
	return g, &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: []int{19}, Coast: []int{7, 8, 15, 21, 27, 33, 38, 32, 31, 24}, Barbarians: 36, Gold: 100}, nil
}
func newCatanAttackDesert(n int) (*State, error) {
	if n == 3 {
		return newCatanAttackDesertThree()
	}
	board, m, e := newCatanAttackDesertFourBoard()
	if e != nil {
		return nil, e
	}
	s, e := NewCatan(n, CatanOptions{})
	if e != nil {
		return nil, e
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize, g.Seafarers = board.Tiles, board.Vertices, board.Edges, board.Ports, board.HexSize, board.Seafarers
	g.Robber = -1
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Attack, e = newCatanAttackPieces(g, m)
	if e != nil {
		return nil, e
	}
	s.catanScores()
	return s, s.validateCatanAttack()
}
