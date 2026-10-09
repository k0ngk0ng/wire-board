package game

import (
	"errors"
	"reflect"
	"slices"
)

// 2025 Merchant Trains + Seafarers p2. A central watering-hole island
// launches trains into the sea; the printed resource relocations preserve supply.
func (g *Catan) makeCaravansIslandsMap() (*catanCaravanMap, error) {
	if len(g.Players) > 4 {
		holes := []int{16, 36}
		for _, id := range holes {
			g.Tiles[id].Resource, g.Tiles[id].Number = catanWateringHole, 0
		}
		starts, err := caravanStarts(g, holes)
		if err != nil {
			return nil, err
		}
		g.Seafarers.Islands = g.findIslands()
		return &catanCaravanMap{WateringHoles: holes, Starts: starts, Supply: 33}, nil
	}
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
	if n < 3 || n > 6 {
		return nil, errors.New("商队四岛／六岛支持三至六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "islands", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := g.makeCaravansIslandsMap()
	if err != nil {
		return nil, err
	}
	g.Caravans = &catanCaravans{Sea: CatanCaravansSeafarersRules, Rules: CatanCaravansRules, Map: m, Wagons: []catanCaravanWagon{}}
	if n > 4 {
		g.Caravans.ExtraNumbers = []catanFishingExtraNumber{{Tile: 42, Number: 9}, {Tile: 52, Number: 8}}
		s.Log = append(s.Log, "本站五六人商队六岛：16号麦田和36号牧场改为水源，9与8分别叠到42号麦田及52号牧场；33辆马车、配对回合，15分获胜。保留六岛分区与港口。")
	}
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
	scenario := "islands"
	if n > 4 {
		scenario = "six_islands"
	}
	sea, c := g.Seafarers, g.Caravans
	if !g.caravansSea() || (n < 3 && !g.twoCaravansSea() || n > 6) || sea.Scenario != scenario || sea.Rules != CatanSeafarersRules || sea.Layout != "fixed" || sea.Variable || sea.NumberRecipe != "" || sea.VictoryPoints != 15 || sea.IslandBonus != 2 || len(sea.Seats) != n || sea.Fog != nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil {
		return errors.New("商队四岛配置无效")
	}
	ref, err := NewCatanSeafarers(recipeSeats, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "islands", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	b := ref.Catan
	m, err := b.makeCaravansIslandsMap()
	if err != nil {
		return err
	}
	var extra []catanFishingExtraNumber
	if n > 4 {
		extra = []catanFishingExtraNumber{{Tile: 42, Number: 9}, {Tile: 52, Number: 8}}
	}
	if !reflect.DeepEqual(extra, c.ExtraNumbers) {
		return errors.New("商队六岛额外数字无效")
	}
	if len(g.Ports) != len(b.Ports) {
		return errors.New("商队岛屿港口数量无效")
	}
	got, want := []int{}, []int{}
	for i, p := range g.Ports {
		if p.Edge != b.Ports[i].Edge || n <= 4 && p.Resource != b.Ports[i].Resource {
			return errors.New("商队岛屿港口位置无效")
		}
		got = append(got, p.Resource)
		want = append(want, b.Ports[i].Resource)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		return errors.New("商队岛屿港口库存无效")
	}
	if !reflect.DeepEqual(m, c.Map) || !reflect.DeepEqual(g.Tiles, b.Tiles) || !slices.Equal(sea.Islands, b.Seafarers.Islands) || len(sea.StartIslands) != 0 || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) {
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
