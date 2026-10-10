package game

import "slices"

// The printed attack-plus-knights page only covers the land map. The sea
// nesting reuses those road-knight rules on the printed attack sea maps: the
// barbarian ship track and the pirate-island fleet dice stay out, landings and
// city battles keep driving the attacks, and each target rises by one point.
const CatanAttackSeaKnightsRules = "wire-board-attack-sea-knights-v1"

func (g *Catan) attackSeaKnights() bool {
	return g.attackSea() && g.attackKnights() && g.Attack.SeaKnights == CatanAttackSeaKnightsRules
}

func (g *Catan) attackSeaVictoryPoints(base int) int {
	if g.attackSeaKnights() {
		return base + 1
	}
	return base
}

func NewCatanAttackSeaCitiesKnights(n int, scenario string) (*State, error) {
	s, err := NewCatanAttackSea(n, scenario)
	if err != nil {
		return nil, err
	}
	logs := slices.Clone(s.Log)
	s.enableCitiesKnights()
	g := s.Catan
	if t := g.tribe(); t == nil || t.AttackRules == "" {
		g.initTribeProgress()
	}
	if p := g.pirateIslands(); p != nil {
		// The printed pirate islands attack map keeps its own fortress and
		// fleet track, so the pirate is not parked and no fleet dice roll.
		p.KnightsRules = CatanPirateKnightsRules
		g.CitiesKnights.PirateStart = -1
	} else if g.Seafarers != nil {
		g.CitiesKnights.PirateStart = g.Seafarers.Pirate
		g.Seafarers.Pirate = -1
	}
	a := g.Attack
	a.City = &catanAttackCity{Rules: CatanAttackKnightsRules, Knights: []catanAttackCityKnight{}}
	a.SeaKnights = CatanAttackSeaKnightsRules
	a.Deck, a.Discard = []string{}, []string{}
	g.Seafarers.VictoryPoints = g.attackSeaVictoryPoints(g.Seafarers.VictoryPoints)
	if g.Two != nil {
		g.Two.Knights = CatanTwoKnightsRules
		a.TwoRules = CatanTwoAttackKnightsRules
	}
	s.Log = append(logs, "本站蛮族海图＋城市骑士适配：沿用印刷海图、登陆与城堡规则，取消城市蛮族船轨与海盗舰队骰；骑士在主岛城堡边招募并按等级作战，进步牌与商品照常使用，目标分在印刷目标上加 1")
	s.catanScores()
	if err = s.validateCatanAttack(); err != nil {
		return nil, err
	}
	return s, a.City.validate(g)
}
