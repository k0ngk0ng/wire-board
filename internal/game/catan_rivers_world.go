package game

import (
	"errors"
	"reflect"
	"slices"
)

// Internal default recipe. Prepared maps, other counts, and public admission
// are separate work; ordinary New World generation is unchanged.
func newCatanRiversWorld(n int) (*State, error) {
	if n != 3 && n != 4 {
		return nil, errors.New("河流新世界暂支持三或四人")
	}
	s, err := NewCatanNewWorld(n, CatanOptions{})
	if err != nil {
		return nil, err
	}
	g := s.Catan
	long := g.riverSeaCandidates(4, func(int) bool { return true }, true)
	short := g.riverSeaCandidates(3, func(int) bool { return true }, true)
	shuffle(long)
	shuffle(short)
	var a, b riverSeaCandidate
	for _, x := range long {
		for _, y := range short {
			if g.riversSeparate(x.tiles, y.tiles) {
				a, b = x, y
				break
			}
		}
		if a.tiles != nil {
			break
		}
	}
	if a.tiles == nil {
		return nil, errors.New("新世界无法摆放河流")
	}
	m := g.riverTribeMapFor(a, b)
	terrain := []int{}
	// Remove hills2/pasture1/mountains2/sea2, then add the seven river hexes.
	for r, count := range []int{5, 2, 4, 5, 2, 0, 17, 0} {
		for range count {
			terrain = append(terrain, r)
		}
	}
	river := map[int]int{}
	for i, c := range m.Channels {
		rs := []int{4, 1, 2, catanSwamp}
		if i == 1 {
			rs = []int{4, 1, catanSwamp}
		}
		for j, id := range c.Tiles {
			river[id] = rs[j]
		}
	}
	ready := false
	for tries := 0; tries < 1000; tries++ {
		shuffle(terrain)
		at := 0
		for i := range g.Tiles {
			r, ok := river[i]
			if !ok {
				r = terrain[at]
				at++
			}
			g.Tiles[i].Resource, g.Tiles[i].Number = r, 0
		}
		numbers := []int{}
		for number, count := range []int{0, 0, 1, 3, 3, 3, 2, 0, 2, 3, 3, 2, 1} {
			for range count {
				numbers = append(numbers, number)
			}
		}
		if err = g.newWorldNumbers(numbers); err != nil {
			continue
		}
		// Each port removes at most three coast slots. This guarantees every
		// legal placement order can finish all ten ports without hidden planning.
		if len(g.worldPortEdges()) >= 28 {
			ready = true
			break
		}
		if tries == 999 {
			return nil, errors.New("新世界海岸不足以布置港口")
		}
	}
	if !ready {
		return nil, errors.New("未能生成合法河流新世界地图")
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, Map: m, Gold: make([]int, n), Bank: 100}
	g.Seafarers.Rules, g.Seafarers.Layout = CatanSeafarersRules, "prepared"
	g.Seafarers.Islands = g.findIslands()
	s.Log = []string{"河流＋新世界：随机摆河并让河口朝向海框，再随机分配其余地形；12分获胜", "先轮流放置10个港口，再开始建设；强盗和海盗从海框外出发", "本站移船补充：移入河岸领1金币，移出河岸须先退1金币；桥位不能造船"}
	s.catanScores()
	return s, g.validateRivers()
}

func (g *Catan) validateRiversWorldMap() error {
	sea, r := g.Seafarers, g.Rivers
	n := len(g.Players)
	if n < 3 || n > 4 || sea == nil || r == nil || r.Map == nil || sea.Rules != CatanSeafarersRules || sea.Scenario != "new_world" || sea.Layout != "prepared" || !sea.Variable || sea.NumberRecipe != "" || (r.SeaLayout != "" && r.SeaLayout != "prepared") || sea.NewWorld == nil || sea.Fog != nil || sea.Tribe != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil || sea.VictoryPoints != 12 || sea.IslandBonus != 1 || len(sea.Seats) != n || len(sea.StartIslands) != 0 {
		return errors.New("河流新世界配置无效")
	}
	specs := newWorldFrame(n)
	if len(g.Tiles) != len(specs) {
		return errors.New("河流新世界尺寸无效")
	}
	if err := validateRiverWorldInventory(g.Tiles, r.SeaLayout == "prepared"); err != nil {
		return err
	}
	for i, t := range g.Tiles {
		specs[i].Resource, specs[i].Number = t.Resource, t.Number
		if t.Resource == catanSwamp {
			specs[i].Resource = CatanDesert
		}
	}
	board := &Catan{}
	if err := board.makeScenarioMap(specs); err != nil {
		return err
	}
	for i := range board.Tiles {
		board.Tiles[i].Resource = g.Tiles[i].Resource
	}
	if !reflect.DeepEqual(g.Tiles, board.Tiles) || len(g.Edges) != len(board.Edges) || len(g.Vertices) != len(board.Vertices) {
		return errors.New("新世界地图几何无效")
	}
	for i, v := range g.Vertices {
		w := board.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("新世界交点无效")
		}
	}
	for i, e := range g.Edges {
		w := board.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Warship, e.Damaged = w.Owner, w.Ship, w.Bridge, w.Warship, w.Damaged
		if !reflect.DeepEqual(e, w) {
			return errors.New("新世界边无效")
		}
		if len(e.Tiles) == 2 {
			a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
			if (a == 6 || a == 8) && (b == 6 || b == 8) {
				return errors.New("新世界红点相邻")
			}
		}
	}
	if !slices.Equal(sea.Islands, g.findIslands()) || sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("新世界岛屿或海盗无效")
	}
	m := r.Map
	if len(m.Channels) != 2 {
		return errors.New("新世界河流数量无效")
	}
	candidates := make([]riverSeaCandidate, 2)
	for i, c := range m.Channels {
		found := false
		possible := g.riverSeaCandidates(4-i, func(int) bool { return true }, true)
		if r.SeaLayout == "prepared" {
			possible = g.riverWorldCandidates(4 - i)
		}
		for _, candidate := range possible {
			if candidate.outlet == c.Outlet && slices.Equal(candidate.tiles, c.Tiles) {
				candidates[i] = candidate
				found = true
				break
			}
		}
		if !found {
			return errors.New("新世界河流形状无效")
		}
		rs := []int{4, 1, 2, catanSwamp}
		if i == 1 {
			rs = []int{4, 1, catanSwamp}
		}
		for j, id := range c.Tiles {
			if g.Tiles[id].Resource != rs[j] {
				return errors.New("新世界河流地形无效")
			}
		}
	}
	if r.SeaLayout == "" && !g.riversSeparate(candidates[0].tiles, candidates[1].tiles) || !reflect.DeepEqual(m, g.riverTribeMapFor(candidates[0], candidates[1])) {
		return errors.New("新世界河流或桥位无效")
	}
	for _, id := range candidates[0].tiles {
		if slices.Contains(candidates[1].tiles, id) {
			return errors.New("两条河不能重叠")
		}
	}
	land := 0
	for _, v := range g.Vertices {
		if g.landVertex(v.ID) {
			land++
		}
	}
	if land < 8*n-3 {
		return errors.New("陆地交点不足以保证起始建村")
	}
	w := sea.NewWorld
	if len(w.Ports) != 10 || w.Index < 0 || w.Index > 10 || len(g.Ports) != w.Index {
		return errors.New("新世界港口进度无效")
	}
	ports := slices.Clone(w.Ports)
	slices.Sort(ports)
	if !slices.Equal(ports, []int{-1, -1, -1, -1, -1, 0, 1, 2, 3, 4}) {
		return errors.New("新世界港口库存无效")
	}
	coast := *g
	coast.Rivers = nil
	occupied := map[int]bool{}
	for i, p := range g.Ports {
		if p.Edge < 0 || p.Edge >= len(g.Edges) || p.Resource != w.Ports[i] {
			return errors.New("新世界已放港口无效")
		}
		e := g.Edges[p.Edge]
		if occupied[e.A] || occupied[e.B] || !coast.edgeTerrain(e.ID, true) || !coast.edgeTerrain(e.ID, false) {
			return errors.New("新世界港口位置无效")
		}
		occupied[e.A], occupied[e.B] = true, true
	}
	// Check the initial coastline, independent of how many ports are now placed.
	coast.Ports = nil
	fresh := *w
	fresh.Index = 0
	freshSea := *sea
	freshSea.NewWorld = &fresh
	coast.Seafarers = &freshSea
	if len(coast.worldPortEdges()) < 28 {
		return errors.New("新世界海岸无法保证港口布置完成")
	}
	return nil
}
