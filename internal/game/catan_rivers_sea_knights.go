package game

import (
	"errors"
	"slices"
)

// No printed 2025 combination page exists for rivers + seafarers + knights, so
// the nesting reuses the sea knight conversion and is marked as a site rule.
const CatanRiversSeaKnightsRules = "wire-board-rivers-sea-knights-v1"

func (g *Catan) riversSeaKnights() bool {
	return g.riversSea() && g.riverKnights() && g.Rivers.SeaKnights == CatanRiversSeaKnightsRules
}

// riversSeaVictoryPoints keeps every printed river-sea target while the knight
// conversion raises it by the two points the sea knight recipes add.
func (g *Catan) riversSeaVictoryPoints(base int) int {
	if g.riversSeaKnights() {
		return base + 2
	}
	return base
}

func NewCatanRiversSeaCitiesKnights(n int, setup CatanRiversSeafarersSetup, world *CatanRiversWorldMap) (*State, error) {
	s, err := NewCatanRiversSeafarers(n, setup, world)
	if err != nil {
		return nil, err
	}
	logs := slices.Clone(s.Log)
	if err = s.enableCitiesKnightsSeafarers(); err != nil {
		return nil, err
	}
	g := s.Catan
	g.Rivers.Knights, g.Rivers.SeaKnights = CatanRiversKnightsRules, CatanRiversSeaKnightsRules
	if g.Two != nil {
		g.Two.Knights, g.Two.SeaKnights = CatanTwoKnightsRules, CatanTwoSeafarersKnightsRules
	}
	s.Log = append(logs, "本站河流海图骑士适配：沿用河流金币账本与河岸付款，加入进步牌、商品与城市改良；蛮族船取代海盗，首次进攻前强盗不入场；目标分在印刷目标上增加2分")
	s.catanScores()
	if err = g.validateRivers(); err != nil {
		return nil, err
	}
	if g.Two != nil {
		err = s.validateCatanTwo()
	}
	return s, err
}

func (g *Catan) validateRiversSeaKnights() error {
	r := g.Rivers
	if r.SeaKnights == "" && !g.riversSeaKnights() {
		return nil
	}
	if !g.riversSeaKnights() || g.Seafarers == nil || r.Map == nil {
		return errors.New("河流海图骑士标记无效")
	}
	if r.Attack != "" || r.Transport != "" {
		return errors.New("河流海图骑士不与商队、蛮族进攻或运输组合叠加")
	}
	if g.Fishing != nil && !g.fishingRiversSea() {
		return errors.New("河流海图骑士的渔夫组合标记无效")
	}
	// The barbarian ship replaces the pirate; it only sails from PirateStart
	// once the first invasion happens.
	if g.CitiesKnights.PirateStart < -1 || g.Seafarers.Pirate >= 0 && g.Seafarers.Pirate != g.CitiesKnights.PirateStart {
		return errors.New("河流海图骑士的蛮族船起位无效")
	}
	if g.CitiesKnights.Invasions == 0 && (g.Robber != -1 || g.Seafarers.Pirate != -1) {
		return errors.New("首次蛮族进攻前强盗与蛮族船不能入场")
	}
	return nil
}
