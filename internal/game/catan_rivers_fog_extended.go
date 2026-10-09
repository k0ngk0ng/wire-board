package game

import (
	"errors"
	"reflect"
	"slices"
)

const catanRiversFogExtendedLayout = "wire-board-fog-three-rivers-v1"
const catanRiversFogExtendedNotice = "本站五六人河流迷雾：三条互不相接的河流，河口可流向外框海域；两块麦田替换为沼泽，起始数字移除一枚2和一枚12后按地图顺序配置；探索堆与11港口保持原配置，152金币，12分获胜，配对回合"

func newCatanRiversFogExtended(n int) (*State, error) {
	if n < 5 || n > 6 {
		return nil, errors.New("五六人河流迷雾需要五或六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "fog", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := g.makeRiversFogExtendedMap()
	if err != nil {
		return nil, err
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, SeaLayout: catanRiversFogExtendedLayout, Map: m, Gold: make([]int, n), Bank: 152}
	g.Robber = -1
	s.Phase = "catan_rivers_start"
	s.Log = append(s.Log, catanRiversFogExtendedNotice, "先选择沼泽作为强盗起点；桥位不能造路或船；本站移船补充：河岸造船或移入领1金币，移出河岸须先退1金币；金矿仍选择资源而非金币")
	s.catanScores()
	return s, g.validateRivers()
}

// The visible terrain pool preserves every ordinary resource count except two
// fields replaced by swamps. Hidden terrain and numbers are never consulted.
func (g *Catan) makeRiversFogExtendedMap() (*catanRiversMap, error) {
	paths := [][]int{{13, 22, 31, 40}, {7, 16, 25}, {32, 41, 49}}
	allowed := func(id int) bool { return slices.Contains(g.Seafarers.Fog.StartTiles, id) }
	candidates := []riverSeaCandidate{}
	for _, path := range paths {
		all := append(g.riverSeaCandidates(len(path), allowed, false), g.riverSeaCandidates(len(path), allowed, true)...)
		found := false
		for _, c := range all {
			if slices.Equal(c.tiles, path) {
				candidates = append(candidates, c)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("五六人迷雾河道无法连接海域")
		}
	}
	m := g.riverWorldMapFor(candidates)
	terrain := make([]int, 11)
	numbers := []int{}
	for _, id := range g.Seafarers.Fog.StartTiles {
		t := g.Tiles[id]
		terrain[t.Resource]++
		numbers = append(numbers, t.Number)
	}
	terrain[3] -= 2
	terrain[catanSwamp] += 2
	for _, number := range []int{2, 12} {
		at := slices.Index(numbers, number)
		if at < 0 {
			return nil, errors.New("迷雾起始数字不足")
		}
		numbers = slices.Delete(numbers, at, at+1)
	}
	reserved := map[int]bool{}
	for i, c := range candidates {
		for j, id := range c.tiles {
			r := riverWorldChannels(6)[i][j]
			g.Tiles[id].Resource = r
			terrain[r]--
			reserved[id] = true
		}
	}
	for _, count := range terrain {
		if count < 0 {
			return nil, errors.New("迷雾河流地形库存不足")
		}
	}
	// Retain unaffected printed terrain where possible, then fill the remaining
	// sites in resource order. This makes the site's fixed recipe reproducible.
	pending := []int{}
	for _, id := range g.Seafarers.Fog.StartTiles {
		if reserved[id] {
			continue
		}
		r := g.Tiles[id].Resource
		if terrain[r] > 0 {
			terrain[r]--
		} else {
			pending = append(pending, id)
		}
	}
	at := 0
	for r, count := range terrain {
		for range count {
			if at >= len(pending) {
				return nil, errors.New("迷雾河流地形数量无效")
			}
			g.Tiles[pending[at]].Resource = r
			at++
		}
	}
	if at != len(pending) {
		return nil, errors.New("迷雾河流地形未填满")
	}
	at = 0
	for _, id := range g.Seafarers.Fog.StartTiles {
		g.Tiles[id].Number = 0
		if g.Tiles[id].Resource != catanSwamp {
			if at >= len(numbers) {
				return nil, errors.New("迷雾河流数字数量无效")
			}
			g.Tiles[id].Number = numbers[at]
			at++
		}
	}
	if at != len(numbers) {
		return nil, errors.New("迷雾河流数字未用完")
	}
	g.Seafarers.Islands = g.findIslands()
	return m, nil
}

func (g *Catan) validateRiversFogExtended() error {
	sea, r := g.Seafarers, g.Rivers
	n := len(g.Players)
	if n < 5 || n > 6 || sea == nil || r == nil || r.Map == nil || r.SeaLayout != catanRiversFogExtendedLayout || sea.Scenario != "fog" || sea.Rules != CatanSeafarersRules || sea.Layout != "fixed" || sea.Variable || sea.NumberRecipe != "" || sea.Fog == nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil || len(sea.Seats) != n || sea.VictoryPoints != g.riversSeaVictoryPoints(12) || sea.IslandBonus != 0 {
		return errors.New("五六人河流迷雾配置无效")
	}
	expected, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "fog", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	b := expected.Catan
	m, err := b.makeRiversFogExtendedMap()
	if err != nil {
		return err
	}
	if err = g.validateRiverFog(b); err != nil {
		return err
	}
	if !reflect.DeepEqual(r.Map, m) || !reflect.DeepEqual(g.Tiles, b.Tiles) || !reflect.DeepEqual(g.Ports, b.Ports) || !slices.Equal(sea.Islands, g.findIslands()) || !slices.Equal(sea.StartIslands, b.Seafarers.StartIslands) || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) {
		return errors.New("五六人河流迷雾地图无效")
	}
	for i, v := range g.Vertices {
		w := b.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("五六人河流迷雾交点无效")
		}
	}
	for i, e := range g.Edges {
		w := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("五六人河流迷雾路线无效")
		}
	}
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("五六人河流迷雾海盗位置无效")
	}
	return nil
}
