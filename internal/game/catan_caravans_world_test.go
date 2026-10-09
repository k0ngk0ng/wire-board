package game

import (
	"fmt"
	"testing"
)

func TestCatanCaravansWorldNatural(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanCaravansWorld(n)
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

func TestCatanCaravansWorldRecipe(t *testing.T) {
	for _, n := range []int{3, 6} {
		s, err := newCatanCaravansWorld(n)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if len(g.worldPortEdges()) < 3*len(g.newWorld().Ports)-2 {
			t.Fatal("port capacity")
		}
		for _, id := range g.Caravans.Map.WateringHoles {
			if g.Caravans.WorldBase.Hexes[id].Resource != CatanSea || g.Tiles[id].Resource != catanWateringHole || g.Tiles[id].Number != 0 {
				t.Fatal("water source altered production")
			}
		}
		for _, mutate := range []func(*Catan){func(g *Catan) { g.Caravans.Map.Supply++ }, func(g *Catan) { g.Caravans.WorldBase.Hexes[0].Resource = 99 }, func(g *Catan) { g.Caravans.Map.WateringHoles[0]++ }, func(g *Catan) { g.Seafarers.NewWorld.Ports[0] = 99 }, func(g *Catan) { g.Seafarers.NewWorld.Index++ }} {
			b := clone(*s)
			mutate(b.Catan)
			if b.validateCaravans() == nil {
				t.Fatal("corrupt recipe accepted")
			}
		}
	}
}

func TestCatanCaravansWorldPrepared(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			layout, err := GenerateCatanCaravansWorldMap(n)
			if err != nil {
				t.Fatal(err)
			}
			s, err := NewCatanCaravansWorldWithMap(n, layout)
			if err != nil {
				t.Fatal(err)
			}
			for id, h := range layout.Hexes {
				tile := s.Catan.Tiles[id]
				if tile.Resource == catanWateringHole {
					if h.Resource != CatanSea {
						t.Fatal("source")
					}
				} else if tile.Resource != h.Resource || tile.Number != h.Number {
					t.Fatal("draft changed")
				}
			}
			layout.Hexes[0].Resource = 99
			if s.Catan.Caravans.WorldBase.Hexes[0].Resource == 99 {
				t.Fatal("draft alias")
			}
			if ValidateCatanCaravansWorldMap(n, layout) == nil {
				t.Fatal("bad draft accepted")
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
				if step == 83 {
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
		})
	}
}
