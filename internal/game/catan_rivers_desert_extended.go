package game

import (
	"errors"
	"reflect"
	"slices"
)

// These fixed site recipes preserve the outer islands and port locations.
// The ordinary five/six-player diagram has no published river overlay.
func (g *Catan) makeRiversDesertExtendedMap(layout string) (*catanRiversMap, error) {
	paths := [][]int{{20, 21, 22, 23}, {27, 37, 47}, {40, 41, 42}}
	if layout == "rivers-across" {
		paths = [][]int{{4, 13, 23, 33}, {20, 30, 40}, {27, 37, 47}}
	} else if layout != "desert-belt" {
		return nil, errors.New("河流沙漠布局无效")
	}
	allow := func(id int) bool { return g.Tiles[id].Resource != CatanSea && g.Tiles[id].Resource != CatanGold }
	candidates := []riverSeaCandidate{}
	for _, path := range paths {
		all := append(g.riverSeaCandidates(len(path), allow, false), g.riverSeaCandidates(len(path), allow, true)...)
		found := false
		for _, c := range all {
			if slices.Equal(c.tiles, path) {
				candidates = append(candidates, c)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("河流沙漠河口无效")
		}
	}
	// Keep printed numbers on productive river sites. Transfer numbers removed
	// by swamps to newly productive desert sites, discarding any surplus discs.
	removed := []int{}
	added := []int{}
	for i, c := range candidates {
		for j, id := range c.tiles {
			r := riverWorldChannels(6)[i][j]
			if r == catanSwamp {
				if g.Tiles[id].Number > 0 {
					removed = append(removed, g.Tiles[id].Number)
				}
				g.Tiles[id].Number = 0
			} else if g.Tiles[id].Number == 0 {
				added = append(added, id)
			}
			g.Tiles[id].Resource = r
		}
	}
	if len(added) > len(removed) {
		return nil, errors.New("河流沙漠数字库存不足")
	}
	for i, id := range added {
		g.Tiles[id].Number = removed[i]
	}
	g.Seafarers.Islands = g.findLandRegions(true)
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[40]}
	return g.riverWorldMapFor(candidates), nil
}
func newCatanRiversDesertExtended(n int, layout string) (*State, error) {
	if n < 5 || n > 6 {
		return nil, errors.New("扩大河流沙漠需要五或六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := g.makeRiversDesertExtendedMap(layout)
	if err != nil {
		return nil, err
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, SeaLayout: layout, Map: m, Gold: make([]int, n), Bank: 152}
	s.Log = append(s.Log, "本站五六人河流沙漠：三条河替换指定地形；保留外岛、港口及未替换地块数字，沼泽移出的数字依次用于被河流穿过的沙漠，余下数字移除。保留其余沙漠作为探索分界；14分获胜，152金币，配对回合", "本站移船规则：河岸造船或移入领1金币，移出先退1金币；桥位不能造普通道路或船")
	s.catanScores()
	return s, g.validateRivers()
}
func (g *Catan) validateRiversDesertExtended() error {
	n := len(g.Players)
	sea, r := g.Seafarers, g.Rivers
	if n < 5 || n > 6 || sea == nil || r == nil || sea.Scenario != "desert" || sea.Rules != CatanSeafarersRules || sea.Layout != "fixed" || sea.Variable || sea.NumberRecipe != "" || sea.VictoryPoints != 14 || sea.IslandBonus != 2 || len(sea.Seats) != n || sea.Fog != nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil {
		return errors.New("扩大河流沙漠配置无效")
	}
	expected, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	b := expected.Catan
	m, err := b.makeRiversDesertExtendedMap(r.SeaLayout)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, r.Map) || !reflect.DeepEqual(g.Tiles, b.Tiles) || !slices.Equal(sea.Islands, b.Seafarers.Islands) || !slices.Equal(sea.StartIslands, b.Seafarers.StartIslands) || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) || len(g.Ports) != len(b.Ports) {
		return errors.New("扩大河流沙漠地图无效")
	}
	for i, v := range g.Vertices {
		w := b.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("河流沙漠交点无效")
		}
	}
	for i, e := range g.Edges {
		w := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("河流沙漠路线无效")
		}
	}
	got, want := []int{}, []int{}
	for i, p := range g.Ports {
		if p.Edge != b.Ports[i].Edge {
			return errors.New("河流沙漠港口无效")
		}
		got = append(got, p.Resource)
		want = append(want, b.Ports[i].Resource)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return errors.New("河流沙漠港口库存无效")
	}
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("河流沙漠海盗位置无效")
	}
	return nil
}
