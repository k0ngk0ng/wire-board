package game

import (
	"fmt"
	"reflect"
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

func TestCatanCaravansShoresFourNatural(t *testing.T) {
	for _, events := range []bool{false, true} {
		t.Run(fmt.Sprint(events), func(t *testing.T) {
			s, err := newCatanCaravansShoresExtended(4)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Catan.Caravans.Map.Starts) != 3 || s.Catan.Caravans.Map.Supply != 22 {
				t.Fatal("mainland inventory")
			}
			if events {
				if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
			}
			for step := 0; step < 16000 && !s.Finished; step++ {
				p := twoFullActor(s)
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(e)
				}
				if e = s.Apply(p, a); e != nil {
					t.Fatal(step, s.Phase, e)
				}
				if step%113 == 0 {
					b := clone(*s)
					s = &b
					if e = s.validateCaravans(); e != nil {
						t.Fatal(e)
					}
				}
			}
			if !s.Finished {
				t.Fatal("unfinished")
			}
			t.Log("round", s.Round)
		})
	}
}

func TestCatanCaravansShoresFourMapIntegrity(t *testing.T) {
	for sample := 0; sample < 12; sample++ {
		s, err := newCatanCaravansShoresExtended(4)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if len(g.Tiles) != 42 || len(g.Caravans.Map.WateringHoles) != 1 || len(g.Caravans.Map.Starts) != 3 || len(g.Ports) != 9 || g.Paired != nil {
			t.Fatal("four player recipe")
		}
		for _, id := range caravanShoresMainlandIDs(4) {
			if g.Seafarers.Islands[id] != g.Seafarers.StartIslands[0] {
				t.Fatal("mainland embedded outside home")
			}
		}
		for name, mutate := range map[string]func(*Catan){
			"outer terrain": func(g *Catan) { g.Tiles[0].Resource = 11 },
			"outer number":  func(g *Catan) { g.Tiles[0].Number = 7 },
			"sea":           func(g *Catan) { g.Tiles[2].Resource = 0 },
			"water":         func(g *Catan) { g.Tiles[g.Caravans.Map.WateringHoles[0]].Resource = 0 },
			"number recipe": func(g *Catan) { g.Caravans.Map.NumberRecipe = CatanExtendedNumberRecipe },
			"supply":        func(g *Catan) { g.Caravans.Map.Supply = 33 },
		} {
			b := clone(*s)
			mutate(b.Catan)
			if b.validateCaravans() == nil {
				t.Fatal("accepted", name)
			}
		}
	}
}

func TestCatanCaravansShoresValidationDoesNotMutate(t *testing.T) {
	for _, n := range []int{2, 4, 6} {
		s, err := NewCatanCaravansShoresSeafarers(n)
		if err != nil {
			t.Fatal(err)
		}
		before := clone(*s)
		for i := 0; i < 8; i++ {
			if err = s.validateCaravans(); err != nil {
				t.Fatal(n, err)
			}
		}
		if !reflect.DeepEqual(before, *s) {
			t.Fatal("validation changed live state", n)
		}
	}
}
