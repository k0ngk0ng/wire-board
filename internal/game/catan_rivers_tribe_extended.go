package game

import (
	"errors"
	"reflect"
	"slices"
)

const catanRiverTribeExtended = "wire-board-tribe-three-rivers-v1"

func (g *Catan) makeRiversTribeExtendedMap() (*catanRiversMap, error) {
	paths := [][]int{{20, 19, 18, 17}, {22, 32, 42}, {24, 34, 44}}
	home := g.Seafarers.StartIslands[0]
	allowed := func(id int) bool { return g.Seafarers.Islands[id] == home }
	candidates := []riverSeaCandidate{}
	for _, path := range paths {
		all := append(g.riverSeaCandidates(len(path), allowed, false), g.riverSeaCandidates(len(path), allowed, true)...)
		found := false
		for _, c := range all {
			if slices.Equal(path, c.tiles) {
				candidates = append(candidates, c)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("部落河流无法通海")
		}
	}
	m := g.riverWorldMapFor(candidates)
	// Preserve the printed token inventory: swamp tokens are relocated as extra
	// discs on the weakest pasture/field, using the existing combination rule.
	removed := []int{}
	for i, c := range candidates {
		for j, id := range c.tiles {
			r := riverWorldChannels(6)[i][j]
			if r == catanSwamp {
				removed = append(removed, g.Tiles[id].Number)
				g.Tiles[id].Number = 0
			}
			g.Tiles[id].Resource = r
		}
	}
	for _, number := range removed {
		sites := g.riverTribeNumberSites(m.ExtraNumbers)
		if len(sites) == 0 {
			return nil, errors.New("部落河流缺少数字位置")
		}
		m.ExtraNumbers = append(m.ExtraNumbers, catanFishingExtraNumber{Tile: sites[0], Number: number})
	}
	return m, nil
}
func newCatanRiversTribeExtended(n int) (*State, error) {
	if n < 5 || n > 6 {
		return nil, errors.New("扩大河流部落需要五或六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := g.makeRiversTribeExtendedMap()
	if err != nil {
		return nil, err
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, SeaLayout: catanRiverTribeExtended, Map: m, Gold: make([]int, n), Bank: 152}
	g.Robber = -1
	s.Phase = "catan_rivers_start"
	s.Log = append(s.Log, "本站五六人河流部落：固定主岛替换三条河，河口可朝外框；沼泽数字依次叠放在最弱牧场或麦田，同概率按地图编号选择。保留全部部落奖励、34张发展卡、152金币、13分与配对回合", "本站移船规则：河岸造船或移入领1金币，移出先退1金币；桥位不能造普通道路或船")
	s.catanScores()
	return s, g.validateRivers()
}
func (g *Catan) validateRiversTribeExtended() error {
	n := len(g.Players)
	sea, r := g.Seafarers, g.Rivers
	if n < 5 || n > 6 || sea == nil || r == nil || sea.Tribe == nil || sea.Scenario != "tribe" || sea.Layout != "fixed" || sea.Variable || sea.Rules != CatanSeafarersRules || sea.NumberRecipe != "" || r.SeaLayout != catanRiverTribeExtended || sea.VictoryPoints != 13 || sea.IslandBonus != 0 || len(sea.Seats) != n || sea.Fog != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil {
		return errors.New("扩大河流部落配置无效")
	}
	expected, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	b := expected.Catan
	m, err := b.makeRiversTribeExtendedMap()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, r.Map) || !reflect.DeepEqual(g.Tiles, b.Tiles) || !slices.Equal(sea.Islands, b.Seafarers.Islands) || !slices.Equal(sea.StartIslands, b.Seafarers.StartIslands) || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) {
		return errors.New("扩大河流部落地图无效")
	}
	for i, v := range g.Vertices {
		w := b.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("部落交点无效")
		}
	}
	for i, e := range g.Edges {
		w := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("部落路线无效")
		}
	}
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("部落海盗位置无效")
	}
	return g.validateRiverTribeRewards(b)
}
