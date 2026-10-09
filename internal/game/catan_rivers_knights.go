package game

import (
	"errors"
	"slices"
)

const CatanRiversKnightsRules = "catan-rivers-knights-2025"

func NewCatanRiversCitiesKnights(n int, options CatanOptions) (*State, error) {
	var s *State
	var err error
	if n == 2 {
		s, err = NewCatanTwoRivers(n, options)
	} else {
		s, err = NewCatanRivers(n, options)
	}
	if err != nil {
		return nil, err
	}
	s.enableCitiesKnights()
	g := s.Catan
	g.Rivers.Knights = CatanRiversKnightsRules
	if g.Two != nil {
		g.Two.Knights = CatanTwoKnightsRules
	}
	s.Log = []string{"河流＋城市与骑士：起始村庄和城市在河岸均领1金币；13分获胜；首次蛮族进攻前强盗留在棋盘外", "商品可按兑换比例出售换金币，金币只能购买普通资源；可付5金币免除一次城市劫掠"}
	if n > 4 {
		s.Log = append(s.Log, catanExtendedNumberNotice)
	}
	s.catanScores()
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	if g.Two != nil {
		err = s.validateCatanTwo()
	}
	return s, err
}
func (g *Catan) riverKnights() bool {
	return g.Rivers != nil && g.Rivers.Knights == CatanRiversKnightsRules && g.CitiesKnights != nil
}
func (g *Catan) validateRiverKnights() error {
	r := g.Rivers
	if r.Knights == "" && g.CitiesKnights == nil {
		return nil
	}
	if !g.riverKnights() || g.CitiesKnights.Rules != catanCitiesKnightsRules(len(g.Players)) || r.Map == nil {
		return errors.New("河流城市骑士组合标记无效")
	}
	if g.riversAttack() {
		if !g.attackKnights() || g.CitiesKnights.RobberStart != -1 || g.Robber != -1 {
			return errors.New("河流蛮族城市骑士不使用强盗")
		}
		return nil
	}
	start := g.CitiesKnights.RobberStart
	if !slices.Contains(r.Map.Swamps, start) && !(g.SetupStep == 0 && start == -1) {
		return errors.New("河流骑士强盗起点必须为沼泽")
	}
	if g.CitiesKnights.Invasions == 0 && g.Robber != -1 {
		return errors.New("首次蛮族进攻前强盗不能入场")
	}
	return nil
}
func (g *Catan) canRiverPillageGold(player int) bool {
	return g.riverKnights() && !g.riversAttack() && player >= 0 && player < len(g.Players) && g.Rivers.Gold[player] >= 5 && len(g.pillageSites(player)) > 0
}
func (g *Catan) diplomacyRoadsFor(player int) []int {
	roads := g.diplomacyRoads()
	if g.riverKnights() {
		roads = slices.DeleteFunc(roads, func(id int) bool { return g.riverEdge(id) && g.riverGold()[player] < 1 })
	}
	return roads
}
