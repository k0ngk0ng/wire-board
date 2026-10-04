package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestSanguoshaHegemonyMingshi(t *testing.T) {
	for _, shown := range [][]int{nil, {0}, {0, 1}} {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_kongrong", "heg_mateng"})
		s.sgHegShow(1, []int{0}, false)
		s.sgHegShow(0, shown, false)
		s.sgDamage(SGEvent{Actor: 0, Target: 1, Kind: "slash", Amount: 1})
		want := 3
		if len(shown) == 2 {
			want = 2
		}
		if s.Sanguosha.Players[1].HP != want {
			t.Fatal("Mingshi must depend on both generals being shown", shown)
		}
		s.sgRun()
		if s.Sanguosha.Pending != nil {
			t.Fatal("fully prevented damage triggered a hurt skill")
		}
		s.sgDamage(SGEvent{Actor: -1, Target: 1, Kind: "lightning", Amount: 1})
		if s.Sanguosha.Players[1].HP != want-1 {
			t.Fatal("Mingshi reduced sourceless damage")
		}
	}
	t.Run("each chain recipient applies its own reduction", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_kongrong", "heg_jiling"})
		s.sgHegShow(1, []int{0}, false)
		s.Sanguosha.Players[1].Chained, s.Sanguosha.Players[2].Chained = true, true
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 2, Kind: "fire_slash", Amount: 1})
		s.sgRun()
		if s.Sanguosha.Players[1].HP != 3 || s.Sanguosha.Players[2].HP != 3 || !s.Sanguosha.Players[1].Chained {
			t.Fatal("prevention of propagated damage or chain state")
		}
	})
	t.Run("transfer uses recipient Mingshi", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 2, [2]string{"heg_kongrong", "heg_jiling"})
		s.sgHegShow(1, []int{1}, false)
		s.sgHegShow(2, []int{0}, false)
		heart := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 1 })
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Kind: "slash", Amount: 1})
		s.sgRun()
		sgRestoreMountain(t, s)
		sgDo(t, s, 1, Action{Cards: []int{heart}, Targets: []int{2}})
		if s.Sanguosha.Players[1].HP != 3 || s.Sanguosha.Players[2].HP != 3 {
			t.Fatal("transferred one damage should be prevented by Mingshi")
		}
		sgHegConservation(t, s)
	})
}

func TestSanguoshaHegemonyLirangMovementReason(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_kongrong", "heg_mateng"})
	s.sgHegShow(0, []int{0}, false)
	used := sgGive(t, s, 0, "jink")
	old := sgGive(t, s, 0, "slash")
	s.sgSpent(0, []int{used, old})
	s.sgRun()
	if s.Sanguosha.Pending != nil {
		t.Fatal("used/response cards cannot be redistributed")
	}
	costs := []int{sgGive(t, s, 0, "slash"), sgGive(t, s, 0, "duel"), sgGive(t, s, 0, "eight_diagram")}
	s.sgDiscard(0, costs)
	s.sgRun()
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_lirang" || !slices.Equal(q.Cards, costs) {
		t.Fatal("discard batch was not retained", q)
	}
	sgGodIllegal(t, s, 0, Action{Cards: []int{old}, Targets: []int{1}})
	sgGodIllegal(t, s, 0, Action{Cards: costs[:1], Targets: []int{0}})
	sgDo(t, s, 0, Action{Cards: costs[:1], Targets: []int{1}})
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Cards: costs[1:2], Targets: []int{2}})
	sgDo(t, s, 0, Action{Choice: "pass"})
	if !slices.Equal(s.Sanguosha.Players[1].Hand, costs[:1]) || !slices.Equal(s.Sanguosha.Players[2].Hand, costs[1:2]) || !slices.Contains(s.Sanguosha.Discard, costs[2]) {
		t.Fatal("Lirang partial delivery")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyShuangren(t *testing.T) {
	for _, win := range []bool{false, true} {
		s := sgHegState(6)
		sgHegSetPair(s, 0, [2]string{"heg_jiling", "heg_panfeng"})
		s.sgHegShow(0, []int{0}, false)
		s.sgHegShow(2, []int{0}, false)
		s.sgHegShow(6-1, []int{0}, false)
		sgHegSetPair(s, 4, [2]string{"heg_machao", "heg_huangzhong"})
		s.sgHegShow(4, []int{0}, false)
		high := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Rank == 13 })
		low := sgFindGive(t, s, 2, func(c SGCard) bool { return c.Rank == 1 })
		if !win {
			s.Sanguosha.Players[0].Hand, s.Sanguosha.Players[2].Hand = []int{low}, []int{high}
			high, low = low, high
		}
		s.Sanguosha.InPlay = false
		s.sgPush(SGEvent{Type: "play_phase", Actor: 0})
		s.sgRun()
		sgDo(t, s, 0, Action{Cards: []int{high}, Targets: []int{2}})
		visible, _ := json.Marshal(s.sgView(2))
		if s.Sanguosha.Pending.Kind != "pindian" || strings.Contains(string(visible), "pindian_result") || len(s.Sanguosha.Table) != 0 {
			t.Fatal("pindian initiation must stay private until both choose")
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 2, Action{Cards: []int{low}})
		if win {
			q := s.Sanguosha.Pending
			if q == nil || q.Kind != "heg_shuangren_slash" || !slices.Equal(q.Targets, []int{2, 4}) {
				t.Fatal("Shuangren public-faction choices", q)
			}
			sgGodIllegal(t, s, 0, Action{Targets: []int{5}})
			if s.sgDistance(0, 4) <= s.sgRange(0) {
				t.Fatal("range fixture")
			}
			sgDo(t, s, 0, Action{Targets: []int{4}})
			sgPassAll(t, s)
			if s.Turn != 0 || !s.Sanguosha.InPlay || s.Sanguosha.Players[0].Used["slash"] != 0 || s.Sanguosha.Players[4].HP != 3 {
				t.Fatal("Shuangren slash consumed normal limit or play phase")
			}
		} else if s.Turn == 0 && s.Sanguosha.InPlay {
			t.Fatal("lost Shuangren did not skip play")
		}
		sgHegConservation(t, s)
	}
}

func TestSanguoshaHegemonyDuanbing(t *testing.T) {
	s := sgHegState(6)
	sgHegSetPair(s, 0, [2]string{"heg_dingfeng", "heg_sunquan"})
	s.sgHegShow(0, []int{0}, false)
	sgWear(t, s, 0, "spear")
	card := sgGive(t, s, 0, "slash")
	sgGodIllegal(t, s, 0, Action{Type: "sg_play", Cards: []int{card}, Targets: []int{2, 3}})
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{card}, Targets: []int{2, 1}})
	sgPassAll(t, s)
	if s.Sanguosha.Players[1].HP != 2 || s.Sanguosha.Players[2].HP != 3 {
		t.Fatal("two-target Slash via Duanbing")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyQingcheng(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_zoushi", "heg_jiling"})
	s.sgHegShow(0, []int{0}, false)
	s.sgHegShow(1, []int{0, 1}, false)
	cost := sgWear(t, s, 0, "eight_diagram")
	sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "heg_qingcheng", Cards: []int{cost}, Targets: []int{2}, Choice: "head"})
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_qingcheng", Cards: []int{cost}, Targets: []int{1}, Choice: "deputy"})
	if s.sgHas(1, "hongyan") || !s.sgHas(1, "yingzi") || s.Sanguosha.Players[1].Role != "wu" || s.sgEquip(0, "armor") != 0 {
		t.Fatal("Qingcheng slot hiding and skill/faction effects")
	}
	sgRestoreMountain(t, s)
	s.sgHegShow(1, []int{1}, false)
	cost = sgGive(t, s, 0, "crossbow")
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_qingcheng", Cards: []int{cost}, Targets: []int{1}, Choice: "head"})
	if s.sgHas(1, "yingzi") || !s.sgHas(1, "hongyan") {
		t.Fatal("Qingcheng is repeatable with equipment payment")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyDuanchang(t *testing.T) {
	s := sgHegState(6)
	sgHegSetPair(s, 0, [2]string{"heg_caiwenji", "heg_jiling"})
	s.sgHegShow(1, []int{0}, false)
	s.sgDie(0, 1)
	s.sgRun()
	q := s.Sanguosha.Pending
	if q == nil || q.Kind != "heg_duanchang" || q.Player != 0 {
		t.Fatal("dead owner must choose the lost slot", q)
	}
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Choice: "deputy"})
	if !s.Sanguosha.Players[1].Hegemony.Lost[1] || s.Sanguosha.Players[1].Hegemony.Shown[1] || !s.sgHas(1, "yingzi") {
		t.Fatal("Duanchang must remove only one slot without exposing it")
	}
	s.sgHegShow(1, []int{1}, false)
	if s.sgHas(1, "hongyan") || s.sgHas(1, "tianxiang") {
		t.Fatal("revealing cannot restore lost skills")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonySuishi(t *testing.T) {
	s := sgHegState(6)
	sgHegSetPair(s, 0, [2]string{"heg_tianfeng", "heg_jiling"})
	s.sgHegShow(0, []int{0}, false)
	s.sgHegShow(3, []int{0}, false)
	s.Sanguosha.Players[2].HP = 0
	s.sgEnterDying(SGEvent{Actor: 3, Target: 2, Step: 0})
	s.sgRun()
	if len(s.Sanguosha.Players[0].Hand) != 1 {
		t.Fatal("same-faction damage source should trigger Suishi")
	}
	sgRestoreMountain(t, s)
	sgPassAll(t, s)
	if len(s.Sanguosha.Players[0].Hand) != 1 {
		t.Fatal("multiple rescue responses repeated Dying trigger")
	}
	s.sgDie(3, 1)
	s.sgRun()
	if s.Sanguosha.Players[0].HP != 2 {
		t.Fatal("death of public ally should lose one HP")
	}
	sgHegConservation(t, s)
}
