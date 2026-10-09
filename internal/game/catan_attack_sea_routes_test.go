package game

import "testing"

func TestCatanAttackSeaCoastalAnchor(t *testing.T) {
	s, e := NewCatanSeafarers(4, CatanOptions{}, CatanSeafarersSetup{Scenario: "islands", Layout: "fixed"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	g := s.Catan
	g.Attack = &catanAttack{Barbarians: make([]int, len(g.Tiles))}
	edge := -1
	for _, r := range g.Edges {
		if g.edgeTerrain(r.ID, true) && g.edgeTerrain(r.ID, false) && len(r.Tiles) == 2 {
			edge = r.ID
			break
		}
	}
	if edge < 0 {
		t.Fatal("no coast")
	}
	v := g.Edges[edge].A
	g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
	if !g.canShip(0, edge) {
		t.Fatal("unconquered anchor")
	}
	for _, tile := range g.Tiles {
		if tile.Resource != CatanSea {
			g.Attack.Barbarians[tile.ID] = 3
		}
	}
	if !g.Attack.conqueredBuilding(g, v) {
		t.Fatal("sea improperly prevents conquest")
	}
	if g.canShip(0, edge) {
		t.Fatal("conquered anchor allows extension")
	}
	// Liberating adjacent land restores the coastal building and its route.
	restored := -1
	for _, tile := range g.Tiles {
		if tile.Resource == CatanSea {
			continue
		}
		for _, id := range tile.Vertices {
			if id == v {
				for _, adj := range g.Edges[edge].Tiles {
					if adj == tile.ID {
						restored = tile.ID
					}
				}
			}
		}
	}
	if restored < 0 {
		t.Fatal("land missing")
	}
	g.Attack.Barbarians[restored] = 0
	if !g.canShip(0, edge) {
		t.Fatal("liberation does not restore route")
	}
}
