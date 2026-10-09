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
	number := 2
	if len(g.Players) == 4 {
		number = 11
	}
	// Locate the printed terrain/number pair, independent of display coordinates.
	sourceResource, targetResource, targetNumber := 3, 2, 12
	if len(g.Players) == 4 {
		sourceResource, targetResource, targetNumber = 1, 1, 2
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
	n := len(g.Players)
	sea, c := g.Seafarers, g.Caravans
	if !g.caravansSea() || (n != 3 && n != 4) || sea.Scenario != "desert" || sea.Layout != "fixed" || sea.Rules != CatanSeafarersRules || sea.Variable || sea.VictoryPoints != 16 || sea.IslandBonus != 2 || len(sea.Seats) != n || sea.Fog != nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil {
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
	if sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("商队海盗位置无效")
	}
	return nil
}
