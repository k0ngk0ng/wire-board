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

func TestCatanAttackDesertThreeNatural(t *testing.T) {
	for _, events := range []bool{false, true} {
		s, e := newCatanAttackDesertThree()
		if e != nil {
			t.Fatal(e)
		}
		if events {
			if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
				t.Fatal(e)
			}
		}
		for step := 0; step < 16000 && !s.Finished; step++ {
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(step, e)
			}
			if e = s.Apply(p, a); e != nil {
				t.Fatal(step, e)
			}
			if step%137 == 0 {
				b := clone(*s)
				s = &b
				if e = s.validateCatanAttack(); e != nil {
					t.Fatal(e)
				}
			}
		}
		if !s.Finished {
			t.Fatal("unfinished")
		}
	}
}

func TestCatanAttackDesertThreeLandingBoundary(t *testing.T) {
	s, e := newCatanAttackDesertThree()
	if e != nil {
		t.Fatal(e)
	}
	g := s.Catan
	seen := map[int]bool{}
	for _, id := range g.Attack.Map.Coast {
		if seen[g.Tiles[id].Number] || g.Seafarers.Islands[id] != g.Seafarers.StartIslands[0] {
			t.Fatal("landing partition")
		}
		seen[g.Tiles[id].Number] = true
	}
	for _, edge := range g.Attack.recruitEdges(g, 0, "swift_knight") {
		if !g.attackSeaKnightEdge(edge) {
			t.Fatal("recruit outside mainland")
		}
	}
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Tiles[6].Number = 4 }, func(g *Catan) { g.Attack.Map.Coast[0] = 0 }, func(g *Catan) { g.Seafarers.StartIslands[0] = g.Seafarers.Islands[0] }} {
		b := clone(*s)
		mutate(b.Catan)
		if b.validateCatanAttack() == nil {
			t.Fatal("invalid desert map accepted")
		}
	}
}

func TestCatanAttackDesertFourNatural(t *testing.T) {
	for _, events := range []bool{false, true} {
		s, e := newCatanAttackDesert(4)
		if e != nil {
			t.Fatal(e)
		}
		if events {
			if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
				t.Fatal(e)
			}
		}
		for step := 0; step < 16000 && !s.Finished; step++ {
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(step, e)
			}
			if e = s.Apply(p, a); e != nil {
				t.Fatal(step, e)
			}
			if step%137 == 0 {
				b := clone(*s)
				s = &b
				if e = s.validateCatanAttack(); e != nil {
					t.Fatal(e)
				}
			}
		}
		if !s.Finished {
			t.Fatal("unfinished")
		}
	}
}

func TestCatanAttackDesertFourPrintedRecipe(t *testing.T) {
	g, m, e := newCatanAttackDesertFourBoard()
	if e != nil {
		t.Fatal(e)
	}
	if g.Tiles[19].Resource != catanCastle || g.Tiles[15].Number != 12 || g.Tiles[24].Number != 2 || g.Tiles[38].Number != 10 || len(m.Coast) != 10 {
		t.Fatal("printed recipe")
	}
	seen := map[int]bool{}
	for _, id := range m.Coast {
		n := g.Tiles[id].Number
		if seen[n] || n < 2 || n == 7 || g.Seafarers.Islands[id] != g.Seafarers.StartIslands[0] {
			t.Fatal("landing number or region", id, n)
		}
		seen[n] = true
	}
	for _, p := range g.Ports {
		if !g.edgeTerrain(p.Edge, true) || !g.edgeTerrain(p.Edge, false) {
			t.Fatal("port")
		}
	}
	if g.Tiles[0].Resource != CatanGold || g.Tiles[40].Number != 6 || g.Tiles[41].Number != 12 {
		t.Fatal("outer exploration")
	}
}
