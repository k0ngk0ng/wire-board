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

func TestCatanAttackSeaCannotMoveFromConqueredAnchor(t *testing.T) {
	s, err := NewCatanSeafarers(4, CatanOptions{}, CatanSeafarersSetup{Scenario: "islands", Layout: "fixed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.Attack = &catanAttack{Barbarians: make([]int, len(g.Tiles))}
	edge := -1
	for _, r := range g.Edges {
		if g.edgeTerrain(r.ID, true) && g.landVertex(r.A) {
			edge = r.ID
			break
		}
	}
	if edge < 0 {
		t.Fatal("no coast")
	}
	e := g.Edges[edge]
	g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
	g.Edges[edge].Owner, g.Edges[edge].Ship = 0, true
	if !g.movableShip(0, edge) {
		t.Fatal("unconquered source should move")
	}
	for i, tile := range g.Tiles {
		if tile.Resource != CatanSea {
			g.Attack.Barbarians[i] = 3
		}
	}
	if !g.openRoute(0, edge) {
		t.Fatal("fixture is not open")
	}
	if g.movableShip(0, edge) || len(g.shipDestinations(0, edge)) != 0 {
		t.Fatal("conquered route can be modified")
	}
	// Existing sea games without Barbarian Attack retain their former behavior.
	g.Attack = nil
	if !g.movableShip(0, edge) {
		t.Fatal("ordinary sea movement changed")
	}
}

func TestCatanAttackSeaShipAllowedAlongConqueredLand(t *testing.T) {
	s, err := NewCatanSeafarers(4, CatanOptions{}, CatanSeafarersSetup{Scenario: "islands", Layout: "fixed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.Attack = &catanAttack{Barbarians: make([]int, len(g.Tiles))}
	for _, e := range g.Edges {
		if !g.edgeTerrain(e.ID, true) || !g.edgeTerrain(e.ID, false) {
			continue
		}
		for _, v := range []int{e.A, e.B} {
			for _, land := range e.Tiles {
				if g.Tiles[land].Resource == CatanSea {
					continue
				}
				g.Attack.Barbarians[land] = 3
				g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
				if !g.Attack.conqueredBuilding(g, v) {
					if !g.canShip(0, e.ID) || g.canRoad(0, e.ID) {
						t.Fatal("conquered coast must allow ships but forbid roads")
					}
					return
				}
				g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
				g.Attack.Barbarians[land] = 0
			}
		}
	}
	t.Fatal("no coast with surviving building")
}
