package game

import (
	"slices"
	"testing"
)

func TestCatanExplorerPaidRoadsAndSettlements(t *testing.T) {
	g, f, c, v := explorerCargoFixture(t)
	explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 0, 1, catanFishingSide(g, 0, 3)) })
	for _, side := range []int{0, 1} {
		edge := catanFishingSide(g, 0, side)
		if err := c.buildRoad(g, f, 0, 1, edge); err != nil {
			t.Fatal(err)
		}
		explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 0, 1, edge) })
	}
	if f.Positions[0] != catanFishingSide(g, 0, 0) {
		t.Fatal("building a coastal road must not move its ship")
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildSettlement(g, f, 0, 1, v[1]) })
	if err := c.buildSettlement(g, f, 0, 1, v[2]); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(g.Players[0].Resources, []int{7, 7, 9, 9, 10}) || !slices.Equal(g.Bank, []int{12, 12, 10, 10, 9}) || g.Players[0].Score != 3 || g.Vertices[v[2]].Owner != 0 || g.Vertices[v[2]].Level != 1 {
		t.Fatal("road/settlement costs, bank and score")
	}
	explorerCargoRestore(t, g, f, c)
	if err := c.beginMovement(g, f, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 0, 1, catanFishingSide(g, 0, 2)) })
	explorerCargoReject(t, g, f, c, func() error { return c.buildSettlement(g, f, 0, 1, v[4]) })
}

func TestCatanExplorerRoadsCannotContinueThroughOtherOrNeutralBuilding(t *testing.T) {
	for _, owner := range []int{1, -2, -3} {
		g, f, c, v := explorerCargoFixture(t)
		g.Vertices[v[0]].Owner, g.Vertices[v[0]].Level = -1, 0
		g.Edges[catanFishingSide(g, 0, 0)].Owner = 0
		g.Vertices[v[1]].Owner, g.Vertices[v[1]].Level = owner, 1
		target := catanFishingSide(g, 0, 1)
		explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 0, 1, target) })
		g.Vertices[v[1]].Owner = 0
		if err := c.buildRoad(g, f, 0, 1, target); err != nil {
			t.Fatal("own building connects either side", owner, err)
		}
	}
	g, f, c, v := explorerCargoFixture(t)
	f.Positions[0] = catanFishingSide(g, 0, 1)
	explorerCargoReject(t, g, f, c, func() error { return c.buildSettlement(g, f, 0, 1, v[2]) })
	if err := c.buildRoad(g, f, 0, 1, catanFishingSide(g, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := c.buildRoad(g, f, 0, 1, catanFishingSide(g, 0, 1)); err != nil {
		t.Fatal(err)
	}
	if err := c.buildSettlement(g, f, 0, 1, v[2]); err != nil {
		t.Fatal("roads, not a moving ship, connect paid settlement", err)
	}
}

func TestCatanExplorerRoadFogEdgeVersusFogVertex(t *testing.T) {
	g, f, c, _ := explorerCargoFixture(t)
	if err := g.makeScenarioMap([]CatanHexSpec{{Resource: 0, Number: 6}, {Q: 1, Resource: CatanFog}}); err != nil {
		t.Fatal(err)
	}
	for i := range f.Positions {
		f.Positions[i] = -1
	}
	good, bad, home, tip := -1, -1, -1, -1
	for _, edge := range g.Edges {
		if slices.Contains(edge.Tiles, 0) && slices.Contains(edge.Tiles, 1) {
			bad = edge.ID
		}
		if len(edge.Tiles) != 1 || edge.Tiles[0] != 0 {
			continue
		}
		if catanExplorerLandVertex(g, edge.A) && !catanExplorerLandVertex(g, edge.B) {
			good, home, tip = edge.ID, edge.A, edge.B
		}
		if catanExplorerLandVertex(g, edge.B) && !catanExplorerLandVertex(g, edge.A) {
			good, home, tip = edge.ID, edge.B, edge.A
		}
	}
	if good < 0 || bad < 0 {
		t.Fatal("fixture lacks fog-corner geometry")
	}
	g.Vertices[home].Owner, g.Vertices[home].Level = 0, 2
	if !catanExplorerLandEdge(g, good) || catanExplorerLandEdge(g, bad) {
		t.Fatal("edge vs corner fog rules")
	}
	if err := c.buildRoad(g, f, 0, 1, good); err != nil {
		t.Fatal("known edge can end at fog corner", err)
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 0, 1, bad) })
	explorerCargoReject(t, g, f, c, func() error { return c.buildSettlement(g, f, 0, 1, tip) })
	explorerCargoRestore(t, g, f, c)
}

func TestCatanExplorerPaidBuildingLimitsAndAtomicCosts(t *testing.T) {
	g, f, c, _ := explorerCargoFixture(t)
	if err := g.makeScenarioMap([]CatanHexSpec{{Resource: 0, Number: 6}, {Q: 3, Resource: 0, Number: 6}, {Q: 6, Resource: 0, Number: 6}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		g.Edges[i].Owner = 0
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 0, 1, 15) })
	g.Edges[0].Owner = -1
	if err := c.buildRoad(g, f, 0, 1, 15); err != nil {
		t.Fatal("15th road should be usable", err)
	}
	g, f, c, _ = explorerCargoFixture(t)
	g.Bank[0] += g.Players[0].Resources[0]
	g.Players[0].Resources[0] = 0
	explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 0, 1, catanFishingSide(g, 0, 0)) })
	explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 1, 1, catanFishingSide(g, 0, 0)) })
	explorerCargoReject(t, g, f, c, func() error { return c.buildRoad(g, f, 0, 2, catanFishingSide(g, 0, 0)) })
}
