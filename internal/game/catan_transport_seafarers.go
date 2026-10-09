package game

import (
	"errors"
	"reflect"
	"slices"
)

const CatanTransportSeafarersRules = "catan-transport-seafarers-2025"
const CatanTwoTransportSeaRules = "wire-board-two-transport-sea-v1"
const CatanTransportSeaExtendedLayout = "wire-board-transport-sea-5-6-v1"

func (g *Catan) transportSea() bool {
	return g != nil && g.Transport != nil && g.Transport.Map != nil && g.Transport.Map.Sea == CatanTransportSeafarersRules && g.Seafarers != nil
}
func (g *Catan) twoTransportSea() bool {
	return g.transportSea() && g.twoSeafarers() && g.Two.TransportSea == CatanTwoTransportSeaRules
}

// The official transport combination has just two applicable sea scenarios.
func NewCatanTransportSeafarers(n int, scenario string) (*State, error) {
	if n < 2 || n > 6 || scenario != "shores" && scenario != "desert" {
		return nil, errors.New("运输航海家支持二至六人的新海岸、穿越沙漠")
	}
	recipeSeats := n
	if n == 2 {
		recipeSeats = 4
	}
	b, m, err := newCatanTransportSeaBoard(recipeSeats, scenario)
	if err != nil {
		return nil, err
	}
	var s *State
	if n == 2 {
		s = &State{Kind: "catan", Round: 1}
		s.initCatan(2)
		s.Catan.Two = &CatanTwo{Rules: CatanTwoRules, Seafarers: CatanTwoSeafarersRules, TransportSea: CatanTwoTransportSeaRules, Rolls: []int{}, Tokens: []int{5, 5}, Bank: 10}
	} else {
		s, err = NewCatan(n, CatanOptions{FiveSix: n > 4})
		if err != nil {
			return nil, err
		}
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize, g.Seafarers = b.Tiles, b.Vertices, b.Edges, b.Ports, b.HexSize, b.Seafarers
	g.Seafarers.Seats = make([]CatanSeafarerSeat, n)
	g.Robber, g.LongestOwner = -1, -1
	g.Transport = makeCatanTransportPieces(g, m)
	if n == 2 {
		if err = g.prepareTwoSeaNeutrals(); err != nil {
			return nil, err
		}
		g.StartPlayer = catanRandom(2)
		s.Turn = g.StartPlayer
	}
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	for kind, count := range catanTransportDeckCounts(n) {
		for range count {
			g.DevDeck = append(g.DevDeck, kind)
		}
	}
	shuffle(g.DevDeck)
	if n > 4 {
		g.Transport.DeckRecipe = CatanTransportExtendedDeck
	}
	s.Log = []string{"运输＋航海家：先建村庄，再逆序建城市和马车；海边无船花2移动点，己方船1点，对方船1点并付1金币；无强盗、海盗或最长路线，17分获胜"}
	if n == 2 {
		s.Log = append(s.Log, "本站双人运输海图：使用四人组合图、中立海岸村庄和两次生产；中立船收费沿用累计一半银行、一半对手的运输规则")
	}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人运输海图：保留扩大海图，主岛沿海替换三处货物地，54枚货物、152金币及37张运输牌，配对行动；不称为官方扩大组合图")
	}
	s.catanScores()
	return s, s.validateCatanTransport()
}

// Commodity-center paths are additions to the ordinary sea graph. They are
// land routes, even though their edge has only one adjacent terrain hex.
func (g *Catan) transportInterior(edge int) bool {
	if !g.transportSea() || edge < 0 || edge >= len(g.Edges) {
		return false
	}
	e := g.Edges[edge]
	return g.Transport.Map.siteAt(e.A) >= 0 || g.Transport.Map.siteAt(e.B) >= 0
}

func installTransportSeaSites(g *Catan, m *catanTransportMap, specs []catanTransportSiteSpec) {
	for _, spec := range specs {
		t := &g.Tiles[spec.tile]
		t.Resource, t.Number = catanTransportTerrain, 0
		center := len(g.Vertices)
		g.Vertices = append(g.Vertices, CatanVertex{ID: center, X: t.X, Y: t.Y, Owner: -1})
		site := catanTransportSite{Tile: t.ID, Kind: spec.kind, Center: center, Paths: []int{}, Blocked: []int{}}
		for _, side := range spec.blocked {
			site.Blocked = append(site.Blocked, catanFishingSide(g, t.ID, side))
		}
		for corner, vertex := range t.Vertices {
			if slices.Contains(spec.blocked, corner) && slices.Contains(spec.blocked, (corner+5)%6) {
				continue
			}
			id := len(g.Edges)
			g.Edges = append(g.Edges, CatanEdge{ID: id, A: vertex, B: center, Tiles: []int{t.ID}, Owner: -1})
			site.Paths = append(site.Paths, id)
		}
		m.Sites = append(m.Sites, site)
	}
}

func newCatanTransportSeaBoard(n int, scenario string) (*Catan, *catanTransportMap, error) {
	if n < 3 || n > 6 || scenario != "shores" && scenario != "desert" {
		return nil, nil, errors.New("运输海图配方无效")
	}
	layout := "fixed"
	if scenario == "shores" && n > 4 {
		layout = "variable"
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: scenario, Layout: layout}, nil)
	if err != nil {
		return nil, nil, err
	}
	g := s.Catan
	if n > 4 && scenario == "shores" {
		// Fixed site recipe avoids deriving commodity supply from shuffled
		// terrain; base Seafarers keeps its original random mainland.
		for i, id := range riverShoresMainlandIDs() {
			g.Tiles[id].Resource = i % 5
			g.Tiles[id].Number = []int{5, 2, 6, 3, 8, 10, 9, 12, 11, 4}[i%10]
		}
		g.Seafarers.NumberRecipe = ""
		g.Seafarers.Variable = false
	}
	m := &catanTransportMap{Rules: catanTransportRules, Sea: CatanTransportSeafarersRules, Gold: 100}
	if n > 4 {
		m.Gold = 152
	}
	specs := []catanTransportSiteSpec{}
	if scenario == "shores" && n <= 4 {
		// Both official combination diagrams use the four-player sea frame.
		if n == 3 {
			if err = g.makeSeafarersShoresFour(); err != nil {
				return nil, nil, err
			}
		}
		base, _, e := newCatanTransportBoard(4)
		if e != nil {
			return nil, nil, e
		}
		ids := []int{12, 13, 14, 18, 19, 20, 21, 24, 25, 26, 27, 28, 31, 32, 33, 34, 37, 38, 39}
		for i, id := range ids {
			g.Tiles[id].Resource, g.Tiles[id].Number = base.Tiles[i].Resource, base.Tiles[i].Number
		}
		g.Ports = nil
		for _, p := range base.Ports {
			for _, tile := range base.Edges[p.Edge].Tiles {
				for side := 0; side < 6; side++ {
					if catanFishingSide(base, tile, side) == p.Edge {
						g.Ports = append(g.Ports, CatanPort{Edge: catanFishingSide(g, ids[tile], side), Resource: p.Resource})
					}
				}
			}
		}
		for i, p := range catanTransportBoardRecipe(false).barbarians {
			m.Barbarians[i] = catanFishingSide(g, ids[p[0]], p[1])
		}
		specs = []catanTransportSiteSpec{{14, "quarry", []int{3, 4, 5}}, {24, "castle", []int{1, 2, 3}}, {39, "glassworks", []int{0, 1, 5}}}
		g.Tiles[0].Resource, g.Tiles[0].Number = 4, 8
		g.Tiles[3].Resource, g.Tiles[3].Number = CatanSea, 0
		g.Tiles[10].Resource, g.Tiles[10].Number = 4, 2
		g.Tiles[23].Resource, g.Tiles[23].Number = CatanGold, 10
		m.ExtraNumbers = []catanFishingExtraNumber{{Tile: 10, Number: 12}}
		if n == 3 {
			terrain := map[int]seaTerrain{1: {2, 4}, 12: {2, 5}, 13: {0, 6}, 18: {3, 4}, 19: {1, 11}, 20: {5, 0}, 21: {2, 9}, 25: {0, 10}, 26: {5, 0}, 27: {4, 4}, 28: {0, 5}, 31: {4, 3}, 32: {5, 0}, 33: {1, 9}, 34: {0, 8}, 37: {5, 0}, 38: {3, 8}}
			for id, t := range terrain {
				g.Tiles[id].Resource, g.Tiles[id].Number = t.resource, t.number
			}
		}
	} else if scenario == "desert" && n <= 4 {
		if n == 3 {
			specs = []catanTransportSiteSpec{{4, "quarry", []int{1, 2, 3}}, {20, "glassworks", []int{0, 4, 5}}, {26, "castle", []int{1, 2, 3}}}
			g.Tiles[0].Resource = 2
			g.Tiles[6].Resource = 0
			m.Barbarians = [3]int{catanFishingSide(g, 5, 1), catanFishingSide(g, 21, 0), catanFishingSide(g, 25, 1)}
		} else {
			specs = []catanTransportSiteSpec{{5, "quarry", []int{1, 2, 3}}, {21, "glassworks", []int{0, 4, 5}}, {31, "castle", []int{1, 2, 3}}}
			g.Tiles[0].Resource = 2
			m.Barbarians = [3]int{catanFishingSide(g, 6, 1), catanFishingSide(g, 20, 0), catanFishingSide(g, 25, 1)}
		}
	} else {
		// Labelled site recipe: three widely separated mainland coastal sites.
		main := g.Seafarers.StartIslands[0]
		candidates := []int{}
		for _, tile := range g.Tiles {
			if g.Seafarers.Islands[tile.ID] != main || tile.Resource < 0 || tile.Resource >= 5 {
				continue
			}
			coast := false
			for side := 0; side < 6; side++ {
				edge := catanFishingSide(g, tile.ID, side)
				for _, id := range g.Edges[edge].Tiles {
					coast = coast || g.Tiles[id].Resource == CatanSea
				}
				coast = coast || len(g.Edges[edge].Tiles) == 1
			}
			if coast {
				candidates = append(candidates, tile.ID)
			}
		}
		chosen := []int{}
		for _, kind := range []string{"quarry", "castle", "glassworks"} {
			best, score := -1, -1.0
			for _, id := range candidates {
				if slices.Contains(chosen, id) {
					continue
				}
				value := 0.0
				if len(chosen) == 0 {
					value = -g.Tiles[id].Y + 10000
				} else {
					value = 1e9
					for _, other := range chosen {
						dx, dy := g.Tiles[id].X-g.Tiles[other].X, g.Tiles[id].Y-g.Tiles[other].Y
						value = min(value, dx*dx+dy*dy)
					}
				}
				if value > score {
					best, score = id, value
				}
			}
			if best < 0 {
				return nil, nil, errors.New("运输扩大海图缺少货物地点")
			}
			chosen = append(chosen, best)
			specs = append(specs, catanTransportSiteSpec{best, kind, nil})
		}
		at := 0
		for _, edge := range g.Edges {
			if at < 3 {
				land := false
				for _, id := range edge.Tiles {
					land = land || g.Seafarers.Islands[id] == main
				}
				if land {
					m.Barbarians[at] = edge.ID
					at++
				}
			}
		}
	}
	installTransportSeaSites(g, m, specs)
	g.Robber, g.LongestOwner, g.Seafarers.Pirate = -1, -1, -1
	g.Seafarers.Rules, g.Seafarers.Layout = CatanSeafarersRules, "fixed"
	if n > 4 {
		g.Seafarers.Layout, g.Seafarers.Variable = CatanTransportSeaExtendedLayout, false
	}
	g.Seafarers.VictoryPoints = 17
	return g, m, nil
}

func (m catanTransportMap) validateSea(g *Catan) error {
	if !g.transportSea() || m.Rules != catanTransportRules || m.Attack != "" || m.Caravans != "" || m.Rivers != "" || len(m.NumberSwaps) != 0 {
		return errors.New("运输海图组合标记无效")
	}
	n := len(g.Players)
	if n == 2 {
		n = 4
	}
	b, w, err := newCatanTransportSeaBoard(n, g.Seafarers.Scenario)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, *w) || g.HexSize != b.HexSize || len(g.Tiles) != len(b.Tiles) || len(g.Edges) != len(b.Edges) || len(g.Vertices) != len(b.Vertices) || len(g.Ports) != len(b.Ports) {
		return errors.New("运输海图地图或组件无效")
	}
	random := g.Seafarers.Scenario == "shores" && n == 4
	counts, want := map[seaTerrain]int{}, map[seaTerrain]int{}
	for i, t := range g.Tiles {
		ref := b.Tiles[i]
		if random && g.Seafarers.Islands[i] == g.Seafarers.StartIslands[0] && t.Resource != catanTransportTerrain {
			counts[seaTerrain{t.Resource, t.Number}]++
			want[seaTerrain{ref.Resource, ref.Number}]++
			// Terrain is shuffled independently of the fixed numbers.
			t.Resource = ref.Resource
		}
		if !reflect.DeepEqual(t, ref) {
			return errors.New("运输海图地块不匹配")
		}
	}
	if random {
		actual, expected := [5]int{}, [5]int{}
		for x, c := range counts {
			if x.resource >= 0 && x.resource < 5 {
				actual[x.resource] += c
			}
		}
		for x, c := range want {
			if x.resource >= 0 && x.resource < 5 {
				expected[x.resource] += c
			}
		}
		if actual != expected {
			return errors.New("运输海图地形库存不符")
		}
	}
	for i, v := range g.Vertices {
		ref := b.Vertices[i]
		v.Owner, v.Level = ref.Owner, ref.Level
		if !reflect.DeepEqual(v, ref) {
			return errors.New("运输海图交点无效")
		}
	}
	for i, e := range g.Edges {
		ref := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = ref.Owner, ref.Ship, ref.Bridge, ref.Damaged, ref.Warship
		if !reflect.DeepEqual(e, ref) {
			return errors.New("运输海图路线无效")
		}
	}
	a, z := []int{}, []int{}
	for i, p := range g.Ports {
		if p.Edge != b.Ports[i].Edge {
			return errors.New("运输海图港口位置无效")
		}
		a = append(a, p.Resource)
		z = append(z, b.Ports[i].Resource)
	}
	slices.Sort(a)
	slices.Sort(z)
	if !slices.Equal(a, z) {
		return errors.New("运输海图港口库存无效")
	}
	sea := g.Seafarers
	if sea.Rules != CatanSeafarersRules || sea.Layout != b.Seafarers.Layout || sea.Variable != b.Seafarers.Variable || sea.VictoryPoints != 17 || sea.Pirate != -1 || sea.Fog != nil || sea.Tribe != nil || sea.Cloth != nil || sea.PirateIslands != nil || sea.Wonders != nil || sea.NewWorld != nil || len(sea.Seats) != len(g.Players) || !slices.Equal(sea.StartIslands, b.Seafarers.StartIslands) || !slices.Equal(sea.Islands, b.Seafarers.Islands) {
		return errors.New("运输航海家状态无效")
	}
	return nil
}
