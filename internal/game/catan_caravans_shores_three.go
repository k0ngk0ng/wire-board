package game

import (
	"errors"
	"reflect"
)

// 2025 Merchant Trains + Seafarers, page 1, three-player printed diagram.
func (g *Catan) makeCaravansShoresThree() (*catanCaravanMap, []catanFishingExtraNumber, error) {
	g.Tiles[15].Number = 9
	g.Tiles[22].Resource, g.Tiles[22].Number = catanWateringHole, 0
	starts, err := caravanStarts(g, []int{22})
	if err != nil {
		return nil, nil, err
	}
	return &catanCaravanMap{WateringHoles: []int{22}, Starts: starts, Supply: 22}, []catanFishingExtraNumber{{Tile: 0, Number: 2}}, nil
}
func newCatanCaravansShoresThree() (*State, error) {
	s, err := NewCatanSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "shores", Layout: "fixed"}, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, extra, err := g.makeCaravansShoresThree()
	if err != nil {
		return nil, err
	}
	g.Caravans = &catanCaravans{Sea: CatanCaravansSeafarersRules, Rules: CatanCaravansRules, Map: m, ExtraNumbers: extra, Wagons: []catanCaravanWagon{}}
	g.Seafarers.VictoryPoints = 16
	s.Log = append(s.Log, "三人商队新海岸：采用官方组合固定图，中央水源不生产，西北山丘2与12均产出，16分获胜；马车可走海格边，同边己方船计两段路线")
	s.catanScores()
	return s, s.validateCaravans()
}
func (g *Catan) validateCaravansShoresThree() error {
	sea, c := g.Seafarers, g.Caravans
	if !g.caravansSea() || len(g.Players) != 3 || sea.Scenario != "shores" || sea.Rules != CatanSeafarersRules || sea.Layout != "fixed" || sea.Variable || sea.NumberRecipe != "" || sea.VictoryPoints != g.caravansSeaVictoryPoints(16) || sea.IslandBonus != 2 || len(sea.Seats) != 3 || sea.Fog != nil || sea.Tribe != nil || sea.NewWorld != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil {
		return errors.New("三人商队新海岸配置无效")
	}
	ref, err := NewCatanSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "shores", Layout: "fixed"}, nil)
	if err != nil {
		return err
	}
	b := ref.Catan
	m, extra, err := b.makeCaravansShoresThree()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(c.Map, m) || !reflect.DeepEqual(c.ExtraNumbers, extra) || !reflect.DeepEqual(g.Tiles, b.Tiles) || !reflect.DeepEqual(g.Ports, b.Ports) || !reflect.DeepEqual(sea.Islands, b.Seafarers.Islands) || !reflect.DeepEqual(sea.StartIslands, b.Seafarers.StartIslands) || len(g.Vertices) != len(b.Vertices) || len(g.Edges) != len(b.Edges) {
		return errors.New("三人商队新海岸地图无效")
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
			return errors.New("商队海岸边无效")
		}
	}
	if g.Robber >= 0 && !g.robberLandAllowed(g.Robber) || sea.Pirate < -1 || sea.Pirate >= len(g.Tiles) || sea.Pirate >= 0 && g.Tiles[sea.Pirate].Resource != CatanSea {
		return errors.New("商队海岸强盗海盗位置无效")
	}
	return nil
}
