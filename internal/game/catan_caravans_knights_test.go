package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func caravanKnightRestore(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCaravans(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	if next.Catan.Two != nil {
		if err = next.validateCatanTwo(); err != nil {
			t.Fatal(err)
		}
	}
	if err = next.validateCityProgressInventory(); err != nil {
		t.Fatal(err)
	}
	*s = next
}
func caravanKnightGame(t *testing.T, n int, events bool) *State {
	t.Helper()
	s, err := NewCatanCaravansCitiesKnights(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	if events {
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
	}
	if s.Catan.victoryTarget() != 15 || s.Catan.Robber != -1 || len(s.Catan.DevDeck) != 0 {
		t.Fatal("combination setup")
	}
	return s
}
func TestCatanCaravansKnightsNaturalGames(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s := caravanKnightGame(t, n, events)
				seen := map[string]int{}
				for step := 0; step < 12000 && !s.Finished; step++ {
					actor := twoFullActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						trial := clone(*s)
						endErr := trial.Apply(actor, Action{Type: "catan_end"})
						t.Fatalf("step=%d phase=%s bot=%v end=%v event=%+v city=%+v", step, s.Phase, err, endErr, s.Catan.RevealedEvent, s.Catan.CitiesKnights.Pending)
					}
					if err = s.Apply(actor, a); err != nil {
						t.Fatal(s.Phase, a, err)
					}
					seen[a.Type]++
					if step%67 == 0 {
						caravanKnightRestore(t, s)
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round, s.Phase, seen)
				}
				caravanKnightRestore(t, s)
				if seen["catan_caravan_bid"] == 0 {
					t.Fatal("no caravan bids", seen)
				}
				t.Log("rounds", s.Round, "actions", seen)
			})
		}
	}
}
func caravanKnightReady(t *testing.T, n int) *State {
	t.Helper()
	s := caravanKnightGame(t, n, false)
	finishFishHelperSetup(t, s)
	for s.Phase != "catan_turn" {
		twoSeaStep(t, s)
	}
	caravanKnightRestore(t, s)
	return s
}
func TestCatanCaravansKnightsBidsAndInvention(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := caravanKnightReady(t, n)
			g := s.Catan
			choices := g.inventionNumbers()
			left, right := choices[0], choices[1]
			for _, c := range choices {
				if c.Number != left.Number {
					right = c
					break
				}
			}
			if err := s.catanInvention(Action{Tile: left.Tile, Target: right.Tile}); err != nil {
				t.Fatal(err)
			}
			caravanKnightRestore(t, s)
			if len(s.Catan.Caravans.Map.NumberSwaps) != 1 || s.Catan.Tiles[left.Tile].Number != right.Number {
				t.Fatal("invention not applied")
			}
			bad := clone(*s)
			bad.Catan.Caravans.Map.NumberSwaps = nil
			if bad.validateCaravans() == nil {
				t.Fatal("missing swap ledger accepted")
			}
			bad = clone(*s)
			bad.Catan.Caravans.Knights = ""
			if bad.validateCaravans() == nil {
				t.Fatal("unmarked combination accepted")
			}
			g = s.Catan
			actor := s.Turn
			for color := 0; color < 4; color++ {
				g.Bank[color] -= 2
				g.Players[actor].Resources[color] += 2
			}
			g.Caravans.Built = true
			if !s.catanBeginCaravanVote() {
				t.Fatal("no vote")
			}
			before := clone(*s)
			if err := s.Apply(actor, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 1, 1, 0}}); err == nil {
				t.Fatal("old bid accepted")
			}
			if !slices.Equal(s.Catan.Players[actor].Resources, before.Catan.Players[actor].Resources) {
				t.Fatal("invalid bid mutated hand")
			}
			helperApply(t, s, actor, Action{Type: "catan_caravan_bid", Tokens: []int{1, 1, 0, 0, 0}})
			caravanKnightRestore(t, s)
			for s.Catan.Caravans.Pending != nil {
				twoSeaStep(t, s)
				caravanKnightRestore(t, s)
			}
			want := 1
			if n == 2 {
				want = 2
			}
			if len(s.Catan.Caravans.Wagons) != want {
				t.Fatal("wagon count")
			}
		})
	}
}

func TestCatanCaravansKnightsAlchemyAndEndDiscard(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := caravanKnightGame(t, n, true)
			finishFishHelperSetup(t, s)
			actor := s.Turn
			ckProgressGive(t, s, actor, 0)
			helperApply(t, s, actor, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 4}})
			for s.Phase != "catan_turn" {
				twoSeaStep(t, s)
			}
			// For two players the second production may be an ordinary event.
			ckProgressGive(t, s, actor, 1, 2, 3, 4, 5)
			s.Catan.Caravans.Built = true
			helperApply(t, s, actor, Action{Type: "catan_end"})
			if s.Phase != "catan_progress_end" || s.Catan.Caravans.Pending != nil {
				t.Fatal("end discard must precede vote")
			}
			caravanKnightRestore(t, s)
			twoSeaStep(t, s)
			if s.Phase != "catan_caravan_bid" || s.Turn != actor {
				t.Fatal("discard skipped vote", s.Phase)
			}
			caravanKnightRestore(t, s)
			for s.Catan.Caravans.Pending != nil {
				twoSeaStep(t, s)
			}
			if s.Turn == actor {
				t.Fatal("turn did not advance")
			}
			caravanKnightRestore(t, s)
		})
	}
}
