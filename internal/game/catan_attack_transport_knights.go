package game

import (
	"errors"
	"slices"
)

const CatanAttackTransportKnightsRules = "wire-board-attack-transport-knights-v1"

func (g *Catan) attackTransportKnights() bool {
	return g.attackTransport() && g.attackKnights() && g.Transport.Knights == CatanAttackTransportKnightsRules
}
func newCatanAttackTransportKnights(n int) (*State, error) {
	s, err := newCatanAttackTransportState(n)
	if err != nil {
		return nil, err
	}
	logs := slices.Clone(s.Log)
	logs[0] = "蛮族进攻＋运输＋城市骑士：共用蛮族和金币，使用道路骑士与进步牌；不使用强盗、最长道路和最大骑士军队，14分获胜"
	s.enableCitiesKnights()
	g := s.Catan
	g.Attack.City = &catanAttackCity{Rules: CatanAttackKnightsRules, Knights: []catanAttackCityKnight{}}
	g.Attack.Deck, g.Attack.Discard = []string{}, []string{}
	g.Transport.Knights = CatanAttackTransportKnightsRules
	if n == 2 {
		g.Two.Knights = CatanTwoKnightsRules
		g.Attack.TwoRules = CatanTwoAttackKnightsRules
	}
	// Site triple recipe: retain the Attack+Transport terrain and numbers;
	// the Attack road-knight rules replace the independent transport knights.
	s.Log = append(logs, "本站蛮族运输城市骑士组合：保留组合地图，使用道路骑士与进步牌，不使用海上蛮族；蛮族不足时继续记账发放，共用金币，14分获胜；2／12总是登陆，船面时不重复登陆")
	s.catanScores()
	return s, s.validateCatanTransport()
}
func (g *Catan) validateAttackTransportMap(b *catanAttackTransportBoard) error {
	if !g.attackKnights() {
		return b.validate(g)
	}
	if !g.attackTransportKnights() {
		return errors.New("蛮族运输城市骑士组合标记无效")
	}
	if err := g.Attack.City.validateNumbers(g); err != nil {
		return err
	}
	restored := *g
	restored.Tiles = slices.Clone(g.Tiles)
	for id, number := range g.attackPrintedNumbers() {
		restored.Tiles[id].Number = number
	}
	return b.validate(&restored)
}
func (s *State) attackTransportCityLanding(dice [2]int) ([]int, error) {
	g := s.Catan
	if !g.attackTransportKnights() || dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 {
		return nil, errors.New("共享蛮族登陆骰无效")
	}
	total := dice[0] + dice[1]
	targets := []int{}
	if total == 7 {
		return targets, nil
	}
	for _, tile := range g.Attack.Map.Coast {
		if g.Tiles[tile].Number == total && !g.Attack.conquered(tile) {
			targets = append(targets, tile)
		}
	}
	p := &g.AttackTransport.Pieces
	missing := max(0, len(targets)-p.supply())
	if g.Attack.City.Issued > catanGoldLedgerLimit-missing {
		return nil, errors.New("共享蛮族发放超出记账上限")
	}
	next := catanAttackTransportPieces{Barbarians: slices.Clone(p.Barbarians)}
	for range missing {
		next.Barbarians = append(next.Barbarians, catanAttackTransportBarbarian{-1, -1, -1})
	}
	// Update a trial state too: piece validation includes the issued ledger.
	trial := *g
	a := *g.Attack
	c := *a.City
	c.Issued += missing
	a.City = &c
	trial.Attack = &a
	for _, tile := range targets {
		if _, err := next.land(&trial, trial.attackTransportBoard(), tile); err != nil {
			return nil, err
		}
	}
	g.Attack.City.Issued = c.Issued
	*p = next
	g.syncAttackTransportCounts()
	if m := g.CitiesKnights.Merchant; m != nil && g.Attack.conquered(m.Tile) {
		g.CitiesKnights.Merchant = nil
	}
	return targets, nil
}
func (g *Catan) captureAttackBattle(tile int, prisoners []int) error {
	if g.attackTransport() {
		if err := g.AttackTransport.Pieces.captureBattle(g, g.attackTransportBoard(), tile, prisoners); err != nil {
			return err
		}
		g.syncAttackTransportCounts()
		return nil
	}
	g.Attack.Barbarians[tile] = 0
	for p, n := range prisoners {
		g.Attack.Prisoners[p] += n
	}
	return nil
}
