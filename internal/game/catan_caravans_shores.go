package game

import (
	"errors"
	"reflect"
	"slices"
)

// The extension's 30-hex mainland matches the extended merchant-train board.
// Its printed outer islands and exploration regions remain unchanged.
func newCatanCaravansShoresExtended(n int) (*State, error) {
	if n < 5 || n > 6 {
		return nil, errors.New("商队新海岸扩大配方需要五或六人")
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "shores", Layout: "variable"}, nil)
	if err != nil {
		return nil, err
	}
	mainland, err := NewCatanCaravans(n, CatanOptions{FiveSix: true})
	if err != nil {
		return nil, err
	}
	g := s.Catan
	ids := riverShoresMainlandIDs()
	for i, id := range ids {
		g.Tiles[id].Resource, g.Tiles[id].Number = mainland.Catan.Tiles[i].Resource, mainland.Catan.Tiles[i].Number
	}
	holes, _, _ := catanCaravanRecipe(true)
	for i := range holes {
		holes[i] = ids[holes[i]]
	}
	starts, err := caravanStarts(g, holes)
	if err != nil {
		return nil, err
	}
	g.Caravans = &catanCaravans{Sea: CatanCaravansSeafarersRules, Rules: CatanCaravansRules, Map: &catanCaravanMap{WateringHoles: holes, Starts: starts, Supply: 33, NumberRecipe: CatanExtendedNumberRecipe}, Wagons: []catanCaravanWagon{}}
	g.Robber = -1
	g.Seafarers.VictoryPoints = 16
	s.Log = append(s.Log, "本站五六人商队新海岸：主岛采用五六人商队双水源与数字序列，保留航海家外岛和探索区域；33辆马车，强盗从场外开始，16分获胜，配对回合", catanExtendedNumberNotice, "马车可沿海格边延伸，不受海盗阻挡；己方船与马车同边时计两段最长路线")
	s.catanScores()
	return s, s.validateCaravans()
}

func (g *Catan) validateCaravansShoresExtended() error {
	n := len(g.Players)
	sea, c := g.Seafarers, g.Caravans
	if !g.caravansSea() || n < 5 || n > 6 || sea.Scenario != "shores" || sea.Layout != "variable" || !sea.Variable || sea.Rules != CatanSeafarersRules || sea.NumberRecipe != CatanExtendedNumberRecipe || sea.VictoryPoints != 16 || sea.IslandBonus != 2 || len(sea.Seats) != n || sea.Fog != nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil || c.Map == nil || len(c.ExtraNumbers) != 0 {
		return errors.New("扩大商队新海岸配置无效")
	}
	expected, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "shores", Layout: "variable"}, nil)
	if err != nil {
		return err
	}
	b := expected.Catan
	if len(g.Tiles) != len(b.Tiles) || len(g.Edges) != len(b.Edges) || len(g.Vertices) != len(b.Vertices) || !slices.Equal(sea.Islands, b.Seafarers.Islands) || !slices.Equal(sea.StartIslands, b.Seafarers.StartIslands) {
		return errors.New("商队新海岸尺寸或分区无效")
	}
	main, err := NewCatanCaravans(n, CatanOptions{FiveSix: true})
	if err != nil {
		return err
	}
	ids := riverShoresMainlandIDs()
	for i, id := range ids {
		main.Catan.Tiles[i].Resource, main.Catan.Tiles[i].Number = g.Tiles[id].Resource, g.Tiles[id].Number
		b.Tiles[id].Resource, b.Tiles[id].Number = g.Tiles[id].Resource, g.Tiles[id].Number
	}
	if err = main.validateCaravans(); err != nil {
		return err
	}
	holes, _, _ := catanCaravanRecipe(true)
	for i := range holes {
		holes[i] = ids[holes[i]]
	}
	starts, err := caravanStarts(b, holes)
	if err != nil {
		return err
	}
	want := &catanCaravanMap{WateringHoles: holes, Starts: starts, Supply: 33, NumberRecipe: CatanExtendedNumberRecipe}
	if !reflect.DeepEqual(c.Map, want) || !reflect.DeepEqual(g.Tiles, b.Tiles) {
		return errors.New("商队新海岸地形数字或水源无效")
	}
	for i, v := range g.Vertices {
		w := b.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("商队海岸交点无效")
		}
	}
	for i, e := range g.Edges {
		w := b.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("商队海岸路线无效")
		}
	}
	if len(g.Ports) != len(b.Ports) {
		return errors.New("商队海岸港口数量无效")
	}
	got, ports := []int{}, []int{}
	for i, p := range g.Ports {
		if p.Edge != b.Ports[i].Edge {
			return errors.New("商队海岸港口位置无效")
		}
		got = append(got, p.Resource)
		ports = append(ports, b.Ports[i].Resource)
	}
	slices.Sort(got)
	slices.Sort(ports)
	if !slices.Equal(got, ports) {
		return errors.New("商队海岸港口库存无效")
	}
	if g.Robber >= 0 && !g.robberLandAllowed(g.Robber) || sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("商队海岸强盗海盗位置无效")
	}
	return nil
}

func NewCatanCaravansShoresSeafarers(n int) (*State, error) { return newCatanCaravansShoresExtended(n) }
