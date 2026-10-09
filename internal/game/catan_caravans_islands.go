package game

import (
	"errors"
	"reflect"
	"slices"
)

// 2025 Merchant Trains + Seafarers p2. A central watering-hole island
// launches trains into the sea; the printed resource relocations preserve supply.
func (g *Catan) makeCaravansIslandsMap() (*catanCaravanMap, error) {
	if len(g.Players) == 3 {
		for id, t := range map[int]seaTerrain{12: {CatanSea, 0}, 13: {1, 11}, 14: {0, 8}, 17: {catanWateringHole, 0}, 22: {CatanSea, 0}, 32: {0, 3}} {
			g.Tiles[id].Resource, g.Tiles[id].Number = t.resource, t.number
		}
	} else {
		for id, t := range map[int]seaTerrain{3: {1, 3}, 12: {CatanSea, 0}, 13: {1, 10}, 14: {0, 6}, 17: {catanWateringHole, 0}} {
			g.Tiles[id].Resource, g.Tiles[id].Number = t.resource, t.number
		}
	}
	// Ports facing relocated land must follow that land. The printed diagram
	// specifies their types; positions are resolved from tile sides.
	ports := []seaPort{{1, 4, 4, -1}, {1, 1, 0, 4}, {2, 0, 3, -1}, {2, 4, 1, 1}, {4, 0, 4, 0}, {4, 0, 1, -1}, {6, 0, 0, 2}, {4, 4, 3, -1}, {4, 5, 0, 3}}
	if len(g.Players) == 4 {
		ports = []seaPort{{1, 0, 2, 3}, {2, 0, 0, -1}, {4, 4, 3, -1}, {2, 5, 1, 0}, {4, 5, 3, 4}, {4, 5, 0, -1}, {5, 1, 4, -1}, {5, 2, 0, 1}, {6, 0, 2, 2}}
	}
	rows := []int{4, 5, 6, 5, 6, 5, 4}
	g.Ports = nil
	for _, p := range ports {
		id := p.col
		for r := 0; r < p.row; r++ {
			id += rows[r]
		}
		edge := catanFishingSide(g, id, p.side)
		if edge < 0 {
			return nil, errors.New("商队四岛港口缺失")
		}
		g.Ports = append(g.Ports, CatanPort{Edge: edge, Resource: p.resource})
	}
	g.Seafarers.Islands = g.findIslands()
	starts, err := caravanStarts(g, []int{17})
	if err != nil {
		return nil, err
	}
	return &catanCaravanMap{WateringHoles: []int{17}, Starts: starts, Supply: 22}, nil
}
func NewCatanCaravansIslandsSeafarers(n int) (*State, error) {
	if n == 2 {
		return newCatanTwoCaravansSea("islands")
	}
	return newCatanCaravansIslands(n)
}

func newCatanCaravansIslands(n int) (*State, error) {
	if n != 3 && n != 4 {
		return nil, errors.New("商队四岛暂支持三四人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "islands", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := g.makeCaravansIslandsMap()
	if err != nil {
		return nil, err
	}
	g.Caravans = &catanCaravans{Sea: CatanCaravansSeafarersRules, Rules: CatanCaravansRules, Map: m, Wagons: []catanCaravanWagon{}}
	g.Seafarers.VictoryPoints = 15
	s.Log = append(s.Log, "商队四岛：中央水源向海格延伸马车，海盗不阻挡商队；己方船与马车同边计两段路线，15分获胜")
	if n == 3 {
		s.Log = append(s.Log, "本站三人四岛补充：官方图海格上的孤立4点数字不入库存，海洋不生产资源；该位置保持海洋")
	}
	s.catanScores()
	return s, s.validateCaravans()
}
func (g *Catan) validateCaravansIslands() error {
	n := len(g.Players)
	recipeSeats := n
	if g.twoCaravansSea() {
		recipeSeats = 4
	}
	sea, c := g.Seafarers, g.Caravans
	if !g.caravansSea() || (n != 3 && n != 4 && !g.twoCaravansSea()) || sea.Scenario != "islands" || sea.Rules != CatanSeafarersRules || sea.Layout != "fixed" || sea.Variable || sea.NumberRecipe != "" || sea.VictoryPoints != 15 || sea.IslandBonus != 2 || len(sea.Seats) != n || sea.Fog != nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil || len(c.ExtraNumbers) != 0 {
		return errors.New("商队四岛配置无效")
	}
	ref, err := NewCatanSeafarers(recipeSeats, CatanOptions{}, CatanSeafarersSetup{Scenario: "islands", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	b := ref.Catan
	m, err := b.makeCaravansIslandsMap()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, c.Map) || !reflect.DeepEqual(g.Tiles, b.Tiles) || !reflect.DeepEqual(g.Ports, b.Ports) || !slices.Equal(sea.Islands, b.Seafarers.Islands) || len(sea.StartIslands) != 0 || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) {
		return errors.New("商队四岛地图无效")
	}
	for i, v := range g.Vertices {
		w := b.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("商队四岛交点无效")
		}
	}
	for i, e := range g.Edges {
		w := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("商队四岛边无效")
		}
	}
	if g.Robber >= 0 && !g.robberLandAllowed(g.Robber) || sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("商队四岛强盗海盗位置无效")
	}
	return nil
}
