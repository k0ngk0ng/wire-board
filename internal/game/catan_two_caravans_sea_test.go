package game

import (
	"fmt"
	"testing"
)

func TestCatanTwoCaravansSeaNatural(t *testing.T) {
	for _, scenario := range []string{"new_world", "islands", "shores", "desert", "tribe"} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/events%t", scenario, events), func(t *testing.T) {
				s, err := newCatanTwoCaravansSea(scenario)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 18000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(step, e)
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, s.Phase, a, e)
					}
					if step%137 == 0 {
						b := clone(*s)
						s = &b
						if e = s.validateCatanTwo(); e != nil {
							t.Fatal(e)
						}
						if e = s.validateCaravans(); e != nil {
							t.Fatal(e)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				t.Log("round", s.Round)
			})
		}
	}
}

func TestCatanTwoCaravansSeaIsolation(t *testing.T) {
	for _, scenario := range []string{"new_world", "islands", "shores", "desert", "tribe"} {
		s, err := newCatanTwoCaravansSea(scenario)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if len(g.Players) != 2 || len(g.Seafarers.Seats) != 2 || len(g.Two.SeaStarts) != 2 || len(g.Caravans.Map.Starts) != 3 {
			t.Fatal("seat or start inventory")
		}
		if g.tribe() != nil && (len(g.tribe().Points) != 2 || len(g.tribe().HeldPorts) != 2) {
			t.Fatal("phantom reward seats")
		}
		for _, mutate := range []func(*Catan){func(g *Catan) { g.Two.CaravansSea = "unknown" }, func(g *Catan) { g.Two.CaravansSea = "" }, func(g *Catan) { g.Caravans.Sea = "unknown" }, func(g *Catan) { g.Seafarers.VictoryPoints -= 2 }} {
			b := clone(*s)
			mutate(b.Catan)
			if b.validateCatanTwo() == nil && b.validateCaravans() == nil {
				t.Fatal("invalid combination accepted")
			}
		}
	}
}
