package game

import "testing"

func TestCatanTwoAttackSeaNatural(t *testing.T) {
	for _, events := range []bool{false, true} {
		s, e := newCatanTwoAttackShores()
		if e != nil {
			t.Fatal(e)
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
				t.Fatal(step, e)
			}
			if e = s.Apply(p, a); e != nil {
				t.Fatal(step, s.Phase, e)
			}
			if step%137 == 0 {
				b := clone(*s)
				s = &b
				if e = s.validateCatanTwo(); e != nil {
					t.Fatal(e)
				}
			}
		}
		if !s.Finished {
			t.Fatal("unfinished", events)
		}
	}
}

func TestCatanTwoAttackSeaIsolation(t *testing.T) {
	s, e := newCatanTwoAttackShores()
	if e != nil {
		t.Fatal(e)
	}
	g := s.Catan
	if len(g.Players) != 2 || len(g.Attack.Gold) != 2 || len(g.Attack.Prisoners) != 2 || len(g.Seafarers.Seats) != 2 || len(g.Two.SeaStarts) != 2 {
		t.Fatal("four player state leaked")
	}
	for _, v := range g.Two.SeaStarts {
		if g.Vertices[v].Owner >= -1 || g.Vertices[v].Level != 1 {
			t.Fatal("neutral setup")
		}
	}
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Two.AttackSea = "" }, func(g *Catan) { g.Two.AttackSea = "bad" }, func(g *Catan) { g.Attack.TwoRules = "" }, func(g *Catan) { g.Attack.Map.Sea = "" }, func(g *Catan) { g.Seafarers.VictoryPoints = 12 }} {
		b := clone(*s)
		mutate(b.Catan)
		if b.validateCatanTwo() == nil && b.validateCatanAttack() == nil {
			t.Fatal("unmarked mix accepted")
		}
	}
}
