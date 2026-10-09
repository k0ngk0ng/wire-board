package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanCaravansShoresExtendedNatural(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanCaravansShoresExtended(n)
				if err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if len(g.Tiles) != 56 || len(g.Caravans.Map.Starts) != 6 || g.Caravans.Map.Supply != 33 || len(g.Ports) != 11 || g.Paired == nil || g.victoryTarget() != 16 {
					t.Fatal("components")
				}
				regions := map[int]int{}
				for _, id := range g.Seafarers.Islands {
					if id >= 0 {
						regions[id]++
					}
				}
				sizes := []int{}
				for _, size := range regions {
					sizes = append(sizes, size)
				}
				slices.Sort(sizes)
				if !slices.Equal(sizes, []int{2, 2, 3, 3, 30}) {
					t.Fatal("regions")
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 24000 && !s.Finished; step++ {
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
						if e = s.validateCaravans(); e != nil {
							t.Fatal(e)
						}
						if e = s.validateCatanEventSession(); e != nil {
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

func TestCatanCaravansShoresExtendedMapIntegrity(t *testing.T) {
	s, err := newCatanCaravansShoresExtended(6)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	for _, pair := range [][2]int{{6, 23}, {40, 55}} {
		before := g.Seafarers.Seats[0].IslandPoints
		for _, id := range pair {
			v := g.Tiles[id].Vertices[0]
			if g.seaSetupAllowed(v) {
				t.Fatal("outer setup")
			}
			s.catanSettleIsland(0, v, false)
		}
		if g.Seafarers.Seats[0].IslandPoints != before+2 {
			t.Fatal("duplicate exploration reward")
		}
	}
	for name, mutate := range map[string]func(*Catan){
		"terrain": func(g *Catan) { g.Tiles[0].Resource = 0 }, "number": func(g *Catan) { g.Tiles[0].Number = 7 },
		"home": func(g *Catan) { g.Seafarers.StartIslands = nil }, "region": func(g *Catan) { g.Seafarers.Islands[23] = g.Seafarers.StartIslands[0] },
		"hole": func(g *Catan) { g.Caravans.Map.WateringHoles[0] = 0 }, "origin": func(g *Catan) { g.Caravans.Map.Starts[0].Edge = -1 },
		"port": func(g *Catan) { g.Ports[0].Resource = 9 }, "supply": func(g *Catan) { g.Caravans.Map.Supply = 22 },
	} {
		b := clone(*s)
		mutate(b.Catan)
		if b.validateCaravans() == nil {
			t.Fatal("accepted", name)
		}
	}
}
