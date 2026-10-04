package game

import (
	"slices"
	"testing"
)

func TestSanguoshaHegemonyShenzhiShushen(t *testing.T) {
	s := sgHegState(6)
	sgHegSetPair(s, 0, [2]string{"heg_ganfuren", "heg_liubei"})
	s.sgHegShow(0, []int{0}, false)
	s.sgHegShow(2, []int{0}, false)
	s.Sanguosha.Players[0].HP = 1
	s.sgDraw(0, 2)
	s.sgPush(SGEvent{Type: "heg_start", Actor: 0})
	s.sgRun()
	sgDo(t, s, 0, Action{Choice: "yes"})
	q := s.Sanguosha.Pending
	if q == nil || q.Kind != "heg_shushen" || !slices.Equal(q.Targets, []int{2}) || s.Sanguosha.Players[0].HP != 2 || len(s.Sanguosha.Players[0].Hand) != 0 {
		t.Fatal("Shenzhi payment/heal or Shushen public allies", q)
	}
	sgGodIllegal(t, s, 0, Action{Targets: []int{1}})
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Targets: []int{2}})
	if len(s.Sanguosha.Players[2].Hand) != 1 {
		t.Fatal("Shushen should draw one")
	}
	s.Sanguosha.Players[0].HP = 1
	s.sgHeal(0, 5) // capped at three HP, so exactly two opportunities
	s.sgRun()
	sgDo(t, s, 0, Action{Targets: []int{2}})
	sgDo(t, s, 0, Action{Choice: "pass"})
	if s.Sanguosha.Pending != nil || len(s.Sanguosha.Players[2].Hand) != 2 {
		t.Fatal("Shushen triggered for unhealed HP")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyXiaoguo(t *testing.T) {
	for _, discard := range []bool{false, true} {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_yuejin", "heg_xuchu"})
		s.sgHegShow(1, []int{0}, false)
		cost := sgGive(t, s, 1, "jink")
		wrong := sgGive(t, s, 1, "duel")
		armor := sgGive(t, s, 0, "eight_diagram")
		s.sgPush(SGEvent{Type: "heg_finish", Actor: 0})
		s.sgRun()
		if q := s.Sanguosha.Pending; q == nil || q.Player != 1 || q.Kind != "heg_xiaoguo" {
			t.Fatal("finish-stage trigger", q)
		}
		sgGodIllegal(t, s, 1, Action{Cards: []int{wrong}})
		sgDo(t, s, 1, Action{Cards: []int{cost}})
		sgRestoreMountain(t, s)
		if discard {
			sgDo(t, s, 0, Action{Cards: []int{armor}})
		} else {
			sgDo(t, s, 0, Action{Choice: "pass"})
		}
		want := 3
		if discard {
			want = 4
		}
		if s.Sanguosha.Players[0].HP != want || len(s.Sanguosha.Players[1].Hand) != 1 {
			t.Fatal("national-war Xiaoguo damage or unexpected identity-version draw")
		}
		sgHegConservation(t, s)
	}
}

func TestSanguoshaHegemonySijian(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_tianfeng", "heg_jiling"})
	s.sgHegShow(0, []int{0}, false)
	card := sgGive(t, s, 0, "jink")
	armor := sgWear(t, s, 1, "eight_diagram")
	judgment := sgGive(t, s, 2, "indulgence")
	s.Sanguosha.Players[2].Hand = nil
	s.Sanguosha.Players[2].Judgment = []SGDelayed{{Card: judgment, Kind: "indulgence"}}
	s.sgSpent(0, []int{card}) // actual loss, not restricted to discards
	s.sgRun()
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_sijian" || !slices.Equal(q.Targets, []int{1}) {
		t.Fatal("Sijian allowed a judgment-only target", q)
	}
	sgDo(t, s, 0, Action{Targets: []int{1}})
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Card: armor})
	if !slices.Contains(s.Sanguosha.Discard, armor) || len(s.Sanguosha.Players[0].Hand) != 0 {
		t.Fatal("Sijian stole instead of discarding")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyKuangfu(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_panfeng", "heg_jiling"})
	sgHegSetPair(s, 1, [2]string{"heg_sunshangxiang", "heg_sunquan"})
	s.sgHegShow(0, []int{0}, false)
	s.sgHegShow(1, []int{0}, false)
	armor := sgWear(t, s, 1, "eight_diagram")
	second := sgWear(t, s, 2, "renwang_shield")
	s.sgPush(SGEvent{Type: "damage_dealt", Actor: 0, Target: 1, Kind: "slash", Amount: 1, Chain: true})
	s.sgRun()
	if s.Sanguosha.Pending != nil {
		t.Fatal("chain damage triggered Kuangfu")
	}
	s.sgPush(SGEvent{Type: "damage_dealt", Actor: 0, Target: 1, Kind: "slash", Amount: 1})
	s.sgRun()
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Choice: "move", Cards: []int{armor}})
	if s.sgEquip(0, "armor") != armor || s.sgEquip(1, "armor") != 0 || s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "invoke" {
		t.Fatal("equipment transfer or Xiaoji loss trigger", s.Sanguosha.Pending)
	}
	sgDo(t, s, 1, Action{Choice: "yes"})
	if len(s.Sanguosha.Players[1].Hand) != 2 {
		t.Fatal("Xiaoji did not draw")
	}
	s.Sanguosha.Players[2].Equip = sgRemove(s.Sanguosha.Players[2].Equip, second)
	s.Sanguosha.Players[1].Equip = append(s.Sanguosha.Players[1].Equip, second)
	s.sgPush(SGEvent{Type: "damage_dealt", Actor: 0, Target: 1, Kind: "slash", Amount: 1})
	s.sgRun()
	sgGodIllegal(t, s, 0, Action{Choice: "move", Cards: []int{second}})
	sgDo(t, s, 0, Action{Choice: "discard", Cards: []int{second}})
	sgDo(t, s, 1, Action{Choice: "pass"})
	sgHegConservation(t, s)
}
