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
	for _, n := range []int{3, 4, 5, 6} {
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

func TestCatanCaravansIslandsPrintedPortsAndSeaDisc(t *testing.T) {
	s, err := newCatanCaravansIslands(3)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	if g.Tiles[19].Resource != CatanSea || g.Tiles[19].Number != 0 {
		t.Fatal("stray sea disc became productive land")
	}
	for _, want := range []struct{ tile, side, resource int }{{31, 0, 2}, {24, 3, -1}, {13, 1, 1}} {
		edge := catanFishingSide(g, want.tile, want.side)
		if !slices.ContainsFunc(g.Ports, func(p CatanPort) bool { return p.Edge == edge && p.Resource == want.resource }) {
			t.Fatal("printed port", want)
		}
	}
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	for _, v := range g.Tiles[19].Vertices {
		g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
	}
	before := append([]int{}, g.Players[0].Resources...)
	// Compare the same state without adjacent ordinary producing tiles: only
	// the former sea-disc location could otherwise cause a spurious reward.
	for i := range g.Tiles {
		if i != 19 {
			g.Tiles[i].Number = 0
		}
	}
	if err = s.catanRollProduction(4); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before, g.Players[0].Resources) {
		t.Fatal("sea generated resources")
	}
}

func TestCatanCaravansSixIslandsRecipe(t *testing.T) {
	for _, n := range []int{5, 6} {
		s, err := NewCatanCaravansIslandsSeafarers(n)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if g.Tiles[42].Resource != 3 || g.Tiles[52].Resource != 2 {
			t.Fatal("transfer resources", g.Tiles[42], g.Tiles[52])
		}
		for _, mutate := range []func(*Catan){func(g *Catan) { g.Caravans.ExtraNumbers[0].Number = 6 }, func(g *Catan) { g.Caravans.Map.Supply = 22 }, func(g *Catan) { g.Tiles[16].Resource = 3 }, func(g *Catan) { g.Ports[0].Resource = 99 }} {
			b := clone(*s)
			mutate(b.Catan)
			if b.validateCaravans() == nil {
				t.Fatal("invalid expanded recipe accepted")
			}
		}
		regions := map[int]bool{}
		for _, r := range g.Seafarers.Islands {
			if r >= 0 {
				regions[r] = true
			}
		}
		if len(regions) != 6 || len(g.Caravans.Map.Starts) != 6 || g.Caravans.Map.Supply != 33 {
			t.Fatal("six island recipe", len(regions))
		}
	}
}
