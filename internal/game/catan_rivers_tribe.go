package game

import (
	"errors"
	"math"
	"reflect"
	"slices"
)

func newCatanRiversTribe(n int) (*State, error) {
	if n != 3 && n != 4 {
		return nil, errors.New("河流部落暂支持三或四人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := g.makeRiversTribeMap()
	if err != nil {
		return nil, err
	}
	g.Rivers = &CatanRivers{Rules: CatanRiversRules, Sea: CatanRiversSeafarersRules, Map: m, Gold: make([]int, n), Bank: 100}
	g.Seafarers.Layout = "variable"
	g.Seafarers.Variable = true
	g.Robber = -1
	s.Phase = "catan_rivers_start"
	s.Log = []string{"河流＋遗忘部落：随机主岛，两条河互不相接且流入大海；13分获胜；强盗先选择沼泽", "2和12放在产出较弱的牧场或麦田；本站补充：同权重位置随机选择，第二枚按加入第一枚后的总概率计算", "本站移船补充：河岸造船或移入领1金币，移出河岸须先退1金币；桥位不能造船"}
	s.catanScores()
	return s, g.validateRivers()
}

type riverSeaCandidate struct {
	tiles  []int
	outlet int
}

func (g *Catan) riverTribeCandidates(length int) []riverSeaCandidate {
	home := g.Seafarers.StartIslands[0]
	return g.riverSeaCandidates(length, func(id int) bool { return g.Seafarers.Islands[id] == home }, false)
}

func (g *Catan) riverSeaCandidates(length int, allowed func(int) bool, frame bool) []riverSeaCandidate {
	result := []riverSeaCandidate{}
	for _, start := range g.Tiles {
		if !allowed(start.ID) {
			continue
		}
		for side := 0; side < 6; side++ {
			angle := float64(side+1) * math.Pi / 3
			dx, dy := math.Sqrt(3)*g.HexSize*math.Cos(angle), math.Sqrt(3)*g.HexSize*math.Sin(angle)
			ids := []int{start.ID}
			for step := 1; step < length; step++ {
				found := -1
				for _, t := range g.Tiles {
					if allowed(t.ID) && math.Abs(t.X-start.X-float64(step)*dx) < 0.01 && math.Abs(t.Y-start.Y-float64(step)*dy) < 0.01 {
						found = t.ID
						break
					}
				}
				if found < 0 {
					break
				}
				ids = append(ids, found)
			}
			if len(ids) != length {
				continue
			}
			outlet := catanFishingSide(g, ids[len(ids)-1], side)
			sea := false
			for _, tile := range g.Edges[outlet].Tiles {
				sea = sea || g.Tiles[tile].Resource == CatanSea
			}
			if sea && !frame || frame && len(g.Edges[outlet].Tiles) == 1 {
				result = append(result, riverSeaCandidate{ids, outlet})
			}
		}
	}
	return result
}
func (g *Catan) riversSeparate(a, b []int) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return false
			}
			for _, v := range g.Tiles[x].Vertices {
				if slices.Contains(g.Tiles[y].Vertices, v) {
					return false
				}
			}
		}
	}
	return true
}
func (g *Catan) riverTribeMapFor(a, b riverSeaCandidate) *catanRiversMap {
	m := &catanRiversMap{DoubleNumberTile: -1}
	for _, c := range []riverSeaCandidate{a, b} {
		m.Channels = append(m.Channels, catanRiverChannel{Tiles: slices.Clone(c.tiles), Outlet: c.outlet})
		m.Swamps = append(m.Swamps, c.tiles[len(c.tiles)-1])
		for i := 1; i < len(c.tiles); i++ {
			for _, e := range g.Edges {
				if slices.Contains(e.Tiles, c.tiles[i-1]) && slices.Contains(e.Tiles, c.tiles[i]) {
					m.Bridges = append(m.Bridges, e.ID)
					break
				}
			}
		}
		m.Bridges = append(m.Bridges, c.outlet)
	}
	return m
}
func (g *Catan) riverTribeNumberSites(extra []catanFishingExtraNumber) []int {
	result := []int{}
	best := 100
	for _, t := range g.Tiles {
		if g.Seafarers.Islands[t.ID] != g.Seafarers.StartIslands[0] || (t.Resource != 2 && t.Resource != 3) {
			continue
		}
		weight := 6 - absCatan(7-t.Number)
		for _, e := range extra {
			if e.Tile == t.ID {
				weight += 6 - absCatan(7-e.Number)
			}
		}
		if weight < best {
			best = weight
			result = nil
		}
		if weight == best {
			result = append(result, t.ID)
		}
	}
	return result
}
func (g *Catan) riverTribeNumbersValid() bool {
	for _, id := range []int{18, 26, 33} {
		n := g.Tiles[id].Number
		if n == 5 || n == 6 || n == 8 || n == 9 {
			return false
		}
	}
	for _, e := range g.Edges {
		if len(e.Tiles) != 2 {
			continue
		}
		a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
		if (a == 6 || a == 8) && (b == 6 || b == 8) {
			return false
		}
	}
	return true
}
func (g *Catan) makeRiversTribeMap() (*catanRiversMap, error) {
	long, short := g.riverTribeCandidates(4), g.riverTribeCandidates(3)
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
		return nil, errors.New("主岛无法摆放两条不接触的河流")
	}
	terrain, numbers := []int{}, []int{}
	for _, t := range g.Tiles {
		if g.Seafarers.Islands[t.ID] == g.Seafarers.StartIslands[0] {
			terrain = append(terrain, t.Resource)
			if t.Number != 2 && t.Number != 12 {
				numbers = append(numbers, t.Number)
			}
		}
	}
	for _, r := range []int{1, 1, 2, 2, 3, 4, 4} {
		i := slices.Index(terrain, r)
		if i < 0 {
			return nil, errors.New("部落河流替换组件不足")
		}
		terrain = slices.Delete(terrain, i, i+1)
	}
	shuffle(terrain)
	m := g.riverTribeMapFor(a, b)
	for i, c := range m.Channels {
		rs := []int{4, 1, 2, catanSwamp}
		if i == 1 {
			rs = []int{4, 1, catanSwamp}
		}
		for j, id := range c.Tiles {
			g.Tiles[id].Resource = rs[j]
			g.Tiles[id].Number = 0
		}
	}
	sites := []int{}
	at := 0
	for i, t := range g.Tiles {
		if g.Seafarers.Islands[t.ID] != g.Seafarers.StartIslands[0] {
			continue
		}
		if !slices.Contains(a.tiles, t.ID) && !slices.Contains(b.tiles, t.ID) {
			g.Tiles[i].Resource = terrain[at]
			at++
		}
		if g.Tiles[i].Resource != catanSwamp {
			sites = append(sites, i)
		}
	}
	if len(sites) != len(numbers) {
		return nil, errors.New("部落河流数字数量不符")
	}
	valid := false
	for tries := 0; tries < 10000; tries++ {
		shuffle(numbers)
		for i, id := range sites {
			g.Tiles[id].Number = numbers[i]
		}
		if g.riverTribeNumbersValid() {
			valid = true
			break
		}
	}
	if !valid {
		return nil, errors.New("未找到合法河流部落数字布局")
	}
	for _, n := range []int{2, 12} {
		candidates := g.riverTribeNumberSites(m.ExtraNumbers)
		if len(candidates) == 0 {
			return nil, errors.New("缺少额外圆片位置")
		}
		m.ExtraNumbers = append(m.ExtraNumbers, catanFishingExtraNumber{Tile: candidates[catanRandom(len(candidates))], Number: n})
	}
	return m, nil
}

func (g *Catan) validateRiversTribeMap() error {
	if len(g.Players) > 4 {
		return g.validateRiversTribeExtended()
	}
	sea, r := g.Seafarers, g.Rivers
	if sea == nil || r == nil || r.Map == nil || len(g.Players) < 2 || len(g.Players) == 2 && !g.twoRiversSea() || len(g.Players) > 4 || sea.Scenario != "tribe" || sea.Layout != "variable" || !sea.Variable || sea.Rules != CatanSeafarersRules || sea.NumberRecipe != "" || r.SeaLayout != "" || sea.Fog != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil || sea.Tribe == nil || len(sea.Seats) != len(g.Players) {
		return errors.New("河流部落配置无效")
	}
	recipeSeats := len(g.Players)
	if g.twoRiversSea() {
		recipeSeats = 4
	}
	expected, err := NewCatanSeafarers(recipeSeats, CatanOptions{}, CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	board := expected.Catan
	if len(g.Tiles) != len(board.Tiles) || len(g.Edges) != len(board.Edges) || len(g.Vertices) != len(board.Vertices) || !slices.Equal(sea.StartIslands, board.Seafarers.StartIslands) || !slices.Equal(sea.Islands, board.Seafarers.Islands) || sea.VictoryPoints != 13 || sea.IslandBonus != 0 {
		return errors.New("河流部落地图尺寸或岛屿无效")
	}
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("河流部落海盗位置无效")
	}
	for i, v := range g.Vertices {
		w := board.Vertices[i]
		v.Owner = w.Owner
		v.Level = w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("河流部落交点无效")
		}
	}
	for i, e := range g.Edges {
		w := board.Edges[i]
		e.Owner = w.Owner
		e.Ship = w.Ship
		e.Bridge = w.Bridge
		e.Damaged = w.Damaged
		e.Warship = w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("河流部落路线无效")
		}
	}
	m := r.Map
	if len(m.Channels) != 2 || m.DoubleNumberTile != -1 || m.NumberRecipe != "" || len(m.NumberSwaps) != 0 || len(m.ExtraNumbers) != 2 {
		return errors.New("河流部落河道或圆片无效")
	}
	for i, tile := range g.Tiles {
		want := board.Tiles[i]
		want.Resource, want.Number = tile.Resource, tile.Number
		if !reflect.DeepEqual(tile, want) {
			return errors.New("河流部落地块几何无效")
		}
	}
	if !slices.Equal(sea.Islands, g.findIslands()) {
		return errors.New("河流部落岛屿无效")
	}
	candidates := make([]riverSeaCandidate, 2)
	for i, c := range m.Channels {
		length := 4 - i
		found := false
		for _, candidate := range g.riverTribeCandidates(length) {
			if candidate.outlet == c.Outlet && slices.Equal(candidate.tiles, c.Tiles) {
				candidates[i] = candidate
				found = true
				break
			}
		}
		if !found {
			return errors.New("河流部落河道形状或河口无效")
		}
		rs := []int{4, 1, 2, catanSwamp}
		if i == 1 {
			rs = []int{4, 1, catanSwamp}
		}
		for j, id := range c.Tiles {
			if g.Tiles[id].Resource != rs[j] {
				return errors.New("河流部落地形顺序无效")
			}
		}
	}
	if !g.riversSeparate(candidates[0].tiles, candidates[1].tiles) {
		return errors.New("两条河不能接触")
	}
	want := g.riverTribeMapFor(candidates[0], candidates[1])
	want.ExtraNumbers = slices.Clone(m.ExtraNumbers)
	if !reflect.DeepEqual(want, m) {
		return errors.New("河流部落桥位或沼泽无效")
	}
	terrain, numbers := []int{}, []int{}
	for i, t := range g.Tiles {
		w := board.Tiles[i]
		if sea.Islands[i] == sea.StartIslands[0] {
			terrain = append(terrain, t.Resource)
			if t.Resource == catanSwamp {
				if t.Number != 0 {
					return errors.New("沼泽不能放数字")
				}
			} else {
				numbers = append(numbers, t.Number)
			}
			w.Resource = t.Resource
			w.Number = t.Number
		}
		if !reflect.DeepEqual(t, w) {
			return errors.New("河流部落地块被更改")
		}
	}
	// Main island: forest4, hills3, pasture3, fields3, mountains3, swamp2.
	slices.Sort(terrain)
	slices.Sort(numbers)
	if !slices.Equal(terrain, []int{0, 0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3, 4, 4, 4, catanSwamp, catanSwamp}) || !slices.Equal(numbers, []int{3, 3, 4, 4, 5, 5, 6, 6, 8, 8, 9, 9, 10, 10, 11, 11}) || !g.riverTribeNumbersValid() {
		return errors.New("河流部落资源或数字配方无效")
	}
	for i, e := range m.ExtraNumbers {
		number := 2
		if i == 1 {
			number = 12
		}
		if e.Number != number || !slices.Contains(g.riverTribeNumberSites(m.ExtraNumbers[:i]), e.Tile) {
			return errors.New("额外圆片没有放在最弱麦田或牧场")
		}
	}
	return g.validateRiverTribeRewards(board)
}

func (g *Catan) validateRiverTribeRewards(board *Catan) error {
	t, initial := g.tribe(), board.tribe()
	if len(t.Points) != len(g.Players) || len(t.HeldPorts) != len(g.Players) || t.ProgressRules != "" {
		return errors.New("河流部落奖励座位无效")
	}
	seen := map[int]bool{}
	points := len(t.Tokens)
	for _, edge := range t.Tokens {
		if !slices.Contains(initial.Tokens, edge) || seen[edge] {
			return errors.New("部落分数标记无效")
		}
		seen[edge] = true
	}
	for _, p := range t.Points {
		if p < 0 {
			return errors.New("部落分数无效")
		}
		points += p
	}
	if points != len(initial.Tokens) {
		return errors.New("部落分数不守恒")
	}
	cards := make([]int, 5)
	seen = map[int]bool{}
	for _, d := range t.Development {
		if d.Card < 0 || d.Card >= 5 || seen[d.Edge] || !slices.ContainsFunc(initial.Development, func(x CatanTribeDevelopment) bool { return x.Edge == d.Edge }) {
			return errors.New("部落发展卡奖励无效")
		}
		seen[d.Edge] = true
		cards[d.Card]++
	}
	for _, pile := range [][]int{g.DevDeck, g.DevDiscard, g.HelperExile} {
		for _, c := range pile {
			if c < 0 || c >= 5 {
				return errors.New("发展卡无效")
			}
			cards[c]++
		}
	}
	for _, p := range g.Players {
		if len(p.Dev) != 5 {
			return errors.New("发展卡手牌无效")
		}
		for i, c := range p.Dev {
			if c < 0 {
				return errors.New("负数发展卡")
			}
			cards[i] += c
		}
	}
	wantCards := []int{14, 2, 2, 2, 5}
	if len(g.Players) > 4 {
		wantCards = []int{20, 3, 3, 3, 5}
	}
	if !slices.Equal(cards, wantCards) {
		return errors.New("部落发展卡不守恒")
	}
	portCounts := make([]int, 6)
	seen = map[int]bool{}
	for _, p := range t.Ports {
		if p.Resource < -1 || p.Resource > 4 || seen[p.Edge] || !slices.ContainsFunc(initial.Ports, func(x CatanPort) bool { return x.Edge == p.Edge }) {
			return errors.New("部落待领取港口无效")
		}
		seen[p.Edge] = true
		portCounts[p.Resource+1]++
	}
	occupied := map[int]bool{}
	for _, p := range append(slices.Clone(t.Ports), g.Ports...) {
		if p.Edge < 0 || p.Edge >= len(g.Edges) || p.Resource < -1 || p.Resource > 4 {
			return errors.New("部落港口无效")
		}
		e := g.Edges[p.Edge]
		if occupied[e.A] || occupied[e.B] {
			return errors.New("部落港口相邻")
		}
		occupied[e.A] = true
		occupied[e.B] = true
	}
	for _, p := range g.Ports {
		// Ports may share bridge edges: ports are not route pieces.
		boardWithTerrain := *g
		boardWithTerrain.Rivers = nil
		if !boardWithTerrain.edgeTerrain(p.Edge, true) || !boardWithTerrain.edgeTerrain(p.Edge, false) {
			return errors.New("部落港口不在海岸")
		}
		portCounts[p.Resource+1]++
	}
	for _, held := range t.HeldPorts {
		for _, r := range held {
			if r < -1 || r > 4 {
				return errors.New("持有港口无效")
			}
			portCounts[r+1]++
		}
	}
	wantPorts := make([]int, 6)
	for _, p := range initial.Ports {
		wantPorts[p.Resource+1]++
	}
	if !slices.Equal(portCounts, wantPorts) {
		return errors.New("部落港口不守恒")
	}
	return nil
}
