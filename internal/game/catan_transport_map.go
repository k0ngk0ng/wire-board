package game

import (
	"errors"
	"math"
	"slices"
)

const catanTransportRules = "catan-traders-barbarians-transport-2025"
const catanTransportTerrain = 13

type catanTransportSite struct {
	Tile    int    `json:"tile"`
	Kind    string `json:"kind"`
	Center  int    `json:"center"`
	Paths   []int  `json:"paths"`
	Blocked []int  `json:"blocked"`
}
type catanTransportMap struct {
	Rules      string               `json:"rules"`
	Sites      []catanTransportSite `json:"sites"`
	Barbarians [3]int               `json:"barbarians"`
	Gold       int                  `json:"gold"`
}

type catanTransportSiteSpec struct {
	tile    int
	kind    string
	blocked []int // Hex sides, not corner/vertex IDs.
}
type catanTransportRecipe struct {
	rows, starts, numbers, order []int
	resources                    [5]int
	deserts                      []int
	sites                        []catanTransportSiteSpec
	ports, barbarians            [][2]int
}

// Official 2025 T&B pp20–23 and 5–6 pp10–11. Extended numeric faces are
// printed on this map, so they do not depend on the unresolved A–Zc backs.
// The extended island is 37 hexes (4/5/6/7/6/5/4), NOT the base 30-hex board.
func catanTransportBoardRecipe(extended bool) catanTransportRecipe {
	if extended {
		return catanTransportRecipe{
			rows: []int{4, 5, 6, 7, 6, 5, 4}, starts: []int{0, -1, -2, -3, -3, -3, -3},
			resources: [5]int{6, 5, 6, 6, 5}, deserts: []int{5, 31},
			numbers:    []int{0, 6, 10, 0, 4, 0, 3, 12, 8, 3, 5, 10, 5, 9, 11, 0, 8, 12, 0, 2, 6, 0, 11, 9, 4, 9, 5, 3, 6, 2, 11, 0, 4, 0, 10, 8, 0},
			sites:      []catanTransportSiteSpec{{0, "quarry", []int{2, 3, 4}}, {3, "glassworks", nil}, {15, "glassworks", nil}, {18, "castle", nil}, {21, "quarry", nil}, {33, "quarry", nil}, {36, "glassworks", []int{0, 1, 5}}},
			ports:      [][2]int{{1, 3}, {3, 4}, {8, 5}, {27, 5}, {32, 0}, {35, 0}, {33, 1}, {28, 2}, {15, 2}, {9, 3}, {4, 3}},
			barbarians: [][2]int{{2, 0}, {16, 1}, {31, 5}},
		}
	}
	return catanTransportRecipe{
		rows: []int{3, 4, 5, 4, 3}, starts: []int{0, -1, -2, -2, -2}, resources: [5]int{4, 3, 3, 3, 3},
		sites:      []catanTransportSiteSpec{{2, "quarry", []int{3, 4, 5}}, {7, "castle", []int{1, 2, 3}}, {18, "glassworks", []int{0, 1, 5}}},
		order:      []int{2, 1, 0, 3, 7, 12, 16, 17, 18, 15, 11, 6, 5, 4, 8, 13, 14, 10, 9},
		numbers:    []int{5, 6, 3, 8, 10, 9, 11, 4, 8, 10, 9, 4, 5, 6, 3, 11},
		ports:      [][2]int{{0, 3}, {1, 4}, {6, 4}, {11, 5}, {15, 0}, {17, 0}, {16, 1}, {12, 2}, {3, 2}},
		barbarians: [][2]int{{1, 0}, {8, 1}, {14, 5}},
	}
}

// Construct geometry with non-producing placeholders, then mark commodity
// terrain locally. Do not expand the accepted terrain range of other scenarios.
func catanTransportGeometry(n int) (*Catan, *catanTransportMap, error) {
	if n < 2 || n > 6 {
		return nil, nil, errors.New("运输地图需要2至6位玩家")
	}
	r := catanTransportBoardRecipe(n > 4)
	specs := []CatanHexSpec{}
	for row, count := range r.rows {
		for col := range count {
			specs = append(specs, CatanHexSpec{Q: r.starts[row] + col, R: row, Resource: CatanDesert})
		}
	}
	g := &Catan{Players: make([]CatanPlayer, n), Options: CatanOptions{FiveSix: n > 4}}
	if err := g.makeScenarioMap(specs); err != nil {
		return nil, nil, err
	}
	g.Robber, g.LongestOwner, g.ArmyOwner = -1, -1, -1
	m := &catanTransportMap{Rules: catanTransportRules, Gold: 100}
	if n > 4 {
		m.Gold = 152
	}
	for _, spec := range r.sites {
		tile := &g.Tiles[spec.tile]
		tile.Resource = catanTransportTerrain
		center := len(g.Vertices)
		g.Vertices = append(g.Vertices, CatanVertex{ID: center, X: tile.X, Y: tile.Y, Owner: -1})
		site := catanTransportSite{Tile: spec.tile, Kind: spec.kind, Center: center, Paths: []int{}, Blocked: []int{}}
		for _, side := range spec.blocked {
			site.Blocked = append(site.Blocked, catanFishingSide(g, spec.tile, side))
		}
		for corner, vertex := range tile.Vertices {
			// A corner touching two crossed-out sides has no interior path.
			if slices.Contains(spec.blocked, corner) && slices.Contains(spec.blocked, (corner+5)%6) {
				continue
			}
			id := len(g.Edges)
			g.Edges = append(g.Edges, CatanEdge{ID: id, A: vertex, B: center, Tiles: []int{tile.ID}, Owner: -1})
			site.Paths = append(site.Paths, id)
		}
		m.Sites = append(m.Sites, site)
	}
	for i, pair := range r.barbarians {
		m.Barbarians[i] = catanFishingSide(g, pair[0], pair[1])
	}
	for _, pair := range r.ports {
		g.Ports = append(g.Ports, CatanPort{Edge: catanFishingSide(g, pair[0], pair[1]), Resource: -1})
	}
	return g, m, nil
}

// Internal map foundation, not a playable/public scenario constructor.
func newCatanTransportBoard(n int) (*Catan, *catanTransportMap, error) {
	g, m, err := catanTransportGeometry(n)
	if err != nil {
		return nil, nil, err
	}
	r := catanTransportBoardRecipe(n > 4)
	resources := []int{}
	for color, count := range r.resources {
		for range count {
			resources = append(resources, color)
		}
	}
	shuffle(resources)
	at := 0
	for i := range g.Tiles {
		if g.Tiles[i].Resource == catanTransportTerrain || slices.Contains(r.deserts, i) {
			continue
		}
		g.Tiles[i].Resource = resources[at]
		at++
	}
	if n > 4 {
		for i, number := range r.numbers {
			g.Tiles[i].Number = number
		}
	} else {
		at = 0
		for _, i := range r.order {
			if g.Tiles[i].Resource != catanTransportTerrain {
				g.Tiles[i].Number = r.numbers[at]
				at++
			}
		}
	}
	ports := []int{-1, -1, -1, -1, 0, 1, 2, 3, 4}
	if n > 4 {
		ports = append(ports, -1, 2)
	}
	shuffle(ports)
	for i, resource := range ports {
		g.Ports[i].Resource = resource
	}
	return g, m, m.validate(g)
}

func (m catanTransportMap) siteAt(vertex int) int {
	for i, site := range m.Sites {
		if site.Center == vertex {
			return i
		}
	}
	return -1
}

// X markers prohibit building roads (2025 p21). They do not prohibit wagon
// movement or placing a barbarian: those rules allow movement along edges.
func (m catanTransportMap) canBuildRoad(g *Catan, edge int) bool {
	if edge < 0 || edge >= len(g.Edges) {
		return false
	}
	for _, site := range m.Sites {
		if slices.Contains(site.Blocked, edge) {
			return false
		}
	}
	return true
}
func (m catanTransportMap) accepts(site int, cargo string) bool {
	if site < 0 || site >= len(m.Sites) {
		return false
	}
	switch m.Sites[site].Kind {
	case "quarry":
		return cargo == "tools"
	case "glassworks":
		return cargo == "sand"
	case "castle":
		return cargo == "marble" || cargo == "glass"
	}
	return false
}

func (m catanTransportMap) validate(g *Catan) error {
	if g == nil {
		return errors.New("运输地图缺失")
	}
	base, expected, err := catanTransportGeometry(len(g.Players))
	if err != nil {
		return err
	}
	if m.Rules != expected.Rules || m.Gold != expected.Gold || m.Barbarians != expected.Barbarians || g.HexSize != base.HexSize || len(m.Sites) != len(expected.Sites) || len(g.Tiles) != len(base.Tiles) || len(g.Vertices) != len(base.Vertices) || len(g.Edges) != len(base.Edges) || len(g.Ports) != len(base.Ports) {
		return errors.New("运输地图尺寸、版本或组件不符")
	}
	near := func(a, b float64) bool { return !math.IsNaN(a) && !math.IsInf(a, 0) && math.Abs(a-b) < .001 }
	for i, site := range m.Sites {
		want := expected.Sites[i]
		if site.Tile != want.Tile || site.Kind != want.Kind || site.Center != want.Center || !slices.Equal(site.Paths, want.Paths) || !slices.Equal(site.Blocked, want.Blocked) {
			return errors.New("货物地块中心、内路或禁行边不符")
		}
	}
	r := catanTransportBoardRecipe(len(g.Players) > 4)
	numbers := slices.Clone(r.numbers)
	if len(g.Players) <= 4 {
		numbers = make([]int, len(g.Tiles))
		at := 0
		for _, id := range r.order {
			if base.Tiles[id].Resource != catanTransportTerrain {
				numbers[id] = r.numbers[at]
				at++
			}
		}
	}
	counts := [5]int{}
	for id, tile := range g.Tiles {
		want := base.Tiles[id]
		if tile.ID != id || !near(tile.X, want.X) || !near(tile.Y, want.Y) || !slices.Equal(tile.Vertices, want.Vertices) || tile.Number != numbers[id] {
			return errors.New("运输地块坐标、拓扑或生产数字不符")
		}
		switch {
		case want.Resource == catanTransportTerrain:
			if tile.Resource != catanTransportTerrain {
				return errors.New("货物地块不可生产普通资源")
			}
		case slices.Contains(r.deserts, id):
			if tile.Resource != CatanDesert {
				return errors.New("运输沙漠位置不符")
			}
		default:
			if tile.Resource < 0 || tile.Resource >= 5 {
				return errors.New("运输普通地形无效")
			}
			counts[tile.Resource]++
		}
	}
	if counts != r.resources {
		return errors.New("运输资源地形库存不符")
	}
	for i, v := range g.Vertices {
		want := base.Vertices[i]
		if v.ID != i || !near(v.X, want.X) || !near(v.Y, want.Y) || v.Owner < -1 && (g.Two == nil || len(g.Players) != 2 || v.Owner < -3 || v.Level != 1) || v.Owner >= len(g.Players) || v.Level < 0 || v.Level > 2 || (v.Owner == -1) != (v.Level == 0) || m.siteAt(i) >= 0 && v.Level != 0 {
			return errors.New("运输交点无效，货物地块中心不可建设建筑")
		}
	}
	for i, e := range g.Edges {
		want := base.Edges[i]
		if e.ID != i || e.A != want.A || e.B != want.B || !slices.Equal(e.Tiles, want.Tiles) || e.Owner < -1 && (g.Two == nil || len(g.Players) != 2 || e.Owner < -3) || e.Owner >= len(g.Players) || e.Ship || e.Bridge || e.Warship || !m.canBuildRoad(g, i) && e.Owner != -1 {
			return errors.New("运输路线拓扑、种类或禁行边无效")
		}
	}
	portCounts := [6]int{}
	for i, p := range g.Ports {
		if p.Edge != base.Ports[i].Edge || p.Resource < -1 || p.Resource > 4 || !m.canBuildRoad(g, p.Edge) || len(g.Edges[p.Edge].Tiles) != 1 {
			return errors.New("运输港口位置或类型无效")
		}
		portCounts[p.Resource+1]++
	}
	wantPorts := [6]int{4, 1, 1, 1, 1, 1}
	if len(g.Players) > 4 {
		wantPorts = [6]int{5, 1, 1, 2, 1, 1}
	}
	if portCounts != wantPorts {
		return errors.New("运输港口库存不符")
	}
	return nil
}
