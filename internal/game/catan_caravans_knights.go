package game

import "errors"

const CatanCaravansKnightsRules = "catan-caravans-knights-2025"

// The printed combination changes bids and the target; all city rules remain.
func NewCatanCaravansCitiesKnights(n int, options CatanOptions) (*State, error) {
	var s *State
	var err error
	if n == 2 {
		s, err = NewCatanTwoCaravans(n, options)
	} else {
		s, err = NewCatanCaravans(n, options)
	}
	if err != nil {
		return nil, err
	}
	s.enableCitiesKnights()
	g := s.Catan
	g.Caravans.Knights = CatanCaravansKnightsRules
	if g.Two != nil {
		g.Two.Knights = CatanTwoKnightsRules
	}
	s.Log = []string{"商队＋城市与骑士：顺序放村庄、逆序放城市；商队以木材和砖块出价，自己回合达到15分获胜；首次蛮族进攻前强盗不入场"}
	if n > 4 {
		s.Log = append(s.Log, catanExtendedNumberNotice)
	}
	if n == 2 {
		s.Log = append(s.Log, "双人组合：每回合两次完整事件与生产；沿用中立骑士与双人商队最多两辆马车规则，贸易筹码供应有限")
	}
	s.catanScores()
	if err = s.validateCaravans(); err != nil {
		return nil, err
	}
	if g.Two != nil {
		err = s.validateCatanTwo()
	}
	return s, err
}

func (g *Catan) caravanKnights() bool {
	return g.Caravans != nil && g.Caravans.Knights == CatanCaravansKnightsRules && g.CitiesKnights != nil
}
func (g *Catan) caravanBidColors() []int {
	if g.caravanKnights() {
		return []int{0, 1}
	}
	return []int{2, 3}
}
func (g *Catan) caravanBidNames() string {
	if g.caravanKnights() {
		return "木材或砖块"
	}
	return "羊毛或粮食"
}
func (g *Catan) validCaravanBid(bid []int) bool {
	if !catanBundle(bid) {
		return false
	}
	colors := g.caravanBidColors()
	for color, n := range bid {
		if color != colors[0] && color != colors[1] && n != 0 {
			return false
		}
	}
	return true
}
func (g *Catan) validateCaravanKnights() error {
	c := g.Caravans
	if c.Knights == "" && g.CitiesKnights == nil {
		return nil
	}
	if !g.caravanKnights() || g.CitiesKnights.Rules != catanCitiesKnightsRules(len(g.Players)) || g.CitiesKnights.RobberStart != -1 && !g.riversCaravans() {
		return errors.New("商队城市骑士组合标记无效")
	}
	return nil
}
