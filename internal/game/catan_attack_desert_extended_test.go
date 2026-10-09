package game

import "testing"

func TestCatanAttackDesertExtendedBoard(t *testing.T) {
	for _, n := range []int{5, 6} {
		g, m, e := newCatanAttackDesertExtendedBoard(n)
		if e != nil {
			t.Fatal(e)
		}
		for _, id := range m.Castles {
			if g.Seafarers.Islands[id] != g.Seafarers.StartIslands[0] {
				t.Fatal("castle outside mainland", id)
			}
		}
		t.Log(m.Coast)
	}
}

func TestCatanAttackDesertExtendedNatural(t *testing.T) {
	for _, n := range []int{5, 6} {
		s, e := newCatanAttackDesert(n)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
			t.Fatal(e)
		}
		for step := 0; step < 18000 && !s.Finished; step++ {
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Apply(p, a); e != nil {
				t.Fatal(step, e)
			}
		}
		if !s.Finished {
			t.Fatal("unfinished")
		}
	}
}
