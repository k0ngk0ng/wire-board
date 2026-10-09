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
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Tiles[0].Number = 9 }, func(g *Catan) { g.Attack.Map.Coast[0] = 0 }, func(g *Catan) { g.Seafarers.Pirate = 2 }, func(g *Catan) { g.Ports[0].Resource = 99 }, func(g *Catan) { g.Seafarers.NewWorld = &CatanNewWorld{} }} {
		b := clone(*s)
		mutate(b.Catan)
		if b.validateCatanAttack() == nil {
			t.Fatal("corrupt sea state")
		}
	}
}

func TestCatanAttackShoresEventsNatural(t *testing.T) {
	s, e := newCatanAttackShores(4)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
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

func TestCatanAttackShoresAllEventFaces(t *testing.T) {
	for kind := range catanCardEventNames {
		t.Run(kind, func(t *testing.T) {
			s, e := newCatanAttackShores(4)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
				t.Fatal(e)
			}
			finishAttackSetup(t, s)
			referenceEventTop(t, s, kind)
			turn := s.Turn
			if e = s.Apply(turn, Action{Type: "catan_roll"}); e != nil {
				t.Fatal(e)
			}
			for step := 0; s.Phase != "catan_turn" && step < 40; step++ {
				b := clone(*s)
				s = &b
				if e = s.validateCatanEventSession(); e != nil {
					t.Fatal(e)
				}
				actor := twoFullActor(s)
				a, e := s.BotAction(actor)
				if e != nil {
					t.Fatal(e)
				}
				if e = s.Apply(actor, a); e != nil {
					t.Fatal(e)
				}
			}
			if s.Phase != "catan_turn" || s.Catan.Robber != -1 || s.Catan.Seafarers.Pirate != -1 || s.Catan.Attack.Sequence != 0 {
				t.Fatal("event changed pieces or failed continuation", s.Phase)
			}
		})
	}
}

func TestCatanAttackShoresExtendedNatural(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, events := range []bool{false, true} {
			s, e := newCatanAttackShores(n)
			if e != nil {
				t.Fatal(n, e)
			}
			if events {
				if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
					t.Fatal(e)
				}
			}
			for step := 0; step < 18000 && !s.Finished; step++ {
				p := twoFullActor(s)
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(n, step, e)
				}
				if e = s.Apply(p, a); e != nil {
					t.Fatal(n, step, s.Phase, e)
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
				t.Fatal("unfinished", n, events)
			}
		}
	}
}

func TestCatanAttackShoresExtendedComponents(t *testing.T) {
	for _, n := range []int{5, 6} {
		s, e := newCatanAttackShores(n)
		if e != nil {
			t.Fatal(e)
		}
		g := s.Catan
		m := g.Attack.Map
		if len(m.Castles) != 2 || len(m.Coast) != 12 || m.Barbarians != 48 || m.Gold != 152 || g.Paired == nil || len(g.Ports) != 11 {
			t.Fatal("extended components")
		}
		for _, id := range m.Coast {
			if g.Seafarers.Islands[id] != g.Seafarers.StartIslands[0] {
				t.Fatal("outside landing")
			}
		}
		used := map[int]bool{}
		for _, p := range g.Ports {
			edge := g.Edges[p.Edge]
			if !g.edgeTerrain(p.Edge, true) || !g.edgeTerrain(p.Edge, false) || used[edge.A] || used[edge.B] {
				t.Fatal("port geometry")
			}
			used[edge.A], used[edge.B] = true, true
		}
		b := clone(*s)
		b.Catan.Attack.Map.Barbarians = 36
		if b.validateCatanAttack() == nil {
			t.Fatal("base supply accepted")
		}
	}
}
