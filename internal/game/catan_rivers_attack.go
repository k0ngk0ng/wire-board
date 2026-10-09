package game

import (
	"errors"
	"slices"
)

func (g *Catan) riversAttack() bool {
	return g.Rivers != nil && g.Rivers.Attack == CatanRiversAttackRules && g.Attack != nil && g.Attack.Map != nil && g.Attack.Map.Rivers == CatanRiversAttackRules
}

// The attack ledger is authoritative in the combined scenario. Rivers keeps
// no duplicate balance; this survives JSON cloning without pointer aliases.
func (g *Catan) riverGold() []int {
	if g.riversTransport() {
		return g.Transport.Gold
	}
	if g.riversAttack() {
		return g.Attack.Gold
	}
	if g.Rivers != nil {
		return g.Rivers.Gold
	}
	return nil
}
func (g *Catan) riverBank() (bank, issued, bought int) {
	if g.riversTransport() {
		return g.Transport.GoldBank, g.Transport.GoldIssued, g.Transport.Bought
	}
	if g.riversAttack() {
		return g.Attack.GoldBank, g.Attack.GoldIssued, g.Attack.Bought
	}
	if g.Rivers != nil {
		return g.Rivers.Bank, g.Rivers.GoldIssued, g.Rivers.Bought
	}
	return
}

func NewCatanRiversAttack(n int, knights ...bool) (*State, error) {
	var s *State
	var err error
	if n == 2 {
		s, err = newCatanTwoCore()
	} else {
		s, err = NewCatan(n, CatanOptions{FiveSix: n > 4})
	}
	if err != nil {
		return nil, err
	}
	board, m, r, err := newCatanRiversAttackBoard(n)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports = board.Tiles, board.Vertices, board.Edges, board.Ports
	g.HexSize, g.Robber = board.HexSize, -1
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Attack, err = newCatanAttackPieces(g, m)
	if err != nil {
		return nil, err
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Attack: CatanRiversAttackRules, Map: r}
	if n == 2 {
		g.Attack.TwoRules = CatanTwoAttackRules
		if err = g.prepareTwoNeutrals(); err != nil {
			return nil, err
		}
	}
	s.Log = []string{"河流＋蛮族进攻：起始村庄和城市在河岸均领1金币；共用金币，贫穷不扣分，自己回合达到12分获胜", "河流＋蛮族进攻：不使用强盗和最大骑士军队；桥梁可供道路骑士通行，已征服地块旁不能新建桥梁"}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人河流蛮族配方：三条河流、两座城堡、独立固定数字，使用配对回合")
	}
	if len(knights) > 0 && knights[0] {
		log := s.Log
		s.enableCitiesKnights()
		g.Rivers.Knights = CatanRiversKnightsRules
		g.Attack.City = &catanAttackCity{Rules: CatanAttackKnightsRules, Knights: []catanAttackCityKnight{}}
		g.Attack.Deck, g.Attack.Discard = []string{}, []string{}
		if n == 2 {
			g.Two.Knights = CatanTwoKnightsRules
			g.Attack.TwoRules = CatanTwoAttackKnightsRules
		}
		s.Log = append(log[1:], "本站河流蛮族城市骑士组合：13分获胜；起始河岸村庄和城市均领1金币，共用金币且贫穷不扣分；使用道路骑士和进步牌，不使用强盗和海上蛮族")
	}
	s.catanScores()
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	if err = s.validateCatanTwo(); err != nil {
		return nil, err
	}
	return s, s.validateCatanAttack()
}

func (g *Catan) validateRiversAttackMap() error {
	if !g.riversAttack() {
		return errors.New("河流蛮族组合标记无效")
	}
	r := g.Rivers
	if r.Map == nil || len(r.Gold) != 0 || r.Bank != 0 || r.GoldIssued != 0 || r.Bought != 0 || (r.Knights != "") != g.attackKnights() {
		return errors.New("河流蛮族须共用单一金币账本")
	}
	board := g
	if g.attackKnights() {
		if err := g.Attack.City.validateNumbers(g); err != nil {
			return err
		}
		restored := clone(*g)
		for id, number := range g.attackPrintedNumbers() {
			restored.Tiles[id].Number = number
		}
		board = &restored
	}
	if err := g.Attack.Map.validate(board); err != nil {
		return err
	}
	want, err := riversAttackChannels(g)
	if err != nil {
		return err
	}
	m := r.Map
	if m.DoubleNumberTile != want.DoubleNumberTile || m.NumberRecipe != "" || len(m.NumberSwaps) != 0 || !slices.Equal(m.Swamps, want.Swamps) || !slices.Equal(m.Bridges, want.Bridges) || len(m.Channels) != len(want.Channels) {
		return errors.New("河流蛮族桥位或数字无效")
	}
	for i, c := range m.Channels {
		if c.Outlet != want.Channels[i].Outlet || !slices.Equal(c.Tiles, want.Channels[i].Tiles) {
			return errors.New("河流蛮族路径无效")
		}
	}
	return nil
}

// Site supplement for the combined board: a castle knight with no currently
// legal destination may wait. Other knights still move and combat still runs.
func (g *Catan) riverAttackCastleBlocked(index, player int) bool {
	if (!g.riversAttack() && !g.attackTransport()) || index < 0 || index >= len(g.Attack.Knights) || player < 0 || player >= len(g.Players) {
		return false
	}
	a := g.Attack
	return a.castleEdge(g, a.Knights[index].Edge) && len(a.knightDestinations(g, index, 3)) == 0 && (g.Players[player].Resources[3] == 0 || len(a.knightDestinations(g, index, 5)) == 0)
}

// Select the immutable printed recipe; invention history is validated separately.
func (g *Catan) attackPrintedNumbers() []int {
	if g.attackTransport() {
		return attackTransportRecipe(len(g.Players) > 4).numbers
	}
	if g.riversAttack() {
		r, _, _, _ := catanRiversAttackRecipe(len(g.Players) > 4)
		return r.numbers
	}
	return catanAttackBoardRecipe(len(g.Players) > 4).numbers
}
