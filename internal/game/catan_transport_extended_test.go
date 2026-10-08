package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func transportExtendedOpening(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanTransport(n)
	if err != nil {
		t.Fatal(err)
	}
	for steps := 0; s.Catan.setup() && steps < 60; steps++ {
		a, err := s.BotAction(s.Turn)
		if err == nil {
			err = s.Apply(s.Turn, a)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.Phase != "catan_roll" {
		t.Fatal("setup incomplete")
	}
	return s
}

func TestCatanTransportExtendedProductionAndPair(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := transportExtendedOpening(t, n)
			// Transfer a starting village to an actual 2/12 hex in this production
			// fixture; no change to number discs, resource stock or piece counts.
			for _, total := range []int{2, 12} {
				trial := clone(*s)
				g := trial.Catan
				tile, vertex := -1, -1
				for i, h := range g.Tiles {
					if h.Number == total {
						tile = i
						break
					}
				}
				if tile < 0 {
					t.Fatal("missing production number", total)
				}
				for i, v := range g.Vertices {
					if slices.Contains(g.Tiles[tile].Vertices, i) && v.Owner == -1 && g.Transport.Map.siteAt(i) < 0 {
						vertex = i
						break
					}
				}
				if vertex < 0 {
					t.Fatal("no producing vertex")
				}
				for i := range g.Vertices {
					if g.Vertices[i].Owner == trial.Turn && g.Vertices[i].Level == 1 {
						g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
						break
					}
				}
				g.Vertices[vertex].Owner, g.Vertices[vertex].Level = trial.Turn, 1
				resource := g.Tiles[tile].Resource
				before := g.Players[trial.Turn].Resources[resource]
				rolls := 0
				err := trial.catanTransportRoll(func() [2]int {
					rolls++
					if rolls > 1 {
						t.Fatal("extended 2/12 was rerolled")
					}
					return [2]int{total / 2, total / 2}
				})
				if err != nil || rolls != 1 || g.RollID != 1 || trial.Phase != "catan_turn" || g.Players[trial.Turn].Resources[resource] <= before {
					t.Fatal("extended production failed", total, err)
				}
			}
			g := s.Catan
			primary, secondary, turnSerial := s.Turn, g.Paired.Secondary, g.TurnSerial
			if err := s.catanTransportRoll(func() [2]int { return [2]int{2, 3} }); err != nil {
				t.Fatal(err)
			}
			finish := func() {
				t.Helper()
				if err := s.Apply(s.Turn, Action{Type: "catan_end"}); err != nil {
					t.Fatal(err)
				}
				if s.Phase != "catan_transport_move" {
					t.Fatal("wagon movement skipped")
				}
				s = transportRestoreState(t, s)
				if err := s.Apply(s.Turn, Action{Type: "catan_transport_stop", Offer: int(s.Catan.Transport.Sequence)}); err != nil {
					t.Fatal(err)
				}
			}
			// Each marker receives a fresh resource-purchase and movement budget.
			s.Catan.Transport.Bought, s.Catan.Transport.Swift = 2, false
			finish()
			g = s.Catan
			if s.Turn != secondary || !g.Paired.Second || s.Phase != "catan_turn" || g.TurnSerial != turnSerial || g.Transport.Active != secondary || g.Transport.Bought != 0 || g.Transport.Moves != 0 || g.Transport.Travel != nil {
				t.Fatal("secondary turn did not start cleanly")
			}
			before := clone(*s)
			for _, action := range []Action{{Type: "catan_roll"}, {Type: "catan_trade_offer", Give: []int{1, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}}} {
				if s.Apply(secondary, action) == nil || !reflect.DeepEqual(before, *s) {
					t.Fatal("secondary production/trade allowed or state changed")
				}
			}
			// Swift Journey's second movement must finish before advancing the markers.
			s.Catan.Transport.Swift = true
			finish()
			if s.Turn != secondary || s.Phase != "catan_transport_move" || s.Catan.Transport.Moves != 2 {
				t.Fatal("swift lost second move")
			}
			if err := s.Apply(secondary, Action{Type: "catan_transport_stop", Offer: int(s.Catan.Transport.Sequence)}); err != nil {
				t.Fatal(err)
			}
			g = s.Catan
			if g.Paired.Second || s.Turn != (primary+1)%n || s.Phase != "catan_roll" || g.TurnSerial != turnSerial+1 || g.Transport.Swift || g.Transport.Bought != 0 || g.Transport.Moves != 0 {
				t.Fatal("next primary/reset incorrect")
			}
			transportRestoreState(t, s)
		})
	}
}

func TestCatanTransportExtendedRecipeCorruption(t *testing.T) {
	for _, n := range []int{3, 5, 6} {
		s, err := NewCatanTransport(n)
		if err != nil {
			t.Fatal(err)
		}
		if (s.Catan.Transport.DeckRecipe != "") != (n > 4) {
			t.Fatal("wrong deck tag")
		}
		transportRestoreState(t, s)
		corruptions := []func(*State){
			func(x *State) { x.Catan.Transport.DeckRecipe = "unknown" },
			func(x *State) { x.Catan.DevDeck = x.Catan.DevDeck[1:] },
			func(x *State) { x.Catan.Options.Helpers = true },
		}
		if n > 4 {
			corruptions = append(corruptions, func(x *State) { x.Catan.Paired = nil }, func(x *State) { x.Catan.Paired.Secondary = n }, func(x *State) { x.Catan.Transport.DeckRecipe = "" })
		}
		for _, corrupt := range corruptions {
			trial := clone(*s)
			corrupt(&trial)
			before := clone(trial)
			if trial.Apply(trial.Turn, Action{Type: "catan_end"}) == nil || !reflect.DeepEqual(before, trial) {
				t.Fatal("corrupt state accepted or mutated", n)
			}
		}
	}
}
