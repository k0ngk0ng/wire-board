package game

import (
	"fmt"
	"testing"
)

func TestCatanSeafarersHelperEventGoldBeforeHilda(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, productive := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/gold=%v", n, productive), func(t *testing.T) {
				s := seaReferenceGameHelpers(t, n, "shores", true, true)
				g := s.Catan
				hilda := (s.Turn + 1) % n
				goldOwner := hilda
				if !productive {
					goldOwner = (hilda + 1) % n
				}
				eventAssignHelper(t, s, hilda, 3)
				// Explicit isolated midgame: one gold city and no ordinary
				// production. Resource/development/helper stocks stay intact.
				for v := range g.Vertices {
					g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
				}
				gold := -1
				for i, tile := range g.Tiles {
					if tile.Number == 2 {
						g.Tiles[i].Number = 3
					}
					if tile.Resource == CatanGold {
						gold = i
					}
				}
				if gold < 0 {
					t.Fatal("no gold on map")
				}
				g.Tiles[gold].Number = 2
				v := g.Tiles[gold].Vertices[0]
				g.Vertices[v].Owner, g.Vertices[v].Level = goldOwner, 2
				s.catanScores()
				before := sum(g.Players[hilda].Resources)
				referenceEventTop(t, s, "plentiful_year")
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				bad := clone(*s)
				bad.Catan.GoldPending = &CatanGoldPending{Claims: []CatanGoldClaim{{Player: goldOwner, Count: 2}}, Resume: "catan_turn"}
				helperReject(t, &bad, ckActor(&bad), Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}})
				for s.Catan.CardEvent != nil {
					if s.Catan.HelperPending != nil || s.Catan.GoldPending != nil {
						t.Fatal("production/helper response preceded event rewards")
					}
					helperReject(t, s, hilda, Action{Type: "catan_helper_choice", Color: 0})
					helperApply(t, s, ckActor(s), Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}})
				}
				if s.Catan.HelperPending != nil || s.CatanPendingActor() != goldOwner || s.Catan.GoldPending == nil {
					t.Fatal("Hilda must wait for all gold choices")
				}
				s = referenceEventRestore(t, s)
				helperApply(t, s, goldOwner, Action{Type: "catan_gold", Take: []int{0, 2, 0, 0, 0}})
				if productive {
					if s.Catan.HelperPending != nil || sum(s.Catan.Players[hilda].Resources)-before != 3 {
						t.Fatal("gold income must suppress Hilda compensation")
					}
				} else {
					q := s.Catan.HelperPending
					if q == nil || q.Player != hilda || q.Kind != "resource" || !q.Optional || sum(s.Catan.Players[hilda].Resources)-before != 1 {
						t.Fatal("event reward must not suppress Hilda compensation")
					}
					s = referenceEventRestore(t, s)
					corrupt := clone(*s)
					corrupt.Catan.GoldPending = &CatanGoldPending{Claims: []CatanGoldClaim{{Player: goldOwner, Count: 2}}, Resume: "catan_turn"}
					helperReject(t, &corrupt, hilda, Action{Type: "catan_helper_choice", Color: 2})
					helperApply(t, s, hilda, Action{Type: "catan_helper_choice", Color: 2})
					helperApply(t, s, hilda, Action{Type: "catan_helper_choice", Choice: "flip"})
					if sum(s.Catan.Players[hilda].Resources)-before != 2 {
						t.Fatal("wrong compensation amount")
					}
				}
				if s.Phase != "catan_turn" || s.Catan.RollID != 1 || s.Catan.GoldPending != nil || s.Catan.HelperPending != nil {
					t.Fatal("gold/helper chain did not resume original action")
				}
				seaEventConserved(t, s)
				referenceEventRestore(t, s)
			})
		}
	}
}

func TestCatanSeafarersHelperEventPairedUseAndFledRobber(t *testing.T) {
	s := seaReferenceGameHelpers(t, 6, "shores", true, true)
	referenceEventTop(t, s, "robber_flees")
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	s = resolveTwoReferenceProduction(t, s)
	secondary := s.Catan.Paired.Secondary
	eventAssignHelper(t, s, secondary, 11)
	helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	if s.Turn != secondary || !s.Catan.Paired.Second {
		t.Fatal("missing paired action")
	}
	before := sum(s.Catan.Players[secondary].Resources)
	helperApply(t, s, secondary, Action{Type: "catan_helper", Color: 0})
	helperApply(t, s, secondary, Action{Type: "catan_helper_choice", Choice: "flip"})
	if s.Catan.RollID != 1 || sum(s.Catan.Players[secondary].Resources)-before != 1 {
		t.Fatal("secondary helper must work without drawing another event")
	}
	helperReject(t, s, secondary, Action{Type: "catan_roll"})
	referenceEventRestore(t, s)

	// Three-player Shores has no desert: fleeing moves the robber to the
	// frame. Kaja cannot claim an invented terrain resource from there.
	var err error
	s, err = newCatanSeafarersReferenceEventsOptions(3, CatanOptions{Helpers: true, AllHelpers: true}, CatanSeafarersSetup{Scenario: "shores"})
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		actor := ckActor(s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	if len(s.Catan.fleeDeserts()) != 0 {
		t.Fatal("fixture needs a no-desert map")
	}
	referenceEventTop(t, s, "robber_flees")
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	s = resolveTwoReferenceProduction(t, s)
	if s.Catan.Robber != -1 {
		t.Fatal("robber did not leave board")
	}
	for _, id := range []int{10, 11} {
		eventAssignHelper(t, s, s.Turn, id)
		helperReject(t, s, s.Turn, Action{Type: "catan_helper", Color: 0})
	}
}
