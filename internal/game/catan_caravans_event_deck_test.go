package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func caravanReferenceGame(t *testing.T, n int, setup bool) *State {
	t.Helper()
	s, err := newCatanCaravansReferenceEvents(n)
	if err != nil {
		t.Fatal(err)
	}
	for setup && s.Catan.setup() {
		actor := ckActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	return s
}

func TestCatanCaravansEventDeckAllFaces(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for kind := range catanCardEventNames {
			t.Run(fmt.Sprintf("%d/%s", n, kind), func(t *testing.T) {
				s := caravanReferenceGame(t, n, true)
				// Move the robber onto a producing hex so flee cannot pass as a no-op.
				if kind == "robber_flees" {
					s.Catan.Robber = 0
				}
				referenceEventTop(t, s, kind)
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				s = resolveTwoReferenceProduction(t, s)
				if s.Catan.RollID != 1 || s.Catan.RevealedEvent.Kind != kind || !s.Catan.RevealedEvent.ProductionStarted {
					t.Fatal("wrong production")
				}
				if kind == "robber_flees" && s.Catan.Robber != -1 {
					t.Fatal("watering hole is not a desert")
				}
				if s.Catan.Caravans.Built || s.Catan.Caravans.Pending != nil || len(s.Catan.Caravans.Wagons) != 0 {
					t.Fatal("event created a construction reward")
				}
				caravanConserved(t, s)
				s = referenceEventRestore(t, s)
			})
		}
	}
}

func TestCatanCaravansEventDeckNaturalMatches(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := caravanReferenceGame(t, n, false)
			draws, bids, placements := 0, 0, 0
			for step := 0; !s.Finished && step < 8000; step++ {
				actor := ckActor(s)
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatalf("step=%d phase=%s: %v", step, s.Phase, err)
				}
				beforeDraws := s.Catan.RollID
				if a.Type == "catan_roll" {
					other := clone(*s)
					pile := other.Catan.EventDeck.Deck.DrawPile
					pile[0], pile[1] = pile[1], pile[0]
					b, err := other.BotAction(actor)
					if err != nil || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(s.View(-1), other.View(-1)) {
						t.Fatal("private pile changed bot or public view")
					}
					draws++
				}
				if a.Type == "catan_caravan_bid" {
					bids++
				}
				if a.Type == "catan_caravan_place" {
					placements++
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatalf("step=%d phase=%s action=%+v: %v", step, s.Phase, a, err)
				}
				if a.Type != "catan_roll" && s.Catan.RollID != beforeDraws {
					t.Fatal("nonproduction action drew a card")
				}
				caravanConserved(t, s)
				if err = s.validateCatanEventSession(); err != nil {
					t.Fatal(err)
				}
				if step%29 == 0 || s.CatanPendingActor() >= 0 {
					s = referenceEventRestore(t, s)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 12 || s.Catan.Caravans.Pending != nil || draws == 0 || bids == 0 || placements == 0 {
				t.Fatal("incomplete natural game", draws, bids, placements)
			}
			t.Logf("%d players: %d draws / %d cycles, %d bids / %d placements", n, draws, s.Catan.EventDeck.Deck.Cycle, bids, placements)
		})
	}
}

func TestCatanCaravansEventDeckEarthquakeAndRepair(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			// Explicit connected three-road/two-wagon fixture; no artificial event result.
			s := caravanFixture(t, n)
			g, c := s.Catan, s.Catan.Caravans
			first := c.Map.Starts[0]
			if err := c.place(g, first); err != nil {
				t.Fatal(err)
			}
			joint := g.Edges[first.Edge].A + g.Edges[first.Edge].B - first.From
			var second catanCaravanWagon
			found := false
			for _, w := range c.choices(g) {
				if w.From == joint {
					second, found = w, true
					break
				}
			}
			if !found {
				t.Fatal("missing second wagon")
			}
			if err := c.place(g, second); err != nil {
				t.Fatal(err)
			}
			end := g.Edges[second.Edge].A + g.Edges[second.Edge].B - second.From
			third := -1
			for _, edge := range g.touching(end) {
				if edge != first.Edge && edge != second.Edge {
					third = edge
					break
				}
			}
			if third < 0 {
				t.Fatal("third road")
			}
			for _, edge := range []int{first.Edge, second.Edge, third} {
				g.Edges[edge].Owner = 0
			}
			g.Vertices[joint].Owner, g.Vertices[joint].Level = 0, 1
			s.catanScores()
			if g.Players[0].RoadLength != 5 || g.LongestOwner != 0 || g.Players[0].Score != 4 {
				t.Fatal("weighted fixture")
			}
			g.EventDeck = &catanEventSession{Catalogue: catanEventReferenceCatalogue, Deck: newCatanEventDeck()}
			s.Phase = "catan_roll"
			referenceEventTop(t, s, "earthquake")
			helperApply(t, s, 0, Action{Type: "catan_roll"})
			helperApply(t, s, 0, Action{Type: "catan_earthquake", Edge: first.Edge})
			g = s.Catan
			if !g.Edges[first.Edge].Damaged || g.Players[0].RoadLength != 5 || g.LongestOwner != 0 || g.Players[0].Score != 4 || len(g.Caravans.Wagons) != 2 {
				t.Fatal("damage removed wagon or its bonuses")
			}
			for color, want := range []int{1, 1, 0, 0, 0} {
				catanGive(g, 0, color, want-g.Players[0].Resources[color])
			}
			helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: first.Edge})
			g = s.Catan
			if g.Edges[first.Edge].Damaged || sum(g.Players[0].Resources) != 0 || g.Caravans.Built || len(g.Caravans.Wagons) != 2 {
				t.Fatal("repair gave a wagon or wrong cost")
			}
			helperApply(t, s, 0, Action{Type: "catan_end"})
			if s.Catan.Caravans.Pending != nil || s.Catan.RollID != 1 {
				t.Fatal("repair created vote or production")
			}
			s = referenceEventRestore(t, s)
			caravanConserved(t, s)
		})
	}
}

func TestCatanCaravansEventDeckConstructionAndPairedVotes(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := caravanReferenceGame(t, n, true)
			// Two identical production numbers remain two draws in the two-player variant.
			twoReferenceTop(t, s, 22, 23)
			for s.Phase == "catan_roll" {
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				s = resolveTwoReferenceProduction(t, s)
			}
			wantDraws := 1
			if n == 2 {
				wantDraws = 2
				if !slices.Equal(s.Catan.Two.Rolls, []int{8, 8}) {
					t.Fatal("redrew duplicate production")
				}
			}
			stages := 1
			if n == 6 {
				stages = 2
			}
			for stage := 0; stage < stages; stage++ {
				g := s.Catan
				owner := s.Turn
				vertex := -1
				for _, v := range g.Vertices {
					if v.Owner == owner && v.Level == 1 {
						vertex = v.ID
						break
					}
				}
				if vertex < 0 {
					t.Fatal("no starting village")
				}
				for color, want := range []int{0, 0, 0, 2, 3} {
					catanGive(g, owner, color, want-g.Players[owner].Resources[color])
				}
				helperApply(t, s, owner, Action{Type: "catan_city", Vertex: vertex})
				if !s.Catan.Caravans.Built {
					t.Fatal("actual upgrade did not earn wagon")
				}
				before := len(s.Catan.Caravans.Wagons)
				helperApply(t, s, owner, Action{Type: "catan_end"})
				bad := clone(*s)
				bad.Catan.CardEvent = &CatanCardEvent{Kind: "earthquake", Players: []int{owner}}
				if bad.validateCatanEventSession() == nil {
					t.Fatal("overlapping responders accepted")
				}
				for steps := 0; s.Catan.Caravans.Pending != nil && steps < 30; steps++ {
					actor := ckActor(s)
					helperReject(t, s, actor, Action{Type: "catan_roll"})
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					helperApply(t, s, actor, a)
					s = referenceEventRestore(t, s)
					caravanConserved(t, s)
					if s.Catan.RollID != wantDraws {
						t.Fatal("vote drew another card")
					}
				}
				wagons := 1
				if n == 2 {
					wagons = 2
				}
				if s.Catan.Caravans.Pending != nil || len(s.Catan.Caravans.Wagons) != before+wagons {
					t.Fatal("vote did not place expected wagons")
				}
				if n == 6 && stage == 0 && (!s.Catan.Paired.Second || s.Turn != s.Catan.Paired.Secondary || s.Phase != "catan_turn") {
					t.Fatal("missing secondary action")
				}
			}
			if s.Phase != "catan_roll" {
				t.Fatal("did not return to next primary production")
			}
		})
	}
}

func TestCatanCaravansEventDeckRejectsConflictingSavedResponses(t *testing.T) {
	original := caravanReferenceGame(t, 3, true)
	referenceEventTop(t, original, "earthquake")
	helperApply(t, original, original.Turn, Action{Type: "catan_roll"})
	if original.Catan.CardEvent == nil {
		t.Fatal("missing real earthquake queue")
	}
	for name, mutate := range map[string]func(*State){
		"built-before-production": func(s *State) { s.Catan.Caravans.Built = true },
		"missing-deck":            func(s *State) { s.Catan.EventDeck = nil },
		"simultaneous-vote":       func(s *State) { s.Catan.Caravans.Pending = &catanCaravanVote{Kind: "bid", Active: s.Turn} },
		"unverified-version":      func(s *State) { s.Catan.EventDeck.Catalogue = "2025-unverified" },
	} {
		t.Run(name, func(t *testing.T) {
			broken := clone(*original)
			mutate(&broken)
			actor := original.CatanPendingActor()
			a, err := original.BotAction(actor)
			if err != nil {
				t.Fatal(err)
			}
			helperReject(t, &broken, actor, a)
		})
	}
	// A normal public recipe must not gain an event just by calling its effect.
	plain, err := NewCatanCaravans(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for plain.Catan.setup() {
		a, err := plain.BotAction(plain.Turn)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, plain, plain.Turn, a)
	}
	before := clone(*plain)
	if err = plain.catanBeginCardEvent("beautiful_day", 8, 0, 0); err == nil || !reflect.DeepEqual(&before, plain) {
		t.Fatal("unrecorded event accepted or changed game")
	}
}
