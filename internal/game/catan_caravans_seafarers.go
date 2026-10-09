package game

import (
	"errors"
	"reflect"
	"slices"
)

const CatanCaravansSeafarersRules = "catan-caravans-seafarers-2025"

func (g *Catan) caravansSea() bool {
	return g.Caravans != nil && g.Caravans.Sea == CatanCaravansSeafarersRules && g.Seafarers != nil
}
func (g *Catan) makeCaravansDesertSea() (*catanCaravanMap, []catanFishingExtraNumber, error) {
	if g.Seafarers.Scenario == "tribe" && len(g.Players) > 4 {
		return g.makeCaravansTribeExtended()
	}
	number := 2
	if len(g.Players) == 4 {
		number = 11
	}
	// Locate the printed terrain/number pair, independent of display coordinates.
	sourceResource, targetResource, targetNumber := 3, 2, 12
	if len(g.Players) == 4 {
		sourceResource, targetResource, targetNumber = 1, 1, 2
	}
	if g.Seafarers.Scenario == "tribe" {
		number, sourceResource, targetResource, targetNumber = 12, 3, 2, 2
	}
	hole, recipient := -1, -1
	for _, t := range g.Tiles {
		if t.Resource == sourceResource && t.Number == number {
			hole = t.ID
		}
		if t.Resource == targetResource && t.Number == targetNumber {
			recipient = t.ID
		}
	}
	if hole < 0 || recipient < 0 {
		return nil, nil, errors.New("商队沙漠印刷地块缺失")
	}
	g.Tiles[hole].Resource, g.Tiles[hole].Number = catanWateringHole, 0
	starts, err := caravanStarts(g, []int{hole})
	if err != nil {
		return nil, nil, err
	}
	return &catanCaravanMap{WateringHoles: []int{hole}, Starts: starts, Supply: 22}, []catanFishingExtraNumber{{Tile: recipient, Number: number}}, nil
}
func newCatanCaravansDesertSea(n int) (*State, error) {
	if n != 3 && n != 4 {
		return nil, errors.New("商队沙漠内部配方暂支持三四人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, extra, err := g.makeCaravansDesertSea()
	if err != nil {
		return nil, err
	}
	g.Caravans = &catanCaravans{Sea: CatanCaravansSeafarersRules, Rules: CatanCaravansRules, Map: m, ExtraNumbers: extra, Wagons: []catanCaravanWagon{}}
	g.Seafarers.VictoryPoints += 2
	s.Log = append(s.Log, "商队＋穿越沙漠：按官方说明替换水源并叠放数字；马车可沿海格边延伸，不受海盗阻挡，与己方船同边时该船计两段最长路线；16分获胜")
	s.catanScores()
	return s, s.validateCaravans()
}
func (g *Catan) validateCaravansDesertSea() error {
	if g.Seafarers != nil && g.Seafarers.Scenario == "tribe" {
		return g.validateCaravansTribeSea()
	}
	n := len(g.Players)
	sea, c := g.Seafarers, g.Caravans
	if !g.caravansSea() || (n != 3 && n != 4) || sea.Scenario != "desert" || sea.Layout != "fixed" || sea.Rules != CatanSeafarersRules || sea.Variable || sea.NumberRecipe != "" || sea.VictoryPoints != 16 || sea.IslandBonus != 2 || len(sea.Seats) != n || sea.Fog != nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil {
		return errors.New("商队沙漠海图配置无效")
	}
	expected, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	b := expected.Catan
	m, extra, err := b.makeCaravansDesertSea()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(c.Map, m) || !reflect.DeepEqual(c.ExtraNumbers, extra) || !reflect.DeepEqual(g.Tiles, b.Tiles) || !reflect.DeepEqual(g.Ports, b.Ports) || !slices.Equal(sea.Islands, b.Seafarers.Islands) || !slices.Equal(sea.StartIslands, b.Seafarers.StartIslands) || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) {
		return errors.New("商队沙漠地图无效")
	}
	for i, v := range g.Vertices {
		w := b.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("商队海图交点无效")
		}
	}
	for i, e := range g.Edges {
		w := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("商队海图路线无效")
		}
	}
	if g.Robber >= 0 && !g.robberLandAllowed(g.Robber) {
		return errors.New("商队海图强盗位置无效")
	}
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("商队海盗位置无效")
	}
	return nil
}

// NewCatanCaravansDesertSeafarers exposes the verified printed recipe.
func NewCatanCaravansDesertSeafarers(n int) (*State, error) { return newCatanCaravansDesertSea(n) }

func newCatanCaravansTribeSea(n int) (*State, error) {
	if n < 3 || n > 6 {
		return nil, errors.New("商队部落支持三至六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, extra, err := g.makeCaravansDesertSea()
	if err != nil {
		return nil, err
	}
	g.Caravans = &catanCaravans{Sea: CatanCaravansSeafarersRules, Rules: CatanCaravansRules, Map: m, ExtraNumbers: extra, Wagons: []catanCaravanWagon{}}
	g.Robber = m.WateringHoles[0]
	g.Seafarers.VictoryPoints = 15
	if n <= 4 {
		s.Log = append(s.Log, "商队＋遗忘部落：12点麦田替换为水源，12叠放到2点牧场；强盗从水源开始且可返回。马车可沿海边延伸，己方船与马车同边计两段最长路线，15分获胜")
	}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人商队部落：主岛10点与2点麦田替换为两处水源，原数字依次移到最弱牧场，概率相同取编号较小者；33辆马车、34张发展卡、配对回合，15分获胜。三四人印刷配方不适用本图")
	}
	s.catanScores()
	return s, s.validateCaravans()
}
func (g *Catan) validateCaravansTribeSea() error {
	n := len(g.Players)
	sea, c := g.Seafarers, g.Caravans
	if !g.caravansSea() || (n < 3 || n > 6) || sea.Scenario != "tribe" || sea.Layout != "fixed" || sea.Variable || sea.Rules != CatanSeafarersRules || sea.NumberRecipe != "" || sea.VictoryPoints != 15 || sea.IslandBonus != 0 || len(sea.Seats) != n || sea.Tribe == nil || sea.Fog != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil {
		return errors.New("商队部落配置无效")
	}
	expected, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	b := expected.Catan
	m, extra, err := b.makeCaravansDesertSea()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, c.Map) || !reflect.DeepEqual(extra, c.ExtraNumbers) || !reflect.DeepEqual(g.Tiles, b.Tiles) || !slices.Equal(sea.Islands, b.Seafarers.Islands) || !slices.Equal(sea.StartIslands, b.Seafarers.StartIslands) || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) {
		return errors.New("商队部落地图无效")
	}
	for i, v := range g.Vertices {
		w := b.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("商队部落交点无效")
		}
	}
	for i, e := range g.Edges {
		w := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("商队部落路线无效")
		}
	}
	if g.Robber >= 0 && !g.robberLandAllowed(g.Robber) && !(g.EventDeck != nil && g.Tiles[g.Robber].Resource == CatanDesert) {
		return errors.New("商队海图强盗位置无效")
	}
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("商队部落海盗位置无效")
	}
	return g.validateRiverTribeRewards(b)
}

func NewCatanCaravansTribeSeafarers(n int) (*State, error) { return newCatanCaravansTribeSea(n) }

func (g *Catan) makeCaravansTribeExtended() (*catanCaravanMap, []catanFishingExtraNumber, error) {
	holes := []int{30, 33}
	numbers := []int{10, 2}
	for i, id := range holes {
		if g.Tiles[id].Resource != 3 || g.Tiles[id].Number != numbers[i] {
			return nil, nil, errors.New("商队部落扩大水源配方不符")
		}
		g.Tiles[id].Resource, g.Tiles[id].Number = catanWateringHole, 0
	}
	extra := []catanFishingExtraNumber{}
	for _, number := range numbers {
		best, weight := -1, 100
		for _, tile := range g.Tiles {
			if tile.Resource != 2 || tile.Number == 0 {
				continue
			}
			w := 6 - absCatan(7-tile.Number)
			for _, e := range extra {
				if e.Tile == tile.ID {
					w += 6 - absCatan(7-e.Number)
				}
			}
			if w < weight {
				best, weight = tile.ID, w
			}
		}
		if best < 0 {
			return nil, nil, errors.New("商队部落缺少牧场")
		}
		extra = append(extra, catanFishingExtraNumber{Tile: best, Number: number})
	}
	starts, err := caravanStarts(g, holes)
	if err != nil {
		return nil, nil, err
	}
	return &catanCaravanMap{WateringHoles: holes, Starts: starts, Supply: 33}, extra, nil
}
