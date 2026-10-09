package game

import "testing"

func TestCatanAttackDesertThreeGeometry(t *testing.T) {
	g, e := newCatanAttackDesertThreeGeometry()
	if e != nil {
		t.Fatal(e)
	}
	if len(g.Tiles) != 35 || g.Tiles[20].Resource != catanCastle || g.Tiles[6].Number != 12 || g.Tiles[26].Number != 2 || g.Tiles[32].Number != 11 {
		t.Fatal("printed positions")
	}
	for _, id := range []int{1, 5, 10} {
		if g.Tiles[id].Resource != CatanDesert {
			t.Fatal("desert belt")
		}
	}
	if g.Seafarers.Islands[0] == g.Seafarers.StartIslands[0] || g.Seafarers.Islands[22] != g.Seafarers.StartIslands[0] {
		t.Fatal("exploration regions")
	}
	seen := map[int]bool{}
	for _, p := range g.Ports {
		if !g.edgeTerrain(p.Edge, true) || !g.edgeTerrain(p.Edge, false) {
			t.Fatal("noncoastal port", p)
		}
		e := g.Edges[p.Edge]
		if seen[e.A] || seen[e.B] {
			t.Fatal("overlapping ports")
		}
		seen[e.A], seen[e.B] = true, true
	}
}
