package game

import "testing"

func TestCatanAttackShoresPrintedBoard(t *testing.T) {
	g, m, err := newCatanAttackShoresBoard(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Tiles) != 42 || len(m.Coast) != 10 || len(m.Castles) != 1 || m.Castles[0] != 37 || g.Tiles[14].Resource != CatanDesert || g.Tiles[39].Number != 2 || g.Tiles[12].Number != 3 {
		t.Fatal("printed mainland layout")
	}
	if g.Tiles[0].Number != 8 || g.Tiles[3].Resource != CatanGold || g.Tiles[40].Resource != CatanSea || g.Seafarers.VictoryPoints != 14 || g.Robber != -1 || g.Seafarers.Pirate != -1 {
		t.Fatal("outer map or rules")
	}
	nums := map[int]bool{}
	for _, id := range m.Coast {
		n := g.Tiles[id].Number
		if n < 2 || n == 7 || nums[n] {
			t.Fatal("landing numbers not unique")
		}
		nums[n] = true
		if g.Seafarers.Islands[id] != g.Seafarers.StartIslands[0] {
			t.Fatal("landing outside mainland")
		}
	}
	if len(g.Ports) != 9 {
		t.Fatal("port inventory")
	}
	occupied := map[int]bool{}
	for _, p := range g.Ports {
		if !g.edgeTerrain(p.Edge, true) || !g.edgeTerrain(p.Edge, false) {
			t.Fatal("noncoastal port")
		}
		e := g.Edges[p.Edge]
		if occupied[e.A] || occupied[e.B] {
			t.Fatal("overlapping ports")
		}
		occupied[e.A], occupied[e.B] = true, true
	}
}
