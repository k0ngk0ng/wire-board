package game

import (
	"errors"
	"math"
	"slices"
)

type catanExplorerOpening struct {
	Settlement int   `json:"settlement"`
	Harbor     int   `json:"harbor"`
	Road       int   `json:"road"`
	Ship       int   `json:"ship"`
	Resources  []int `json:"resources"`
}
type catanExplorerHidden struct {
	Tile     int  `json:"tile"`
	Region   int  `json:"region"` // 0 parrot, 1 goose.
	Resource int  `json:"resource"`
	Number   int  `json:"number"` // Drawn on discovery, not secretly paired at setup.
	Revealed bool `json:"revealed"`
}
type catanExplorerBoard struct {
	Rules        string                 `json:"rules"`
	Scenario     string                 `json:"scenario"`
	Layout       string                 `json:"layout"`
	Players      int                    `json:"players"`
	Target       int                    `json:"target"`
	Starting     []int                  `json:"starting"`
	FramePasture int                    `json:"framePasture"`
	FrameSea     int                    `json:"frameSea"`
	HarborStarts []int                  `json:"harborStarts"`
	Regions      [2][]int               `json:"regions"`
	Opening      []catanExplorerOpening `json:"opening,omitempty"` // Four printed colors, not placed players.
	Hidden       []catanExplorerHidden  `json:"hidden"`
	Numbers      [2][]int               `json:"numbers"`
}

type catanExplorerMapRecipe struct {
	width      int
	parrot     [3][]int // Columns in rows 0/1/2; goose is the reflected shape.
	resources  []int    // All 15 starting slots, including the fixed frame pasture.
	numbers    []int
	goldFields bool
	target     int
}

// Mission Guide 2025 pp4/8. The tables count loose hexes: include both the
// printed frame's 6-pasture and its opposite sea hex in playable geometry.
func catanExplorerRecipe(scenario string) (catanExplorerMapRecipe, error) {
	r := catanExplorerMapRecipe{numbers: []int{11, 9, 3, 8, 4, 10, 6, 12, 8, 10, 4, 11, 6, 3, 5}}
	switch scenario {
	case "land-ho":
		r.width, r.target = 6, 8
		r.parrot = [3][]int{{4, 5}, {4, 5, 6}, {4, 6, 7}}
		r.resources = []int{4, 0, 3, 2, 1, 2, 2, 4, 0, 3, 2, 0, 1, 4, 0}
	case "pirate-lairs":
		r.width, r.target, r.goldFields = 7, 12, true
		r.parrot = [3][]int{{4, 5, 6}, {4, 5, 6, 7}, {4, 5, 7, 8}}
		r.resources = []int{2, 0, 3, 2, 1, 4, 2, 4, 0, 3, 2, 0, 4, 1, 0}
	default:
		return r, errors.New("此探险家与海盗剧本地图尚未接入")
	}
	return r, nil
}
func catanExplorerRegionResources(region int, gold bool) []int {
	// Six ordinary terrain hexes in each region, with different duplicated
	// resources, plus two sea hexes; Pirate Lairs adds three gold fields each.
	resources := []int{0, 1, 2, 3, 4, 4, CatanSea, CatanSea}
	if region == 1 {
		resources = []int{0, 1, 2, 3, 3, 4, CatanSea, CatanSea}
	}
	if gold {
		resources = append(resources, CatanGold, CatanGold, CatanGold)
	}
	return resources
}
func catanExplorerRegionNumbers(region int) []int {
	if region == 0 {
		return []int{3, 4, 5, 6, 9, 10}
	}
	return []int{4, 5, 8, 9, 10, 11}
}

func catanExplorerGeometry(players int, scenario, layout string) (*Catan, *catanExplorerBoard, error) {
	if players < 2 || players > 4 || layout != "fixed" && layout != "variable" || scenario == "land-ho" && layout != "fixed" {
		return nil, nil, errors.New("此探险地图需要2至4人；初航使用固定布局")
	}
	r, err := catanExplorerRecipe(scenario)
	if err != nil {
		return nil, nil, err
	}
	m := &catanExplorerBoard{Rules: catanExplorerRules, Scenario: scenario, Layout: layout, Players: players, Target: r.target, Starting: []int{}, HarborStarts: []int{}, Regions: [2][]int{{}, {}}}
	specs := []CatanHexSpec{}
	rows := [7][]int{}
	for row := 0; row < 7; row++ {
		count, start := r.width+min(row, 6-row), -min(row, 3)
		for col := 0; col < count; col++ {
			id := len(specs)
			rows[row] = append(rows[row], id)
			resource, number := CatanSea, 0
			if col < 2 || row == 3 && col == 2 {
				i := len(m.Starting)
				resource, number = r.resources[i], r.numbers[i]
				m.Starting = append(m.Starting, id)
			}
			if row == 3 && col == 0 {
				m.FramePasture = id
			}
			if row == 3 && col == count-1 {
				m.FrameSea = id
			}
			for region := 0; region < 2; region++ {
				sourceRow := row
				if region == 1 {
					sourceRow = 6 - row
				}
				if sourceRow >= 0 && sourceRow < 3 && slices.Contains(r.parrot[sourceRow], col) {
					resource = CatanFog
					m.Regions[region] = append(m.Regions[region], id)
				}
			}
			specs = append(specs, CatanHexSpec{Q: start + col, R: row, Resource: resource, Number: number})
		}
	}
	g := &Catan{Players: make([]CatanPlayer, players), Robber: -1, LongestOwner: -1, ArmyOwner: -1}
	if err = g.makeScenarioMap(specs); err != nil {
		return nil, nil, err
	}
	// Green harbor symbols on p8 trace the eastern coast facing explicit sea.
	// Top/bottom outward frame coastline has no green symbol.
	for _, v := range g.Vertices {
		land, sea := false, false
		for _, tile := range g.Tiles {
			if !slices.Contains(tile.Vertices, v.ID) {
				continue
			}
			land = land || slices.Contains(m.Starting, tile.ID)
			sea = sea || tile.Resource == CatanSea
		}
		if land && sea {
			m.HarborStarts = append(m.HarborStarts, v.ID)
		}
	}
	if scenario == "land-ho" {
		// Printed order matches the shared blue/red/white/orange seat palette.
		type spot struct{ row, col, corner int }
		settlements := []spot{{0, 0, 1}, {6, 0, 4}, {3, 0, 0}, {2, 0, 2}}
		harbors := []spot{{6, 1, 4}, {0, 1, 1}, {3, 2, 1}, {2, 1, 0}}
		roads := []spot{{0, 0, 0}, {6, 0, 4}, {3, 1, 1}, {2, 0, 1}}
		ships := []spot{{6, 1, 4}, {0, 1, 0}, {3, 2, 0}, {3, 2, 4}}
		cards := [][]int{{0, 0, 1, 1, 1}, {1, 1, 0, 0, 1}, {0, 0, 1, 1, 1}, {0, 1, 1, 0, 0}}
		for p := range 4 {
			vertex := func(s spot) int { return g.Tiles[rows[s.row][s.col]].Vertices[s.corner] }
			edge := func(s spot) int { return catanFishingSide(g, rows[s.row][s.col], s.corner) }
			m.Opening = append(m.Opening, catanExplorerOpening{vertex(settlements[p]), vertex(harbors[p]), edge(roads[p]), edge(ships[p]), cards[p]})
		}
	}
	return g, m, nil
}

// Private board foundation only: does not place initial pieces, give resources,
// install missions, or expose an unfinished scenario to room creation.
func newCatanExplorerBoard(players int, scenario, layout string) (*Catan, *catanExplorerBoard, error) {
	g, m, err := catanExplorerGeometry(players, scenario, layout)
	if err != nil {
		return nil, nil, err
	}
	r, _ := catanExplorerRecipe(scenario)
	if layout == "variable" {
		resources := []int{}
		for _, tile := range m.Starting {
			if tile != m.FramePasture {
				resources = append(resources, g.Tiles[tile].Resource)
			}
		}
		shuffle(resources)
		for _, tile := range m.Starting {
			if tile != m.FramePasture {
				g.Tiles[tile].Resource, resources = resources[0], resources[1:]
			}
		}
	}
	for region, tiles := range m.Regions {
		resources := catanExplorerRegionResources(region, r.goldFields)
		shuffle(resources)
		for i, tile := range tiles {
			m.Hidden = append(m.Hidden, catanExplorerHidden{Tile: tile, Region: region, Resource: resources[i]})
		}
		m.Numbers[region] = catanExplorerRegionNumbers(region)
		shuffle(m.Numbers[region])
	}
	return g, m, m.validate(g)
}

func (m catanExplorerBoard) validate(g *Catan) error {
	if g == nil || len(g.Players) != m.Players || m.Rules != catanExplorerRules {
		return errors.New("探险地图规则或人数无效")
	}
	base, spec, err := catanExplorerGeometry(m.Players, m.Scenario, m.Layout)
	if err != nil {
		return err
	}
	if m.Target != spec.Target || m.FramePasture != spec.FramePasture || m.FrameSea != spec.FrameSea || !slices.Equal(m.Starting, spec.Starting) || !slices.Equal(m.HarborStarts, spec.HarborStarts) || len(m.Opening) != len(spec.Opening) || len(m.Hidden) != len(spec.Regions[0])+len(spec.Regions[1]) || len(g.Tiles) != len(base.Tiles) || len(g.Vertices) != len(base.Vertices) || len(g.Edges) != len(base.Edges) || len(g.Ports) != 0 || !catanExplorerCoordinate(g.HexSize, base.HexSize) {
		return errors.New("探险地图形状、标记或开局位置不符")
	}
	for region := range m.Regions {
		if !slices.Equal(m.Regions[region], spec.Regions[region]) {
			return errors.New("探险隐藏区域不符")
		}
	}
	for i, p := range m.Opening {
		q := spec.Opening[i]
		if p.Settlement != q.Settlement || p.Harbor != q.Harbor || p.Road != q.Road || p.Ship != q.Ship || !slices.Equal(p.Resources, q.Resources) {
			return errors.New("初航印刷开局棋子或资源不符")
		}
	}
	for i, edge := range g.Edges {
		b := base.Edges[i]
		if edge.ID != b.ID || edge.A != b.A || edge.B != b.B || !slices.Equal(edge.Tiles, b.Tiles) {
			return errors.New("探险海陆边拓扑损坏")
		}
	}
	for i, vertex := range g.Vertices {
		b := base.Vertices[i]
		if vertex.ID != b.ID || !catanExplorerCoordinate(vertex.X, b.X) || !catanExplorerCoordinate(vertex.Y, b.Y) {
			return errors.New("探险交点几何损坏")
		}
	}
	land, expected := []int{}, []int{}
	for i, tile := range g.Tiles {
		b := base.Tiles[i]
		if tile.ID != b.ID || !catanExplorerCoordinate(tile.X, b.X) || !catanExplorerCoordinate(tile.Y, b.Y) || !slices.Equal(tile.Vertices, b.Vertices) {
			return errors.New("探险地块几何损坏")
		}
		if b.Resource == CatanFog {
			continue
		}
		if tile.Number != b.Number || tile.Resource != b.Resource && (m.Layout == "fixed" || !slices.Contains(m.Starting, i) || i == m.FramePasture) {
			return errors.New("探险起始数字或固定边框地块改变")
		}
		if slices.Contains(m.Starting, i) {
			land, expected = append(land, tile.Resource), append(expected, b.Resource)
		}
	}
	if !catanExplorerSameInventory(land, expected) {
		return errors.New("探险起始岛地形数量不守恒")
	}
	r, _ := catanExplorerRecipe(m.Scenario)
	seen := map[int]bool{}
	resources, numbers := [2][]int{}, [2][]int{slices.Clone(m.Numbers[0]), slices.Clone(m.Numbers[1])}
	for _, h := range m.Hidden {
		if h.Region < 0 || h.Region > 1 || !slices.Contains(m.Regions[h.Region], h.Tile) || seen[h.Tile] {
			return errors.New("探险隐藏地块位置、区域或唯一性损坏")
		}
		seen[h.Tile] = true
		resources[h.Region] = append(resources[h.Region], h.Resource)
		tile := g.Tiles[h.Tile]
		if !h.Revealed {
			if h.Number != 0 || tile.Resource != CatanFog || tile.Number != 0 {
				return errors.New("未探索地块不能已有公开地形或数字")
			}
		} else {
			if tile.Resource != h.Resource || tile.Number != h.Number || h.Resource >= CatanDesert && h.Number != 0 || h.Resource < CatanDesert && h.Number == 0 {
				return errors.New("探索地块与已取数字不一致")
			}
			if h.Resource < CatanDesert {
				numbers[h.Region] = append(numbers[h.Region], h.Number)
			}
		}
	}
	for region := range m.Regions {
		if !catanExplorerSameInventory(resources[region], catanExplorerRegionResources(region, r.goldFields)) || !catanExplorerSameInventory(numbers[region], catanExplorerRegionNumbers(region)) {
			return errors.New("探险地区地块或数字库存不守恒")
		}
	}
	return nil
}
func catanExplorerCoordinate(a, b float64) bool {
	return !math.IsNaN(a) && !math.IsInf(a, 0) && math.Abs(a-b) < 1e-9
}
func catanExplorerSameInventory(a, b []int) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// Internal reveal primitive. The sailing/setup controller must verify the
// triggering ship and settle rewards/mission pieces in its atomic transaction.
// Gold-field number discs come from pirate lairs later, not ordinary stacks.
func (m *catanExplorerBoard) reveal(g *Catan, tile int) (catanExplorerHidden, error) {
	if err := m.validate(g); err != nil {
		return catanExplorerHidden{}, err
	}
	for i, hidden := range m.Hidden {
		if hidden.Tile != tile {
			continue
		}
		if hidden.Revealed {
			return catanExplorerHidden{}, errors.New("此地块已经探索，不能重复取数字或奖励")
		}
		if hidden.Resource < CatanDesert {
			if len(m.Numbers[hidden.Region]) == 0 {
				return catanExplorerHidden{}, errors.New("对应地区数字堆不足")
			}
			hidden.Number = m.Numbers[hidden.Region][0]
			m.Numbers[hidden.Region] = m.Numbers[hidden.Region][1:]
		}
		hidden.Revealed = true
		m.Hidden[i] = hidden
		g.Tiles[tile].Resource, g.Tiles[tile].Number = hidden.Resource, hidden.Number
		return hidden, nil
	}
	return catanExplorerHidden{}, errors.New("此地块不在待探索区域")
}

type catanExplorerBoardView struct {
	Rules        string                 `json:"rules"`
	Scenario     string                 `json:"scenario"`
	Layout       string                 `json:"layout"`
	Target       int                    `json:"target"`
	Starting     []int                  `json:"starting"`
	HarborStarts []int                  `json:"harborStarts"`
	Regions      [2][]int               `json:"regions"`
	Opening      []catanExplorerOpening `json:"opening,omitempty"`
	Unexplored   [2]int                 `json:"unexplored"`
	NumbersLeft  [2]int                 `json:"numbersLeft"`
}

func (m catanExplorerBoard) publicView() catanExplorerBoardView {
	v := catanExplorerBoardView{Rules: m.Rules, Scenario: m.Scenario, Layout: m.Layout, Target: m.Target, Starting: slices.Clone(m.Starting), HarborStarts: slices.Clone(m.HarborStarts), Regions: [2][]int{slices.Clone(m.Regions[0]), slices.Clone(m.Regions[1])}, Opening: slices.Clone(m.Opening)}
	for i := range v.Opening {
		v.Opening[i].Resources = slices.Clone(v.Opening[i].Resources)
	}
	for _, h := range m.Hidden {
		if !h.Revealed {
			v.Unexplored[h.Region]++
		}
	}
	for region := range m.Numbers {
		v.NumbersLeft[region] = len(m.Numbers[region])
	}
	return v
}
