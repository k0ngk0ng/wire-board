package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Controlled effect states: the event deck and public recipe remain disabled.
func attackEventFixture(t *testing.T, n int) *State {
	t.Helper()
	s := newAttackState(t, n, false)
	s.Phase = "catan_roll"
	clear(s.Catan.Attack.Barbarians)
	return s
}

func TestCatanAttackEventConflictMandatoryPrivateAndRestored(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, mode := range []string{"manual", "bot", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s := attackEventFixture(t, n)
				turn, leader, target := s.Turn, (s.Turn+1)%n, (s.Turn+2)%n
				attackBattleKnights(s, 0, leader, leader, turn)
				attackHand(s, target, []int{0, 2, 0, 0, 0})
				// One known city production occurs only after the theft.
				tile := s.Catan.Tiles[0]
				vertex := tile.Vertices[0]
				s.Catan.Vertices[vertex].Owner, s.Catan.Vertices[vertex].Level = turn, 2
				s.catanScores()
				production := clone(*s)
				if err := production.catanRollProduction(tile.Number); err != nil {
					t.Fatal(err)
				}
				beginCardEvent(t, s, "conflict", tile.Number, 0, 0)
				if s.CatanPendingActor() != leader || s.Catan.CardEvent.Optional || s.Catan.RevealedEvent.ProductionStarted || sum(s.Catan.Players[turn].Resources) != 0 {
					t.Fatal("conflict eligibility/production order")
				}
				assertAttackRestored(t, s)
				for viewer := -1; viewer < n; viewer++ {
					v := s.View(viewer)["catan"].(map[string]any)
					targets := v["legal"].(map[string][]int)["eventTargets"]
					if (len(targets) == 1) != (viewer == leader) || viewer == leader && targets[0] != target {
						t.Fatal("private legal targets")
					}
					if v["cardEvent"].(map[string]any)["canSkip"] != false {
						t.Fatal("mandatory conflict offered skip")
					}
					for p, raw := range v["players"].([]any) {
						if (raw.(map[string]any)["resources"] != nil) != (p == viewer) {
							t.Fatal("hand exposed")
						}
					}
				}
				attackReject(t, s, leader, Action{Type: "catan_event_skip"})
				attackReject(t, s, turn, Action{Type: "catan_event_steal", Target: target})
				attackReject(t, s, leader, Action{Type: "catan_event_steal", Target: target, Take: []int{0, 1, 0, 0, 0}})
				if err := s.EliminateCatan(leader); err == nil {
					t.Fatal("removed required responder")
				}
				restored := clone(*s)
				s = &restored
				action := Action{Type: "catan_event_steal", Target: target}
				switch mode {
				case "bot":
					var err error
					action, err = s.BotAction(leader)
					if err != nil {
						t.Fatal(err)
					}
					other := clone(*s)
					slices.Reverse(other.Catan.Attack.Deck)
					attackHand(&other, target, []int{0, 0, 2, 0, 0})
					otherAction, err := other.BotAction(leader)
					if err != nil || !reflect.DeepEqual(action, otherAction) {
						t.Fatal("bot used hidden cards")
					}
					fallthrough
				case "manual":
					if err := s.Apply(leader, action); err != nil {
						t.Fatal(err)
					}
				case "timeout":
					s.AutoCatanPending()
				}
				if s.Phase != "catan_turn" || s.Turn != turn || s.Catan.CardEvent != nil || !s.Catan.RevealedEvent.ProductionStarted || s.Catan.RollID != 1 || s.Catan.Robber != -1 {
					t.Fatal("event did not finish once")
				}
				if !slices.Equal(s.Catan.Players[turn].Resources, production.Catan.Players[turn].Resources) || s.Catan.Players[leader].Resources[1] != 1 || s.Catan.Players[target].Resources[1] != 1 {
					t.Fatal("wrong production/theft")
				}
				attackReject(t, s, leader, action)
				assertAttackRestored(t, s)
			})
		}
	}
}

func TestCatanAttackEventConflictTiesAndNoVictims(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"zero", "tie", "empty", "eliminated"} {
			s := attackEventFixture(t, n)
			p := (s.Turn + 1) % n
			if kind != "empty" {
				// Give every live player a potential victim: a tied leader must
				// not be hidden by an unrelated empty-hand fast path.
				for player := range n {
					attackHand(s, player, []int{1, 0, 0, 0, 0})
				}
			}
			switch kind {
			case "tie":
				attackBattleKnights(s, 0, s.Turn, p)
			case "empty":
				attackBattleKnights(s, 0, p, p)
			case "eliminated":
				attackBattleKnights(s, 0, p, p)
				s.Catan.Players[p].Eliminated = true
			}
			beginCardEvent(t, s, "conflict", 6, 0, 0)
			if s.Phase != "catan_turn" || s.Catan.CardEvent != nil {
				t.Fatal("ineligible conflict stalled", kind)
			}
			assertAttackRestored(t, s)
		}
	}
}

func TestCatanAttackEventSevenDiscardsAndStealsOnce(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := attackEventFixture(t, n)
		p := s.Turn
		q := (p + 1) % n
		r := (p + 2) % n
		attackHand(s, p, []int{8, 0, 0, 0, 0})
		attackHand(s, q, []int{0, 8, 0, 0, 0})
		attackHand(s, r, []int{0, 0, 2, 0, 0})
		attackCoins(s, p, 80)
		beginCardEvent(t, s, "robber_attacks", 7, 0, 0)
		if s.Phase != "catan_discard" || s.Catan.DiscardDue[p] != 4 || s.Catan.DiscardDue[q] != 4 {
			t.Fatal("seven discard counts")
		}
		assertAttackRestored(t, s)
		helperApply(t, s, q, Action{Type: "catan_discard", Tokens: []int{0, 4, 0, 0, 0}})
		assertAttackRestored(t, s)
		helperApply(t, s, p, Action{Type: "catan_discard", Tokens: []int{4, 0, 0, 0, 0}})
		if s.Phase != "catan_steal" || !slices.Equal(s.Catan.Victims, s.Catan.cardTheftTargets(p)) {
			t.Fatal("seven did not offer global targets")
		}
		assertAttackRestored(t, s)
		helperApply(t, s, p, Action{Type: "catan_steal", Target: r})
		if s.Phase != "catan_turn" || s.Catan.Robber != -1 || s.Catan.Players[p].Resources[2] != 1 || s.Catan.Attack.Gold[p] != 80 || s.Catan.RollID != 1 {
			t.Fatal("seven stole twice or moved robber")
		}
		assertAttackRestored(t, s)
	}
}

func TestCatanAttackEventFleesNoEffectAndPairedContinuation(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := newAttackState(t, n, true)
		finishAttackSetup(t, s)
		p := s.Turn
		baseline := clone(*s)
		if err := baseline.catanRollProduction(6); err != nil {
			t.Fatal(err)
		}
		beginCardEvent(t, s, "robber_flees", 6, 0, 0)
		if s.Catan.Robber != -1 || s.Catan.CardEvent != nil || !reflect.DeepEqual(s.Catan.Players, baseline.Catan.Players) || s.Catan.Attack.Sequence != 0 {
			t.Fatal("flee changed board or production")
		}
		assertAttackRestored(t, s)
		helperApply(t, s, p, Action{Type: "catan_end"})
		if n == 6 {
			if !s.Catan.Paired.Second || s.Phase != "catan_turn" || s.Catan.RollID != 1 {
				t.Fatal("secondary action redrew")
			}
			rejectCardEvent(t, s, "beautiful_day", 6, 0, 0)
			helperApply(t, s, s.Turn, Action{Type: "catan_end"})
		}
		if s.Phase != "catan_roll" {
			t.Fatal("next production turn missing")
		}
		assertAttackRestored(t, s)
		helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
		if s.Catan.RevealedEvent != nil || s.Catan.RollID != 2 {
			t.Fatal("normal roll retained old face")
		}
		assertAttackRestored(t, s)
	}
}

func TestCatanAttackEventCorruptStateAndUnsupportedEffects(t *testing.T) {
	s := attackEventFixture(t, 6)
	leader := (s.Turn + 1) % 6
	target := (s.Turn + 2) % 6
	attackBattleKnights(s, 0, leader)
	attackHand(s, target, []int{0, 2, 0, 0, 0})
	rejectCardEvent(t, s, "new_year", 6, 0, 0) // Handled by the future draw pipeline, not a face effect.
	rejectCardEvent(t, s, "unknown", 6, 0, 0)
	rejectCardEvent(t, s, "robber_attacks", 6, 0, 0)
	beginCardEvent(t, s, "conflict", 6, 0, 0)
	for _, mutate := range []func(*State){
		func(x *State) { x.Catan.CardEvent.Optional = true },
		func(x *State) { x.Catan.CardEvent.Players = []int{target} },
		func(x *State) { x.Catan.CardEvent.Players = append(x.Catan.CardEvent.Players, leader) },
		func(x *State) { x.Catan.CardEvent.Kind = "earthquake" },
		func(x *State) { x.Catan.CardEvent.Red = 1 },
		func(x *State) { x.Catan.CardEvent.Production = 5 },
		func(x *State) { x.Catan.RevealedEvent = nil },
		func(x *State) { x.Catan.RevealedEvent.ProductionStarted = true },
		func(x *State) { x.Catan.RevealedEvent.RollID++ },
		func(x *State) { x.Catan.RevealedEvent.Face = 1 },
		func(x *State) { x.Catan.Paired.Second = true },
		func(x *State) { x.Phase = "catan_turn" },
	} {
		other := clone(*s)
		mutate(&other)
		if other.validateCatanAttack() == nil {
			t.Fatal("accepted corrupt combination")
		}
		attackReject(t, &other, leader, Action{Type: "catan_event_steal", Target: target})
	}
}
