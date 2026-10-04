package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func sgAnswerBluff(t *testing.T, s *State, challenger int) {
	t.Helper()
	for q := s.Sanguosha.Pending; q != nil && q.Kind == "guhuo_question"; q = s.Sanguosha.Pending {
		choice := "pass"
		if q.Player == challenger {
			choice = "challenge"
		}
		sgDo(t, s, q.Player, Action{Choice: choice})
	}
}

func TestSanguoshaGuhuoTruthHeartAndChallenges(t *testing.T) {
	for _, tc := range []struct {
		name, physical, declared string
		suit, challenge          int
		hp                       int
		draw                     bool
	}{
		{"undoubted lie", "jink", "slash", 3, -1, 4, false},
		{"challenged lie", "jink", "slash", 3, 2, 4, true},
		{"challenged true heart", "slash", "slash", 1, 2, 3, false},
		{"challenged true black", "slash", "slash", 0, 2, 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sgWindState()
			s.Sanguosha.Players[0].General = "yuji"
			id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == tc.physical && c.Suit == tc.suit })
			sgDo(t, s, 0, Action{Type: "sg_play", Skill: "guhuo", Choice: tc.declared, Cards: []int{id}, Targets: []int{1}})
			if s.Sanguosha.Pending.Kind != "guhuo_question" || len(s.Sanguosha.Table) != 0 {
				t.Fatal("did not conceal declaration")
			}
			view := s.View(1)["sanguosha"].(map[string]any)
			if _, ok := view["bluff"].(map[string]any)["card"]; ok {
				t.Fatal("bluff exposed physical card")
			}
			if context := view["bluff"].(map[string]any)["context"].(string); !strings.Contains(context, s.sgName(1)) {
				t.Fatal("challenge did not announce the target", context)
			}
			if _, ok := view["players"].([]map[string]any)[0]["hand"]; ok {
				t.Fatal("bluff exposed declarer's hand")
			}
			sgWindConservation(t, s)
			sgAnswerBluff(t, s, tc.challenge)
			sgPassAll(t, s)
			if s.Sanguosha.Players[2].HP != tc.hp {
				t.Fatal("wrong truth consequence", s.Sanguosha.Players[2].HP, tc.hp)
			}
			wantDamage := tc.challenge < 0 || tc.physical == tc.declared && tc.suit == 1
			wantHP := 4
			if wantDamage {
				wantHP = 3
			}
			if s.Sanguosha.Players[1].HP != wantHP {
				t.Fatal("wrong bluff result", s.Sanguosha.Players[1].HP, wantHP)
			}
			if tc.draw && len(s.Sanguosha.Players[2].Hand) != 1 {
				t.Fatal("false challenge did not draw")
			}
			if !slices.Contains(s.Sanguosha.Discard, id) || len(s.Sanguosha.Bluffs) != 0 {
				t.Fatal("bluff continuation not cleaned")
			}
			sgWindConservation(t, s)
		})
	}
}

func TestSanguoshaGuhuoCounterspellChallengerDeath(t *testing.T) {
	s := sgWindState()
	s.Sanguosha.Players[1].General = "yuji"
	s.Sanguosha.Players[2].HP = 1
	duel := sgGive(t, s, 0, "duel")
	null := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Kind == "nullification" && c.Suit == 0 })
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{duel}, Targets: []int{1}})
	sgDo(t, s, 2, Action{Choice: "pass"})
	sgDo(t, s, 1, Action{Skill: "guhuo", Choice: "nullification", Cards: []int{null}})
	view := s.View(-1)["sanguosha"].(map[string]any)["bluff"].(map[string]any)
	if context := view["context"].(string); !strings.Contains(context, "决斗") || !strings.Contains(context, s.sgName(1)) {
		t.Fatal("counterspell context missing", context)
	}
	sgAnswerBluff(t, s, 2)
	for q := s.Sanguosha.Pending; q != nil && q.Kind == "peach"; q = s.Sanguosha.Pending {
		sgDo(t, s, q.Player, Action{Choice: "pass"})
	}
	if !s.Sanguosha.Players[2].Dead {
		t.Fatal("challenger should have died")
	}
	sgDo(t, s, 0, Action{Choice: "pass"})
	sgDo(t, s, 1, Action{Choice: "pass"})
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "nullification" || !s.View(3)["sanguosha"].(map[string]any)["pending"].(map[string]any)["canRespond"].(bool) {
		t.Fatal("dead passer consumed a living player's counterspell opportunity", q)
	}
	sgDo(t, s, 3, Action{Choice: "pass"})
	sgPassAll(t, s)
	sgWindConservation(t, s)
}

func TestSanguoshaGuhuoResponseAndRetry(t *testing.T) {
	s := sgWindState()
	s.Sanguosha.Players[1].General = "yuji"
	slash := sgGive(t, s, 0, "slash")
	fake := sgGive(t, s, 1, "slash")
	jink := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Kind == "jink" && c.Suit == 1 })
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
	oldPrompt := s.Sanguosha.Pending.ID
	sgDo(t, s, 1, Action{Skill: "guhuo", Choice: "jink", Cards: []int{fake}})
	sgAnswerBluff(t, s, 2)
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "card" || q.Player != 1 || q.ID == oldPrompt {
		t.Fatal("failed bluff did not restore fresh response", q)
	}
	if s.Apply(1, Action{Prompt: oldPrompt, Choice: "pass"}) == nil {
		t.Fatal("stale response accepted")
	}
	sgDo(t, s, 1, Action{Skill: "guhuo", Choice: "jink", Cards: []int{jink}})
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	sgAnswerBluff(t, &restored, 2)
	sgPassAll(t, &restored)
	if restored.Sanguosha.Players[1].HP != 4 || restored.Sanguosha.Players[2].HP != 3 {
		t.Fatal("real response failed")
	}
	sgWindConservation(t, &restored)
}

func TestSanguoshaGuhuoCounterspellAndRestrictions(t *testing.T) {
	t.Run("counterspell resumes original common window", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[1].General = "yuji"
		duel := sgGive(t, s, 0, "duel")
		fake := sgGive(t, s, 1, "peach")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{duel}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Skill: "guhuo", Choice: "nullification", Cards: []int{fake}})
		sgAnswerBluff(t, s, -1)
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 4 || !slices.Contains(s.Sanguosha.Discard, fake) {
			t.Fatal("counterspell failed")
		}
		sgWindConservation(t, s)
	})
	t.Run("delayed, equipment, recast and illegal target cannot be declared", func(t *testing.T) {
		s := sgWindState()
		s.Sanguosha.Players[0].General = "yuji"
		id := sgGive(t, s, 0, "jink")
		for _, a := range []Action{
			{Type: "sg_play", Skill: "guhuo", Choice: "lightning", Cards: []int{id}},
			{Type: "sg_play", Skill: "guhuo", Choice: "crossbow", Cards: []int{id}},
			{Type: "sg_play", Skill: "guhuo", Choice: "iron_chain", Cards: []int{id}},
			{Type: "sg_play", Skill: "guhuo", Choice: "slash", Cards: []int{id}, Targets: []int{2}},
		} {
			before, _ := json.Marshal(s)
			if s.Apply(0, a) == nil {
				t.Fatal("invalid declaration accepted", a)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("invalid declaration mutated state")
			}
		}
	})
}

func TestSanguoshaGuhuoConsequencesPauseForRescue(t *testing.T) {
	s := sgWindState()
	s.Sanguosha.Players[0].General = "yuji"
	s.Sanguosha.Players[2].HP = 1
	id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "slash" && c.Suit == 1 })
	peach := sgGive(t, s, 0, "peach")
	sgDo(t, s, 0, Action{Type: "sg_play", Skill: "guhuo", Choice: "slash", Cards: []int{id}, Targets: []int{1}})
	sgAnswerBluff(t, s, 2)
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "peach" || q.Event.Target != 2 {
		t.Fatal("truth penalty did not suspend for rescue", q)
	}
	sgDo(t, s, 0, Action{Cards: []int{peach}})
	sgPassAll(t, s)
	if s.Sanguosha.Players[1].HP != 3 || s.Sanguosha.Players[2].HP != 1 {
		t.Fatal("did not resume original attack after rescue")
	}
	sgWindConservation(t, s)
}

func TestSanguoshaGuhuoNestedBluffRescue(t *testing.T) {
	s := sgWindState()
	s.Sanguosha.Players[0].General = "yuji"
	s.Sanguosha.Players[2].HP = 1
	id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "slash" && c.Suit == 1 })
	peach := sgGive(t, s, 0, "peach")
	sgDo(t, s, 0, Action{Type: "sg_play", Skill: "guhuo", Choice: "slash", Cards: []int{id}, Targets: []int{1}})
	sgAnswerBluff(t, s, 2)
	sgDo(t, s, 0, Action{Skill: "guhuo", Choice: "peach", Cards: []int{peach}})
	if len(s.Sanguosha.Bluffs) != 2 {
		t.Fatal("nested continuation lost")
	}
	raw, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	sgAnswerBluff(t, &restored, -1)
	sgPassAll(t, &restored)
	if restored.Sanguosha.Players[1].HP != 3 || restored.Sanguosha.Players[2].HP != 1 || len(restored.Sanguosha.Bluffs) != 0 {
		t.Fatal("nested resolution failed")
	}
	sgWindConservation(t, &restored)
}
