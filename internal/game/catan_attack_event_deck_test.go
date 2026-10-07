package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func attackReferenceGame(t *testing.T, n int, setup bool) *State {
	t.Helper()
	s, err := newCatanAttackReferenceEvents(n)
	if err != nil {
		t.Fatal(err)
	}
	if setup {
		finishAttackSetup(t, s)
	}
	return s
}

func TestCatanAttackEventDeckAllFaces(t *testing.T) {
	for _, n := range []int{3, 6} {
		for kind := range catanCardEventNames {
			t.Run(fmt.Sprintf("%d/%s", n, kind), func(t *testing.T) {
				s := attackReferenceGame(t, n, true)
				id := referenceEventTop(t, s, kind)
				owner := s.Turn
				helperReject(t, s, (owner+1)%n, Action{Type: "catan_roll"})
				helperApply(t, s, owner, Action{Type: "catan_roll"})
				if s.Catan.RevealedEvent.Kind != kind || !slices.Equal(s.Catan.EventDeck.Deck.Discard, []int{id}) {
					t.Fatal("wrong actual face")
				}
				for step := 0; s.Phase != "catan_turn" && step < 30; step++ {
					s = referenceEventRestore(t, s)
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					helperReject(t, s, actor, Action{Type: "catan_roll"})
					helperApply(t, s, actor, a)
				}
				if s.Phase != "catan_turn" || s.Turn != owner || s.Catan.RollID != 1 || !s.Catan.RevealedEvent.ProductionStarted || s.Catan.Robber != -1 {
					t.Fatal("draw continuation failed")
				}
				if s.Catan.Attack.Sequence != 0 {
					t.Fatal("production triggered a building landing")
				}
				if err := s.validateCatanAttack(); err != nil {
					t.Fatal(err)
				}
				s = referenceEventRestore(t, s)
				helperApply(t, s, s.Turn, Action{Type: "catan_end"})
				for step := 0; s.Phase == "catan_attack_end" && step < 30; step++ {
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					helperApply(t, s, actor, a)
				}
				if n == 6 {
					if !s.Catan.Paired.Second || s.Phase != "catan_turn" || s.Catan.RollID != 1 {
						t.Fatal("secondary drew a second card")
					}
					helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
				}
			})
		}
	}
}

func TestCatanAttackEventDeckConflictActualDrawAndCorruption(t *testing.T) {
	s := attackReferenceGame(t, 3, true)
	leader := (s.Turn + 1) % 3
	target := (s.Turn + 2) % 3
	attackBattleKnights(s, 0, leader, leader)
	// Keep exactly one steal target; exercise an out-of-turn mandatory response.
	for p := range s.Catan.Players {
		attackHand(s, p, []int{0, 0, 0, 0, 0})
	}
	attackHand(s, target, []int{0, 1, 0, 0, 0})
	referenceEventTop(t, s, "conflict")
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	if s.CatanPendingActor() != leader || s.Catan.CardEvent.Optional {
		t.Fatal("conflict did not use scenario knight leader")
	}
	s = referenceEventRestore(t, s)
	for _, damage := range []func(*State){
		func(x *State) { x.Catan.CardEvent.Optional = true },
		func(x *State) { x.Catan.CardEvent.Players = []int{target} },
		func(x *State) { x.Catan.Attack.Knights = nil },
		func(x *State) { x.Catan.Bank[0]++ },
		func(x *State) { x.Catan.EventDeck.Deck.Discard[0] = 0 },
	} {
		bad := clone(*s)
		damage(&bad)
		helperReject(t, &bad, leader, Action{Type: "catan_event_steal", Target: target})
	}
	helperReject(t, s, leader, Action{Type: "catan_event_skip"})
	helperApply(t, s, leader, Action{Type: "catan_event_steal", Target: target})
	if s.Catan.Players[leader].Resources[1] < 1 || s.Catan.RollID != 1 || !s.Catan.RevealedEvent.ProductionStarted {
		t.Fatal("conflict theft or production failed")
	}
}

func TestCatanAttackEventDeckNaturalMatches(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := attackReferenceGame(t, n, false)
			draws, landings, battles, ends := 0, 0, 0, 0
			for step := 0; !s.Finished && step < 6000; step++ {
				actor := ckActor(s)
				previous := s.Catan.RollID
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatalf("step=%d phase=%s: %v", step, s.Phase, err)
				}
				// Hidden reference order cannot influence a bot or any player's view.
				if a.Type == "catan_roll" {
					other := clone(*s)
					pile := other.Catan.EventDeck.Deck.DrawPile
					pile[0], pile[1] = pile[1], pile[0]
					b, err := other.BotAction(actor)
					if err != nil || !reflect.DeepEqual(a, b) {
						t.Fatal("bot read hidden event order")
					}
					for viewer := -1; viewer < n; viewer++ {
						if !reflect.DeepEqual(s.View(viewer), other.View(viewer)) {
							t.Fatal("hidden event order leaked")
						}
					}
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatalf("step=%d phase=%s action=%+v: %v", step, s.Phase, a, err)
				}
				if a.Type == "catan_roll" {
					draws++
				} else if s.Catan.RollID != previous {
					t.Fatal("scenario dice consumed an event card")
				}
				if s.Catan.RollID != draws {
					t.Fatal("draw count mismatch")
				}
				if err = s.validateCatanAttack(); err != nil {
					t.Fatal(err)
				}
				if err = s.validateCatanEventSession(); err != nil {
					t.Fatal(err)
				}
				landings = s.Catan.Attack.Sequence
				if s.Catan.Attack.EndSequence != ends {
					ends = s.Catan.Attack.EndSequence
					battles += len(s.Catan.Attack.End.Battles)
				}
				if step%31 == 0 || s.Catan.CardEvent != nil {
					s = referenceEventRestore(t, s)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 12 {
				t.Fatal("no natural victory", s.Round, s.Phase)
			}
			if draws == 0 || landings == 0 {
				t.Fatal("match missed real production/building landings")
			}
			if s.View(-1)["catan"].(map[string]any)["eventDeck"] == nil {
				t.Fatal("missing public deck")
			}
			t.Logf("%d players: %d event draws, %d landings, %d battles, %d cycles", n, draws, landings, battles, s.Catan.EventDeck.Deck.Cycle)
		})
	}
}

// Keep buildings fixed to reach the 31/32 boundary independently of victory.
// Actual response actions and paired/end-of-turn transitions are still used.
func TestCatanAttackEventDeckNewYearPairedBoundary(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := attackReferenceGame(t, n, true)
			for draw := 1; draw <= 33; draw++ {
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				s = referenceEventRestore(t, s)
				if s.Catan.EventDeck.Deck.Cycle != uint64((draw-1)/31+1) || len(s.Catan.EventDeck.Deck.Discard) != (draw-1)%31+1 {
					t.Fatal("incorrect New Year boundary", draw)
				}
				// Finish all card responses and any paired second action, without buying.
				for steps := 0; s.Phase != "catan_roll" && steps < 40; steps++ {
					actor := ckActor(s)
					action := Action{Type: "catan_end"}
					if s.Phase != "catan_turn" {
						var err error
						action, err = s.BotAction(actor)
						if err != nil {
							t.Fatal(err)
						}
					} else if s.Catan.Paired != nil && s.Catan.Paired.Second {
						helperReject(t, s, actor, Action{Type: "catan_roll"})
					}
					helperApply(t, s, actor, action)
					s = referenceEventRestore(t, s)
					if s.Catan.RollID != draw {
						t.Fatal("continuation consumed a card")
					}
				}
				if s.Phase != "catan_roll" || s.Catan.Attack.Sequence != 0 {
					t.Fatal("stalled or spurious landing")
				}
			}
		})
	}
}
