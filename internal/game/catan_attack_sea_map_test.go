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

func TestCatanAttackShoresNatural(t *testing.T) {
	s, e := newCatanAttackShores(4)
	if e != nil {
		t.Fatal(e)
	}
	for step := 0; step < 14000 && !s.Finished; step++ {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(step, e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(step, s.Phase, e)
		}
		if step%137 == 0 {
			b := clone(*s)
			s = &b
			if e = s.validateCatanAttack(); e != nil {
				t.Fatal(step, e)
			}
		}
	}
	if !s.Finished {
		t.Fatal("unfinished")
	}
}

func TestCatanAttackShoresKnightBoundaryAndCorruption(t *testing.T) {
	s, e := newCatanAttackShores(4)
	if e != nil {
		t.Fatal(e)
	}
	g := s.Catan
	for _, edge := range g.Attack.recruitEdges(g, 0, "swift_knight") {
		if !g.attackSeaKnightEdge(edge) {
			t.Fatal("outer recruitment")
		}
	}
	for _, edge := range g.Attack.recruitEdges(g, 0, "knighthood") {
		g.Attack.Knights = []catanAttackKnight{{Player: 0, Edge: edge}}
		for dest := range g.Attack.knightDestinations(g, 0, 5) {
			if !g.attackSeaKnightEdge(dest) {
				t.Fatal("outer travel")
			}
		}
	}
	g.Attack.Knights = nil
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Tiles[0].Number = 9 }, func(g *Catan) { g.Attack.Map.Coast[0] = 0 }, func(g *Catan) { g.Seafarers.Pirate = 2 }, func(g *Catan) { g.Ports[0].Resource = 99 }} {
		b := clone(*s)
		mutate(b.Catan)
		if b.validateCatanAttack() == nil {
			t.Fatal("corrupt sea state")
		}
	}
}
