package game

import (
	"slices"
	"testing"
)

func TestCatanCaravansShoresThreePrinted(t *testing.T) {
	s, err := newCatanCaravansShoresThree()
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	if g.Tiles[22].Resource != catanWateringHole || g.Tiles[22].Number != 0 || g.Tiles[15].Resource != 2 || g.Tiles[15].Number != 9 || g.Robber != 0 || g.Tiles[0].Resource != 1 || g.Tiles[0].Number != 12 || g.victoryTarget() != 16 {
		t.Fatal("printed recipe")
	}
	if !slices.Equal(g.Caravans.Map.WateringHoles, []int{22}) || len(g.Caravans.Map.Starts) != 3 {
		t.Fatal("water source")
	}
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	g.Robber = -1
	tile := g.Tiles[0]
	g.Vertices[tile.Vertices[0]].Owner, g.Vertices[tile.Vertices[0]].Level = 0, 1
	for _, number := range []int{2, 12} {
		before := g.Players[0].Resources[1]
		if err = s.catanRollProduction(number); err != nil {
			t.Fatal(err)
		}
		if g.Players[0].Resources[1] != before+1 {
			t.Fatal("double production")
		}
	}
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Tiles[15].Number = 2 }, func(g *Catan) { g.Caravans.ExtraNumbers = nil }, func(g *Catan) { g.Seafarers.Variable = true }, func(g *Catan) { g.Caravans.Map.Starts[0].Edge = -1 }} {
		b := clone(*s)
		mutate(b.Catan)
		if b.validateCaravans() == nil {
			t.Fatal("invalid accepted")
		}
	}
}
