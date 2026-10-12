package sanguosha

// Failure cases: empty response windows, non-holders asked to pass, exhausted
// counters asked again, disabled skills/hand cards admitted, legal conversion,
// equipment or bluff responses suppressed, hidden generals revealed by a probe,
// empty rescue/support loops, and automatic passing consuming a real option.

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSanguoshaResponseEligibilityNullification(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, *State)
		want  bool
	}{
		{"empty hands", func(t *testing.T, s *State) {}, false},
		{"ordinary card", func(t *testing.T, s *State) { sgGive(t, s, 1, "slash") }, false},
		{"real counter", func(t *testing.T, s *State) { sgGive(t, s, 1, "nullification") }, true},
		{"sealed hand", func(t *testing.T, s *State) {
			sgGive(t, s, 1, "nullification")
			s.Sanguosha.Players[1].HandSealed = true
		}, false},
		{"kanpo black hand", func(t *testing.T, s *State) {
			s.Sanguosha.Players[1].General = "wolong"
			sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 && c.Kind == "slash" })
		}, true},
		{"kanpo red only", func(t *testing.T, s *State) {
			s.Sanguosha.Players[1].General = "wolong"
			sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 1 && c.Kind == "slash" })
		}, false},
		{"kanpo silenced", func(t *testing.T, s *State) {
			s.Sanguosha.Players[1].General = "wolong"
			s.Sanguosha.Players[1].Silenced = true
			sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 && c.Kind == "slash" })
		}, false},
		{"kanpo cannot use equipment", func(t *testing.T, s *State) {
			p := &s.Sanguosha.Players[1]
			p.General = "wolong"
			id := sgGive(t, s, 1, "spear")
			p.Hand = sgRemove(p.Hand, id)
			p.Equip = append(p.Equip, id)
		}, false},
		{"longhun insufficient cost", func(t *testing.T, s *State) {
			p := &s.Sanguosha.Players[1]
			p.General = "shenzhaoyun"
			p.HP = 2
			sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 && c.Kind == "slash" })
		}, false},
		{"longhun exact cost", func(t *testing.T, s *State) {
			p := &s.Sanguosha.Players[1]
			p.General = "shenzhaoyun"
			p.HP = 2
			for range 2 {
				sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 && c.Kind == "slash" })
			}
		}, true},
		{"guhuo empty", func(t *testing.T, s *State) { s.Sanguosha.Players[1].General = "yuji" }, false},
		{"guhuo bluff is a legal option", func(t *testing.T, s *State) { s.Sanguosha.Players[1].General = "yuji"; sgGive(t, s, 1, "slash") }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := sgTestState()
			tc.setup(t, s)
			trick := sgGive(t, s, 0, "ex_nihilo")
			sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{trick}})
			q := s.Sanguosha.Pending
			if !tc.want {
				if q != nil {
					t.Fatalf("unusable response interrupted trick: %+v", q)
				}
				if len(s.Sanguosha.Players[0].Hand) != 2 {
					t.Fatal("trick did not resolve")
				}
				return
			}
			if q == nil || q.Kind != "nullification" || !reflect.DeepEqual(s.SanguoshaActors(), []int{1}) {
				t.Fatalf("wrong responders: %+v / %v", q, s.SanguoshaActors())
			}
			before, _ := json.Marshal(s)
			for _, viewer := range []int{-1, 0, 1, 2, 3} {
				pending := s.View(viewer)["sanguosha"].(map[string]any)["pending"].(map[string]any)
				if pending["canRespond"] != (viewer == 1) {
					t.Fatal("unusable player sees a response", viewer, pending)
				}
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("eligibility mutated state")
			}
			sgDo(t, s, 1, Action{Choice: "pass"})
			if s.Sanguosha.Pending != nil || len(s.Sanguosha.Players[0].Hand) != 2 {
				t.Fatal("one pass still waits for non-holders")
			}
		})
	}
}

func TestSanguoshaResponseEligibilityCounterChain(t *testing.T) {
	s := sgTestState()
	trick := sgGive(t, s, 0, "ex_nihilo")
	first := sgGive(t, s, 1, "nullification")
	second := sgGive(t, s, 2, "nullification")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{trick}})
	sgDo(t, s, 2, Action{Choice: "pass"})
	sgDo(t, s, 1, Action{Cards: []int{first}})
	if !reflect.DeepEqual(s.SanguoshaActors(), []int{2}) {
		t.Fatal("spent counter or previous pass not recalculated", s.SanguoshaActors())
	}
	sgDo(t, s, 2, Action{Cards: []int{second}})
	if s.Sanguosha.Pending != nil || len(s.Sanguosha.Players[0].Hand) != 2 {
		t.Fatal("counter chain did not immediately restore trick")
	}
}

func TestSanguoshaResponseEligibilityHegemony(t *testing.T) {
	s := sgHegState(4)
	for i := range s.Sanguosha.Players {
		p := &s.Sanguosha.Players[i]
		s.Sanguosha.Deck = append(s.Sanguosha.Deck, p.Hand...)
		p.Hand = nil
	}
	sgHegSetPair(s, 1, [2]string{"heg_wolong", "heg_huangyueying"})
	black := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 && c.Kind == "slash" })
	trick := sgGive(t, s, 0, "ex_nihilo")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{trick}})
	if !reflect.DeepEqual(s.SanguoshaActors(), []int{1}) {
		t.Fatal("hidden kanpo not eligible", s.SanguoshaActors())
	}
	before, _ := json.Marshal(s)
	view := s.View(-1)["sanguosha"].(map[string]any)
	if view["players"].([]map[string]any)[1]["general"] != "" {
		t.Fatal("probe revealed general")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("probe revealed or rewarded general")
	}
	sgDo(t, s, 1, Action{Cards: []int{black}, Skill: "kanpo"})
	if !s.Sanguosha.Players[1].Hegemony.Shown[0] {
		t.Fatal("actual kanpo did not reveal general")
	}
}

func TestSanguoshaResponseEligibilityCardRescueAndSupport(t *testing.T) {
	t.Run("unanswerable slash resolves", func(t *testing.T) {
		s := sgTestState()
		slash := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
		if s.Sanguosha.Pending != nil || s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("empty jink prompt blocks damage")
		}
	})
	t.Run("equipment response stays available", func(t *testing.T) {
		s := sgTestState()
		armor := sgGive(t, s, 1, "eight_diagram")
		p := &s.Sanguosha.Players[1]
		p.Hand = sgRemove(p.Hand, armor)
		p.Equip = append(p.Equip, armor)
		slash := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "card" || q.Player != 1 {
			t.Fatal("armor opportunity lost", q)
		}
	})
	t.Run("rescue skips players without peach", func(t *testing.T) {
		s := sgTestState()
		s.Turn = 1
		s.Sanguosha.Players[2].HP = 1
		peach := sgGive(t, s, 3, "peach")
		slash := sgGive(t, s, 1, "slash")
		sgDo(t, s, 1, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{2}})
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "peach" || q.Player != 3 {
			t.Fatal("empty rescue prompts", q)
		}
		sgDo(t, s, 3, Action{Cards: []int{peach}})
		if s.Sanguosha.Players[2].HP != 1 || s.Sanguosha.Players[2].Dead {
			t.Fatal("rescue failed")
		}
	})
	t.Run("hujia skips allies without a response", func(t *testing.T) {
		s := sgTestState()
		s.Turn = 3
		s.Sanguosha.Players[0].General = "caocao"
		s.Sanguosha.Players[1].General = "xiahoudun"
		s.Sanguosha.Players[2].General = "guojia"
		sgGive(t, s, 2, "jink")
		slash := sgGive(t, s, 3, "slash")
		sgDo(t, s, 3, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{0}})
		sgDo(t, s, 0, Action{Choice: "hujia"})
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "support" || q.Player != 2 {
			t.Fatal("empty ally prompt", q)
		}
	})
	t.Run("free skill remains optional", func(t *testing.T) {
		s := sgTestState()
		s.Sanguosha.Players[0].General = "huangyueying"
		trick := sgGive(t, s, 0, "ex_nihilo")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{trick}})
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "invoke" || q.Event.Kind != "jizhi" {
			t.Fatal("free skill opportunity lost", q)
		}
	})
}
