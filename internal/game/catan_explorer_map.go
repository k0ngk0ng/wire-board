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
	Tile      int    `json:"tile"`
	Region    int    `json:"region"` // 0 parrot, 1 goose.
	Resource  int    `json:"resource"`
	Number    int    `json:"number"` // Drawn on discovery, not secretly paired at setup.
	Revealed  bool   `json:"revealed"`
	Fish      int    `json:"fish,omitempty"` // Printed die face of a shoal, secret until discovery.
	Farm      string `json:"farm,omitempty"` // swift, pirate, gold; hidden until discovery.
	PirateDie int    `json:"pirateDie,omitempty"`
}
type catanExplorerCouncil struct {
	Tile    int   `json:"tile"`
	Anchors []int `json:"anchors"`
}
type catanExplorerShoal struct {
	Tile   int `json:"tile"`
	Number int `json:"number"`
}
type catanExplorerFarm struct {
	Tile      int    `json:"tile"`
	Ability   string `json:"ability"`
	PirateDie int    `json:"pirateDie,omitempty"`
}
type catanExplorerBoard struct {
	CitiesKnights bool                   `json:"citiesKnights,omitempty"`
	Council       *catanExplorerCouncil  `json:"council,omitempty"`
	Liberated     map[int]int            `json:"liberated,omitempty"`   // Original revealed lair numbers, supplied only by the mission controller.
	NumberSwaps   map[int]int            `json:"numberSwaps,omitempty"` // Current numbers after Invention; original regional/token provenance stays intact.
	Rules         string                 `json:"rules"`
	Scenario      string                 `json:"scenario"`
	Layout        string                 `json:"layout"`
	Players       int                    `json:"players"`
	Target        int                    `json:"target"`
	Starting      []int                  `json:"starting"`
	FramePasture  int                    `json:"framePasture"`
	FrameSea      int                    `json:"frameSea"`
	HarborStarts  []int                  `json:"harborStarts"`
	Regions       [2][]int               `json:"regions"`
	Opening       []catanExplorerOpening `json:"opening,omitempty"` // Four printed colors, not placed players.
	Hidden        []catanExplorerHidden  `json:"hidden"`
	Numbers       [2][]int               `json:"numbers"`
}

type catanExplorerMapRecipe struct {
	width     int
	parrot    [][]int // Northern rows; goose is the reflected shape.
	starting  []int   // Number of starting-island hexes in each row.
	resources []int   // Starting slots, including the fixed frame pasture.
	numbers   []int
	target    int
}

// Mission Guide 2025 pp4/8/16, Rulebook pp6–7 and 5–6 Rulebook pp4–7.
// Tables count loose hexes:
// include the printed frame's 6-pasture and opposite sea in playable geometry.
func catanExplorerRecipe(scenario string, players int) (catanExplorerMapRecipe, error) {
	if players > 4 {
		return catanExplorerSixRecipe(scenario)
	}
	r := catanExplorerMapRecipe{starting: []int{2, 2, 2, 3, 2, 2, 2}, numbers: []int{11, 9, 3, 8, 4, 10, 6, 12, 8, 10, 4, 11, 6, 3, 5}}
	switch scenario {
	case "land-ho":
		r.width, r.target = 6, 8
		r.parrot = [][]int{{4, 5}, {4, 5, 6}, {4, 6, 7}}
		r.resources = []int{4, 0, 3, 2, 1, 2, 2, 4, 0, 3, 2, 0, 1, 4, 0}
	case "pirate-lairs", "fish-for-catan":
		r.width, r.target = 7, 12
		r.parrot = [][]int{{4, 5, 6}, {4, 5, 6, 7}, {4, 5, 7, 8}}
		r.resources = []int{2, 0, 3, 2, 1, 4, 2, 4, 0, 3, 2, 0, 4, 1, 0}
		if scenario == "fish-for-catan" {
			r.target = 15
		}
	case "spices-for-catan":
		r.width, r.target = 8, 15
		r.parrot = [][]int{{4, 5, 6, 7}, {4, 5, 6, 7, 8}, {4, 5, 7, 9}}
		r.resources = []int{2, 0, 3, 2, 1, 4, 2, 4, 0, 3, 2, 0, 4, 1, 0}
	case "explorers-and-pirates":
		r.width, r.target = 9, 17
		r.parrot = [][]int{{4, 5, 6, 7, 8}, {4, 5, 6, 7, 8, 9}, {4, 5, 7, 9, 10}}
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

func catanExplorerScenarioResources(region int, scenario string) []int {
	if catanExplorerSpiceScenario(scenario) {
		// Mission guide p16: six ordinary land, one sea, three shoals,
		// three non-producing farms. The full scenario adds three gold fields.
		resources := append(catanExplorerRegionResources(region, false), CatanSea, CatanSea, CatanDesert, CatanDesert, CatanDesert)
		if scenario == "explorers-and-pirates" {
			// Full rulebook pp6–7: sixteen of each region's seventeen
			// hexes; remove one ordinary sea, keep all three gold fields.
			resources = append(resources, CatanGold, CatanGold, CatanGold)
		}
		return resources
	}
	resources := catanExplorerRegionResources(region, scenario != "land-ho")
	// Mission guide p14: two randomly chosen parrot shoals, all three goose
	// shoals. In the goose region one gold field is left in the box.
	if scenario == "fish-for-catan" && region == 1 {
		resources[len(resources)-1] = CatanSea
	}
	return resources
}

func catanExplorerGeometry(players int, scenario, layout string) (*Catan, *catanExplorerBoard, error) {
	return catanExplorerGeometryVariant(players, scenario, layout, false)
}

func catanExplorerGeometryVariant(players int, scenario, layout string, citiesKnights bool) (*Catan, *catanExplorerBoard, error) {
	if citiesKnights && (players < 3 || layout != "variable" || scenario == "land-ho") {
		return nil, nil, errors.New("探险家与城市骑士组合当前仅接入三至六人任务随机地图")
	}
	if players < 2 || players > 6 || players > 4 && layout != "variable" || layout != "fixed" && layout != "variable" || scenario == "land-ho" && layout != "fixed" || catanExplorerFishScenario(scenario) && layout != "variable" {
		return nil, nil, errors.New("探险地图需要2至6人；初航仅2至4人固定布局，五六人使用任务随机布局")
	}
	r, err := catanExplorerRecipe(scenario, players)
	if err != nil {
		return nil, nil, err
	}
	if citiesKnights {
		// Official combination: return one field and one forest instead of two
		// fields. Apply before shuffling; never alter a revealed terrain later.
		r.resources = slices.Clone(r.resources)
		i := slices.Index(r.resources, 0)
		if i < 0 {
			return nil, nil, errors.New("组合地图缺少可替换森林")
		}
		r.resources[i] = 3
		r.target += 5
	}
	m := &catanExplorerBoard{CitiesKnights: citiesKnights, Rules: catanExplorerRules, Scenario: scenario, Layout: layout, Players: players, Target: r.target, Starting: []int{}, HarborStarts: []int{}, Regions: [2][]int{{}, {}}}
	specs := []CatanHexSpec{}
	rows := make([][]int, len(r.starting))
	middle, last := len(rows)/2, len(rows)-1
	for row := range rows {
		count, start := r.width+min(row, last-row), -min(row, middle)
		for col := 0; col < count; col++ {
			id := len(specs)
			rows[row] = append(rows[row], id)
			resource, number := CatanSea, 0
			if col < r.starting[row] {
				i := len(m.Starting)
				resource, number = r.resources[i], r.numbers[i]
				m.Starting = append(m.Starting, id)
			}
			if row == middle && col == 0 {
				m.FramePasture = id
			}
			if row == middle && col == count-1 {
				m.FrameSea = id
			}
			for region := 0; region < 2; region++ {
				sourceRow := row
				if region == 1 {
					sourceRow = last - row
				}
				if sourceRow >= 0 && sourceRow < len(r.parrot) && slices.Contains(r.parrot[sourceRow], col) {
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
	if catanExplorerFishScenario(scenario) {
		// The English mission overview p14 omits the island artwork. Its
		// position is explicit in the English setup p6 and German scenario p18.
		tile := rows[middle][r.starting[middle]]
		m.Council = &catanExplorerCouncil{Tile: tile, Anchors: []int{g.Tiles[tile].Vertices[4], g.Tiles[tile].Vertices[1]}}
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
	return newCatanExplorerBoardVariant(players, scenario, layout, false)
}

func newCatanExplorerBoardVariant(players int, scenario, layout string, citiesKnights bool) (*Catan, *catanExplorerBoard, error) {
	g, m, err := catanExplorerGeometryVariant(players, scenario, layout, citiesKnights)
	if err != nil {
		return nil, nil, err
	}
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
		resources := m.regionResources(region)
		shuffle(resources)
		faces := []int{1 + 3*region, 2 + 3*region, 3 + 3*region}
		if catanExplorerSpiceScenario(scenario) {
			faces = append(faces, 0)
		}
		if catanExplorerFishScenario(scenario) {
			shuffle(faces)
		}
		farms := []string{"swift", "pirate", "gold"}
		if catanExplorerSpiceScenario(scenario) {
			shuffle(farms)
		}
		for i, tile := range tiles {
			h := catanExplorerHidden{Tile: tile, Region: region, Resource: resources[i]}
			if scenario == "fish-for-catan" && h.Resource == CatanSea {
				h.Fish, faces = faces[0], faces[1:]
			}
			if catanExplorerSpiceScenario(scenario) && h.Resource == CatanSea {
				// Four independently shuffled sea slots: one ordinary sea and
				// the region's three printed fish shoals.
				h.Fish, faces = faces[0], faces[1:]
			}
			if catanExplorerSpiceScenario(scenario) && h.Resource == CatanDesert {
				h.Farm, farms = farms[0], farms[1:]
				// German 2025 rulebook p20 shows both regional component
				// sets explicitly: parrot bonus 5, goose bonus 4.
				if h.Farm == "pirate" {
					h.PirateDie = 5 - region
				}
			}
			m.Hidden = append(m.Hidden, h)
		}
		m.Numbers[region] = m.regionNumbers(region)
		shuffle(m.Numbers[region])
	}
	return g, m, m.validate(g)
}

func (m catanExplorerBoard) validate(g *Catan) error {
	if g == nil || len(g.Players) != m.Players || m.Rules != catanExplorerRules {
		return errors.New("探险地图规则或人数无效")
	}
	base, spec, err := catanExplorerGeometryVariant(m.Players, m.Scenario, m.Layout, m.CitiesKnights)
	if err != nil {
		return err
	}
	if m.Target != spec.Target || m.FramePasture != spec.FramePasture || m.FrameSea != spec.FrameSea || !slices.Equal(m.Starting, spec.Starting) || !slices.Equal(m.HarborStarts, spec.HarborStarts) || len(m.Opening) != len(spec.Opening) || len(m.Hidden) != len(spec.Regions[0])+len(spec.Regions[1]) || len(g.Tiles) != len(base.Tiles) || len(g.Vertices) != len(base.Vertices) || len(g.Edges) != len(base.Edges) || len(g.Ports) != 0 || !catanExplorerCoordinate(g.HexSize, base.HexSize) {
		return errors.New("探险地图形状、标记或开局位置不符")
	}
	if (m.Council == nil) != (spec.Council == nil) || m.Council != nil && (m.Council.Tile != spec.Council.Tile || !slices.Equal(m.Council.Anchors, spec.Council.Anchors)) {
		return errors.New("议会岛或交付锚点与地图不符")
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
		if tile.Number != m.numberAt(i, b.Number) || tile.Resource != b.Resource && (m.Layout == "fixed" || !slices.Contains(m.Starting, i) || i == m.FramePasture) {
			return errors.New("探险起始数字或固定边框地块改变")
		}
		if slices.Contains(m.Starting, i) {
			land, expected = append(land, tile.Resource), append(expected, b.Resource)
		}
	}
	if !catanExplorerSameInventory(land, expected) {
		return errors.New("探险起始岛地形数量不守恒")
	}
	seen := map[int]bool{}
	shoals := map[int]bool{}
	farms := [2][]string{}
	pirateDice := []int{}
	resources, numbers := [2][]int{}, [2][]int{slices.Clone(m.Numbers[0]), slices.Clone(m.Numbers[1])}
	for tile, number := range m.Liberated {
		if !slices.ContainsFunc(m.Hidden, func(h catanExplorerHidden) bool { return h.Tile == tile && h.Revealed && h.Resource == CatanGold }) || number < 2 || number > 12 || number == 7 {
			return errors.New("已解放金矿数字或地块无效")
		}
	}
	for _, h := range m.Hidden {
		if h.Region < 0 || h.Region > 1 || !slices.Contains(m.Regions[h.Region], h.Tile) || seen[h.Tile] {
			return errors.New("探险隐藏地块位置、区域或唯一性损坏")
		}
		seen[h.Tile] = true
		if (m.Scenario == "fish-for-catan" || catanExplorerSpiceScenario(m.Scenario) && h.Fish != 0) && h.Resource == CatanSea {
			if h.Fish < 1+3*h.Region || h.Fish > 3+3*h.Region || shoals[h.Fish] {
				return errors.New("渔场骰面、背面区域或组件唯一性不符")
			}
			shoals[h.Fish] = true
		} else if h.Fish != 0 {
			return errors.New("普通地形不能带有渔场骰面")
		}
		if catanExplorerSpiceScenario(m.Scenario) && h.Resource == CatanDesert {
			if !slices.Contains([]string{"swift", "pirate", "gold"}, h.Farm) || h.Farm == "pirate" && h.PirateDie != 5-h.Region || h.Farm != "pirate" && h.PirateDie != 0 {
				return errors.New("香料农场能力或海盗奖励骰面无效")
			}
			farms[h.Region] = append(farms[h.Region], h.Farm)
			if h.Farm == "pirate" {
				pirateDice = append(pirateDice, h.PirateDie)
			}
		} else if h.Farm != "" || h.PirateDie != 0 {
			return errors.New("普通地形不能带有香料农场能力")
		}
		resources[h.Region] = append(resources[h.Region], h.Resource)
		tile := g.Tiles[h.Tile]
		if !h.Revealed {
			if h.Number != 0 || tile.Resource != CatanFog || tile.Number != 0 {
				return errors.New("未探索地块不能已有公开地形或数字")
			}
		} else {
			wantNumber := h.Number
			if h.Resource == CatanGold {
				wantNumber = m.Liberated[h.Tile]
			}
			if tile.Resource != h.Resource || tile.Number != m.numberAt(h.Tile, wantNumber) || h.Resource >= CatanDesert && h.Number != 0 || h.Resource < CatanDesert && h.Number == 0 {
				return errors.New("探索地块与已取数字不一致")
			}
			if h.Resource < CatanDesert {
				numbers[h.Region] = append(numbers[h.Region], h.Number)
			}
		}
	}
	for region := range m.Regions {
		if !catanExplorerSameInventory(resources[region], m.regionResources(region)) || !catanExplorerSameInventory(numbers[region], m.regionNumbers(region)) {
			return errors.New("探险地区地块或数字库存不守恒")
		}
		if catanExplorerSpiceScenario(m.Scenario) {
			slices.Sort(farms[region])
			if !slices.Equal(farms[region], []string{"gold", "pirate", "swift"}) || !shoals[region*3+1] || !shoals[region*3+2] || !shoals[region*3+3] {
				return errors.New("各区域须有三种不同农场与三个渔场")
			}
		}
	}
	if catanExplorerSpiceScenario(m.Scenario) && !catanExplorerSameInventory(pirateDice, []int{4, 5}) {
		return errors.New("海盗奖励农场组件不守恒")
	}
	return m.validateNumberSwaps(g, base)
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
	CitiesKnights bool                   `json:"citiesKnights,omitempty"`
	Farms         []catanExplorerFarm    `json:"farms,omitempty"`
	Council       *catanExplorerCouncil  `json:"council,omitempty"`
	Shoals        []catanExplorerShoal   `json:"shoals,omitempty"`
	Rules         string                 `json:"rules"`
	Scenario      string                 `json:"scenario"`
	Layout        string                 `json:"layout"`
	Target        int                    `json:"target"`
	Starting      []int                  `json:"starting"`
	HarborStarts  []int                  `json:"harborStarts"`
	Regions       [2][]int               `json:"regions"`
	Opening       []catanExplorerOpening `json:"opening,omitempty"`
	Unexplored    [2]int                 `json:"unexplored"`
	NumbersLeft   [2]int                 `json:"numbersLeft"`
}

func (m catanExplorerBoard) publicView() catanExplorerBoardView {
	v := catanExplorerBoardView{CitiesKnights: m.CitiesKnights, Rules: m.Rules, Scenario: m.Scenario, Layout: m.Layout, Target: m.Target, Starting: slices.Clone(m.Starting), HarborStarts: slices.Clone(m.HarborStarts), Regions: [2][]int{slices.Clone(m.Regions[0]), slices.Clone(m.Regions[1])}, Opening: slices.Clone(m.Opening)}
	v.Council = clone(m.Council)
	for i := range v.Opening {
		v.Opening[i].Resources = slices.Clone(v.Opening[i].Resources)
	}
	for _, h := range m.Hidden {
		if h.Revealed && h.Farm != "" {
			v.Farms = append(v.Farms, catanExplorerFarm{h.Tile, h.Farm, h.PirateDie})
		}
		if h.Revealed && h.Fish > 0 {
			v.Shoals = append(v.Shoals, catanExplorerShoal{Tile: h.Tile, Number: h.Fish})
		}
		if !h.Revealed {
			v.Unexplored[h.Region]++
		}
	}
	for region := range m.Numbers {
		v.NumbersLeft[region] = len(m.Numbers[region])
	}
	return v
}
