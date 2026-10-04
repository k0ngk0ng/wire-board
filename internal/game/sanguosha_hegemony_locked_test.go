package game

import (
	"slices"
	"testing"
)

func sgHegPassCounters(t *testing.T, s *State) {
	t.Helper()
	for n := 0; n < 32 && s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "nullification"; n++ {
		sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
	}
}

func TestSanguoshaHegemonyKongchengCancelsAfterPayment(t *testing.T) {
	for _, shown := range []bool{false, true} {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_zhugeliang", "heg_liubei"})
		if shown {
			s.sgHegShow(1, []int{0}, false)
		}
		id := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		if !shown {
			if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_invoke" || q.Event.Kind != "kongcheng" {
				t.Fatal("hidden Kongcheng must be invokable", q)
			}
			sgDo(t, s, 1, Action{Choice: "yes"})
			sgRestoreMountain(t, s)
			sgDo(t, s, 1, Action{Choice: "draw"})
			if len(s.Sanguosha.Players[1].Hand) != 2 {
				t.Fatal("first-show reward should resolve before target cancellation")
			}
		}
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 3 || !slices.Contains(s.Sanguosha.Discard, id) || s.Sanguosha.Players[0].Used["slash"] != 1 {
			t.Fatal("Kongcheng must consume and then cancel the used Slash")
		}
		sgHegConservation(t, s)
	}
	t.Run("decline does not re-offer at effect resolution", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_zhugeliang", "heg_liubei"})
		id := sgGive(t, s, 0, "duel")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "pass"})
		sgHegPassCounters(t, s)
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "card" {
			t.Fatal("declined Kongcheng re-offered or lost the duel", q)
		}
		sgDo(t, s, 1, Action{Choice: "pass"})
		if s.sgHegShown(1) || s.Sanguosha.Players[1].HP != 2 {
			t.Fatal("decline must remain hidden and take ordinary damage")
		}
	})
}

func TestSanguoshaHegemonyQianxunWeimu(t *testing.T) {
	for _, weimu := range []bool{false, true} {
		s := sgHegState(4)
		kind, skill := "indulgence", "qianxun"
		pair := [2]string{"heg_luxun", "heg_ganning"}
		if weimu {
			kind, skill = "duel", "weimu"
			pair = [2]string{"heg_jiaxu", "heg_mateng"}
		}
		sgHegSetPair(s, 1, pair)
		id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == kind && (c.Suit == 0 || c.Suit == 2) })
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_invoke" || q.Event.Kind != skill {
			t.Fatal("hidden target protection", q)
		}
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgPassAll(t, s)
		if len(s.Sanguosha.Players[1].Judgment) != 0 || s.Sanguosha.Players[1].HP != 3 || !slices.Contains(s.Sanguosha.Discard, id) {
			t.Fatal("cancelled trick still took effect")
		}
		sgHegConservation(t, s)
	}
}

func TestSanguoshaHegemonyHiddenWushuangAndXiangle(t *testing.T) {
	t.Run("Wushuang offered before counter window", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 0, [2]string{"heg_lvbu", "heg_diaochan"})
		id := sgGive(t, s, 0, "duel")
		response := sgGive(t, s, 1, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_invoke" || q.Event.Kind != "wushuang" || q.Player != 0 {
			t.Fatal("Wushuang reveal", q)
		}
		sgDo(t, s, 0, Action{Choice: "yes"})
		sgDo(t, s, 0, Action{Choice: "pass"})
		sgHegPassCounters(t, s)
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "card" || q.Event.Count != 2 {
			t.Fatal("Duel did not require two Slashes", q)
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 1, Action{Cards: []int{response}})
		if q := s.Sanguosha.Pending; q == nil || q.Player != 1 || q.Event.Count != 1 {
			t.Fatal("first of two responses was treated as enough", q)
		}
		sgDo(t, s, 1, Action{Choice: "pass"})
		if s.Sanguosha.Players[1].HP != 2 {
			t.Fatal("Wushuang duel damage")
		}
		sgHegConservation(t, s)
	})
	t.Run("Xiangle reveal then attacker payment", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_liushan", "heg_liubei"})
		id := sgGive(t, s, 0, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgDo(t, s, 1, Action{Choice: "pass"})
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "xiangle" || q.Player != 0 {
			t.Fatal("Xiangle did not resume with attacker's cost", q)
		}
		sgDo(t, s, 0, Action{Choice: "pass"})
		if s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("unpaid Xiangle Slash must fail")
		}
		sgHegConservation(t, s)
	})
}

func TestSanguoshaHegemonyHiddenHongyanAndBazhen(t *testing.T) {
	t.Run("Hongyan may reveal after retrials", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_xiaoqiao", "heg_taishici"})
		top := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 0 })
		s.Sanguosha.Players[0].Hand = nil
		s.Sanguosha.Deck = append([]int{top}, s.Sanguosha.Deck...)
		s.sgPush(SGEvent{Type: "judge", Actor: 1, Target: 0, Kind: "ganglie"})
		s.sgRun()
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_invoke" || q.Event.Kind != "hongyan" {
			t.Fatal("hidden judgment filter", q)
		}
		sgDo(t, s, 1, Action{Choice: "yes"})
		sgRestoreMountain(t, s)
		sgDo(t, s, 1, Action{Choice: "pass"})
		if s.Sanguosha.Players[0].HP != 4 || s.Sanguosha.Pending != nil || !s.sgHas(1, "hongyan") {
			t.Fatal("revealed Hongyan did not change spade to heart")
		}
		sgHegConservation(t, s)
	})
	t.Run("Bazhen reveal is tied to its armor response", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_wolong", "heg_zhaoyun"})
		id := sgGive(t, s, 0, "slash")
		top := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 1 })
		s.Sanguosha.Players[0].Hand = sgRemove(s.Sanguosha.Players[0].Hand, top)
		s.Sanguosha.Deck = append([]int{top}, s.Sanguosha.Deck...)
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "eight_diagram"})
		sgDo(t, s, 1, Action{Choice: "pass"})
		if s.Sanguosha.Players[1].HP != 3 || !s.sgHas(1, "bazhen") {
			t.Fatal("Bazhen armor reveal/response")
		}
		sgHegConservation(t, s)
	})
}

func TestSanguoshaHegemonyHiddenWansha(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_jiaxu", "heg_mateng"})
	s.Sanguosha.Players[2].HP = 0
	s.sgEnterDying(SGEvent{Actor: -1, Target: 2, Step: 0})
	s.sgRun()
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_invoke" || q.Event.Kind != "wansha" {
		t.Fatal("Wansha reveal at dying", q)
	}
	sgDo(t, s, 0, Action{Choice: "yes"})
	sgDo(t, s, 0, Action{Choice: "pass"})
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "peach" || q.Player != 0 {
		t.Fatal("active seat must retain rescue opportunity", q)
	}
	sgDo(t, s, 0, Action{Choice: "pass"})
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "peach" || q.Player != 2 {
		t.Fatal("Wansha must skip third-party rescuers", q)
	}
	sgHegConservation(t, s)
}
