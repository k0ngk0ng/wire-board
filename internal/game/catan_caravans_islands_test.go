package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanCaravansIslandsMap(t *testing.T) {
	for _, n := range []int{3, 4} {
		s, err := newCatanCaravansIslands(n)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if g.Tiles[17].Resource != catanWateringHole || g.victoryTarget() != 15 || len(g.Caravans.Map.Starts) != 3 {
			t.Fatal("recipe")
		}
		occupied := map[int]bool{}
		for _, p := range g.Ports {
			if !g.edgeTerrain(p.Edge, true) || !g.edgeTerrain(p.Edge, false) {
				t.Fatal(n, "noncoastal port", p)
			}
			e := g.Edges[p.Edge]
			for _, v := range []int{e.A, e.B} {
				if occupied[v] {
					t.Fatal("ports touching")
				}
				occupied[v] = true
			}
		}
		for _, w := range g.Caravans.Map.Starts {
			e := g.Edges[w.Edge]
			for _, id := range e.Tiles {
				if g.Tiles[id].Resource == catanWateringHole {
					t.Fatal("water source start follows source perimeter")
				}
			}
		}
		regions := map[int]bool{}
		for _, id := range g.Seafarers.Islands {
			if id >= 0 {
				regions[id] = true
			}
		}
		if len(regions) != 5 {
			t.Fatal("four islands plus watering hole", n, len(regions), g.Seafarers.Islands)
		}
		if n == 3 && !slices.Equal([]int{g.Tiles[13].Resource, g.Tiles[14].Resource, g.Tiles[32].Resource}, []int{1, 0, 0}) {
			t.Fatal("printed relocation")
		}
	}
}
func TestCatanCaravansIslandsNatural(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanCaravansIslands(n)
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
						t.Fatal(e)
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, s.Phase, e)
					}
					if step%137 == 0 {
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
}

func TestCatanCaravansIslandsCorruption(t *testing.T) {
	for _, n := range []int{3, 4} {
		s, err := newCatanCaravansIslands(n)
		if err != nil {
			t.Fatal(err)
		}
		for _, mutate := range []func(*Catan){func(g *Catan) { g.Tiles[17].Resource = CatanSea }, func(g *Catan) { g.Seafarers.Islands[17] = -1 }, func(g *Catan) { g.Ports[0].Resource = 9 }, func(g *Catan) { g.Caravans.Map.Starts[0].From = -1 }, func(g *Catan) { g.Seafarers.VictoryPoints = 13 }} {
			b := clone(*s)
			mutate(b.Catan)
			if b.validateCaravans() == nil {
				t.Fatal("invalid map")
			}
		}
	}
}
