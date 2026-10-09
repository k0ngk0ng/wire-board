package game

const CatanTwoAttackSeaRules = "wire-board-two-attack-sea-v1"

func (g *Catan) twoAttackSea() bool {
	return g.twoAttack() && g.twoSeafarers() && g.attackSea() && g.Two.AttackSea == CatanTwoAttackSeaRules
}
func newCatanTwoAttackShores() (*State, error) {
	board, m, err := newCatanAttackShoresBoard(4)
	if err != nil {
		return nil, err
	}
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize, g.Seafarers = board.Tiles, board.Vertices, board.Edges, board.Ports, board.HexSize, board.Seafarers
	g.Seafarers.Seats = make([]CatanSeafarerSeat, 2)
	g.Robber = -1
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Two = &CatanTwo{Rules: CatanTwoRules, Seafarers: CatanTwoSeafarersRules, AttackSea: CatanTwoAttackSeaRules, Rolls: []int{}, Tokens: []int{5, 5}, Bank: 10}
	g.Attack, err = newCatanAttackPieces(g, m)
	if err != nil {
		return nil, err
	}
	g.Attack.TwoRules = CatanTwoAttackRules
	if err = g.prepareTwoSeaNeutrals(); err != nil {
		return nil, err
	}
	g.StartPlayer = catanRandom(2)
	s.Turn = g.StartPlayer
	s.Log = []string{"本站双人蛮族新海岸：四人组合地图，两家中立海岸村庄；真人两次生产，建船接中立船，中立村庄另触发登陆；共享中立骑士只在主岛活动，14分获胜。"}
	s.catanScores()
	if err = s.validateCatanAttack(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTwo()
}
