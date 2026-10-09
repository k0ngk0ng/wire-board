package game

import (
	"fmt"
	"testing"
)

func TestCatanCaravansDesertSeaNatural(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := newCatanCaravansDesertSea(n)
			if err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 18000 && !s.Finished; step++ {
				p := twoFullActor(s)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, err)
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				if step%137 == 0 {
					b := clone(*s)
					s = &b
					if err = s.validateCaravans(); err != nil {
						t.Fatal(err)
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

func TestCatanCaravansSeaRoutesAndProduction(t *testing.T) {
	for _, n := range []int{3, 4} {
		s, err := newCatanCaravansDesertSea(n)
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if g.victoryTarget() != 16 {
			t.Fatal("target")
		}
		// Follow a legal wagon path until a pure sea edge is reached; the pirate
		// must not block the merchant train, and a colocated ship has weight two.
		type node struct{ c catanCaravans }
		queue := []node{{*g.Caravans}}
		seen := map[int]bool{}
		found := false
		for len(queue) > 0 && !found {
			c := queue[0].c
			queue = queue[1:]
			for _, choice := range c.choices(g) {
				if seen[choice.Edge] {
					continue
				}
				seen[choice.Edge] = true
				next := c
				next.Wagons = append(append([]catanCaravanWagon{}, c.Wagons...), choice)
				e := g.Edges[choice.Edge]
				sea := len(e.Tiles) > 0
				for _, id := range e.Tiles {
					sea = sea && g.Tiles[id].Resource == CatanSea
				}
				if sea {
					g.Caravans = &next
					g.Seafarers.Pirate = e.Tiles[0]
					if err = next.validate(g); err != nil {
						t.Fatal(err)
					}
					g.Edges[e.ID].Owner = 0
					g.Edges[e.ID].Ship = true
					if g.roadLength(0) != 2 {
						t.Fatal("ship not doubled")
					}
					found = true
					break
				}
				queue = append(queue, node{next})
			}
		}
		if !found {
			t.Fatal("no merchant sea path")
		}
		s, err = newCatanCaravansDesertSea(n)
		if err != nil {
			t.Fatal(err)
		}
		g = s.Catan
		g.SetupStep = g.SetupLimit()
		s.Phase = "catan_turn"
		s.Turn = 0
		extra := g.Caravans.ExtraNumbers[0]
		tile := g.Tiles[extra.Tile]
		g.Robber = -1
		g.Vertices[tile.Vertices[0]].Owner = 0
		g.Vertices[tile.Vertices[0]].Level = 2
		for _, number := range []int{tile.Number, extra.Number} {
			before := g.Players[0].Resources[tile.Resource]
			if err = s.catanRollProduction(number); err != nil {
				t.Fatal(err)
			}
			if g.Players[0].Resources[tile.Resource] != before+2 {
				t.Fatal("extra disc production")
			}
		}
		for _, mutate := range []func(*Catan){func(g *Catan) { g.Caravans.Sea = "unknown" }, func(g *Catan) { g.Caravans.ExtraNumbers[0].Number = 7 }, func(g *Catan) { g.Seafarers.VictoryPoints = 14 }, func(g *Catan) { g.Caravans.Map.Starts[0].Edge = -1 }} {
			b := clone(*s)
			mutate(b.Catan)
			if b.validateCaravans() == nil {
				t.Fatal("corrupt accepted")
			}
		}
	}
}

func TestCatanCaravansSeaRejectsRobberAndForeignRecipe(t *testing.T) {
	for _, tribe := range []bool{false, true} {
		s, err := newCatanCaravansDesertSea(3)
		if tribe {
			s, err = newCatanCaravansTribeSea(3)
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, tile := range s.Catan.Tiles {
			illegal := tile.Resource == CatanSea || tribe && !s.Catan.tribeLand(tile.ID)
			if !illegal {
				continue
			}
			b := clone(*s)
			b.Catan.Robber = tile.ID
			if b.validateCaravans() == nil {
				t.Fatal("accepted robber", tribe, tile.ID)
			}
		}
		b := clone(*s)
		b.Catan.Seafarers.NumberRecipe = "foreign"
		if b.validateCaravans() == nil {
			t.Fatal("foreign number rules")
		}
	}
}
