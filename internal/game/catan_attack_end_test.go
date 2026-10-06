package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func attackPlanFixture(t *testing.T, n int) (*State, int, int, int) {
	t.Helper()
	s := newAttackState(t, n, false)
	g, a := s.Catan, s.Catan.Attack
	p := s.Turn
	from := catanFishingSide(g, a.Map.Castles[0], 0)
	a.Knights = []catanAttackKnight{{p, from}}
	short, long := -1, -1
	for edge, d := range a.knightDestinations(g, 0, 5) {
		if d == 3 {
			short = edge
		}
		if d == 5 {
			long = edge
		}
	}
	if short < 0 || long < 0 {
		t.Fatal("missing movement fixture")
	}
	attackHand(s, p, []int{0, 0, 0, 2, 0})
	if err := s.Apply(p, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	return s, from, short, long
}
func TestCatanAttackPlanDraftPrivacyUndoAndConfirm(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		s, from, short, long := attackPlanFixture(t, n)
		p := s.Turn
		original := clone(*s)
		if s.Phase != "catan_attack_end" || s.CatanPendingActor() != p {
			t.Fatal("no pending actor")
		}
		q := s.Catan.Attack.EndPlan
		action := Action{Type: "catan_attack_move", Prompt: q.ID, Choice: "wheat", Edge: from, Target: long}
		attackReject(t, s, p, Action{Type: "catan_attack_move", Prompt: q.ID, Choice: "confirm"})
		for _, actor := range []int{-1, (p + 1) % n} {
			attackReject(t, s, actor, action)
		}
		for _, change := range []func(*Action){func(a *Action) { a.Prompt-- }, func(a *Action) { a.Type = "catan_end" }, func(a *Action) { a.Choice = "normal" }, func(a *Action) { a.Edge = -1 }, func(a *Action) { a.Target = from }, func(a *Action) { a.Target = -1 }, func(a *Action) { a.Choice = "unknown" }} {
			bad := action
			change(&bad)
			attackReject(t, s, p, bad)
		}
		if err := s.Apply(p, action); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(s.Catan.Attack.Knights, original.Catan.Attack.Knights) || !slices.Equal(s.Catan.Players[p].Resources, original.Catan.Players[p].Resources) || !slices.Equal(s.Log, original.Log) {
			t.Fatal("draft changed public state")
		}
		attackReject(t, s, p, action)
		for viewer := -1; viewer < n; viewer++ {
			pub := s.View(viewer)["catan"].(map[string]any)["attack"].(map[string]any)
			if pub["deck"] != nil || pub["canAct"] != (viewer == p) {
				t.Fatal("controls/privacy")
			}
			if viewer == p {
				if pub["canConfirm"] != true || pub["previewWheat"] != 1 || pub["previewKnights"].([]catanAttackKnight)[0].Edge != long {
					t.Fatal("wrong preview", pub)
				}
			} else {
				if pub["endPlan"].(map[string]any)["moves"] != nil || pub["moveChoices"] != nil || pub["previewKnights"] != nil || pub["previewWheat"] != nil || pub["canConfirm"] != nil {
					t.Fatal("unfinished strategy leaked")
				}
			}
		}
		assertAttackRestored(t, s)
		undo := Action{Type: "catan_attack_move", Prompt: q.ID, Choice: "undo"}
		if err := s.Apply(p, undo); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(s, &original) {
			t.Fatal("undo did not restore draft")
		}
		attackReject(t, s, p, undo)
		action.Choice, action.Target = "normal", short
		if err := s.Apply(p, action); err != nil {
			t.Fatal(err)
		}
		if err := s.Apply(p, Action{Type: "catan_attack_move", Prompt: q.ID, Choice: "confirm"}); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Attack.EndPlan != nil || s.CatanPendingActor() != -1 || s.Turn == p || s.Catan.Attack.Knights[0].Edge != short || s.Catan.Players[p].Resources[3] != 2 || s.Catan.Attack.EndSequence != 1 {
			t.Fatal("confirmation/handoff", s.Phase)
		}
		assertAttackRestored(t, s)
		attackReject(t, s, p, action)
	}
}
func TestCatanAttackPlanBotTimeoutAndHiddenInformation(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, auto := range []bool{false, true} {
			s, _, _, _ := attackPlanFixture(t, n)
			// Recruit multiple knights at different sides of the castle; each must leave.
			for side := 1; side < 6; side++ {
				s.Catan.Attack.Knights = append(s.Catan.Attack.Knights, catanAttackKnight{s.Turn, catanFishingSide(s.Catan, s.Catan.Attack.Map.Castles[0], side)})
			}
			p := s.Turn
			for steps := 0; s.Phase == "catan_attack_end"; steps++ {
				if steps > 6 {
					t.Fatal("autoplay failed to finish bounded plan")
				}
				action, err := s.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				hidden := clone(*s)
				slices.Reverse(hidden.Catan.Attack.Deck)
				attackHand(&hidden, (p+1)%n, []int{3, 3, 3, 3, 3})
				other, err := hidden.BotAction(p)
				if err != nil || !reflect.DeepEqual(action, other) {
					t.Fatal("bot reads hidden information", err)
				}
				if auto {
					s.AutoCatanPending()
				} else if err = s.Apply(p, action); err != nil {
					t.Fatal(err)
				}
				assertAttackRestored(t, s)
			}
			if s.Catan.Attack.EndSequence != 1 || s.Turn == p {
				t.Fatal("end phase not completed")
			}
		}
	}
}
func TestCatanAttackPlanCorruptionAtomicity(t *testing.T) {
	s, _, _, _ := attackPlanFixture(t, 3)
	for _, change := range []func(*State){
		func(s *State) { s.Catan.Attack.EndPlan.ID++ }, func(s *State) { s.Catan.Attack.EndPlan.Player++ }, func(s *State) { s.Phase = "catan_turn" }, func(s *State) { s.Catan.Attack.EndPlan = nil }, func(s *State) { s.Catan.Attack.EndPlan.Moves = []catanAttackMove{{-1, 2, false}} },
	} {
		bad := clone(*s)
		change(&bad)
		if err := bad.validateCatanAttack(); err == nil {
			t.Fatal("invalid plan accepted")
		}
		attackReject(t, &bad, bad.Turn, Action{Type: "catan_attack_move", Prompt: 1, Choice: "confirm"})
		before, _ := json.Marshal(&bad)
		bad.AutoCatanPending()
		after, _ := json.Marshal(&bad)
		if string(before) != string(after) {
			t.Fatal("invalid auto mutated state")
		}
	}
}
func TestCatanAttackEndNoOwnKnightsStillBattles(t *testing.T) {
	s := newAttackState(t, 3, false)
	p := s.Turn
	tile := s.Catan.Attack.Map.Coast[0]
	s.Catan.Attack.Barbarians[tile] = 1
	attackBattleKnights(s, tile, (p+1)%3, (p+1)%3)
	if err := s.Apply(p, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Attack.Barbarians[tile] != 0 || s.Catan.Attack.Prisoners[(p+1)%3] != 1 || s.Turn == p || s.Catan.Attack.EndPlan != nil {
		t.Fatal("opponent battle skipped")
	}
	assertAttackRestored(t, s)
}
