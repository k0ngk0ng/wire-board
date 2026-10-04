package game

import (
	"reflect"
	"testing"
)

func TestSanguoshaHegemonyPrepareReveal(t *testing.T) {
	s := sgHegState(4)
	s.sgPush(SGEvent{Type: "begin", Actor: 0})
	s.sgRun()
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_reveal_turn" {
		t.Fatal("missing legal start reveal", q)
	}
	sgGodIllegal(t, s, 1, Action{Choice: "both"})
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Choice: "head"})
	if s.Sanguosha.Players[0].Hegemony.Shown != [2]bool{true, false} || s.Sanguosha.Pending.Kind != "heg_reward" {
		t.Fatal("head reveal or first-show reward")
	}
	sgDo(t, s, 0, Action{Choice: "pass"})
	sgPassAll(t, s)
	if s.Phase != "sg_play" || len(s.Sanguosha.Players[0].Hand) != 2 {
		t.Fatal("reveal interrupted normal phases")
	}
	sgGodIllegal(t, s, 0, Action{Type: "sg_reveal", Choice: "deputy"})
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyActiveSkillRevealsAtomically(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_liubei", "heg_zhangfei"})
	id := sgGive(t, s, 0, "jink")
	sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "heg_rende", Cards: []int{id}, Targets: []int{0}})
	if s.sgHegShown(0) || s.Sanguosha.Hegemony.FirstClaimed {
		t.Fatal("invalid skill revealed a general or consumed first show")
	}
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_rende", Cards: []int{id}, Targets: []int{1}})
	if s.Sanguosha.Players[0].Hegemony.Shown != [2]bool{true, false} || !s.sgOwn(1, id, true) || s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "heg_reward" {
		t.Fatal("successful Rende should reveal only its owner slot")
	}
	sgDo(t, s, 0, Action{Choice: "pass"})
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyConversionRevealAndHuoshui(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_zoushi", "heg_jiling"})
	sgHegSetPair(s, 1, [2]string{"heg_zhaoyun", "heg_liushan"})
	card := sgGive(t, s, 1, "slash")
	wrong := sgGive(t, s, 1, "peach")
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_huoshui"})
	sgDo(t, s, 0, Action{Choice: "pass"})
	sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "heg_huoshui"})
	s.sgPush(SGEvent{Type: "response", Actor: 0, Target: 1, Kind: "slash", Count: 1, Amount: 1})
	s.sgRun()
	sgGodIllegal(t, s, 1, Action{Cards: []int{card}, Skill: "longdan"})
	if s.sgHegShown(1) {
		t.Fatal("Huoshui allowed a hidden conversion")
	}
	// The prohibition ends with the actual active turn, even while responses
	// remain. The identity card conversion validator itself never reveals.
	s.Sanguosha.ActivePhase = "inactive"
	if _, err := s.sgAs(1, []int{card}, "longdan", "jink"); err == nil || s.sgHegShown(1) {
		t.Fatal("read-only conversion query revealed private skills")
	}
	sgGodIllegal(t, s, 1, Action{Cards: []int{wrong}, Skill: "longdan"})
	sgRestoreMountain(t, s)
	sgDo(t, s, 1, Action{Cards: []int{card}, Skill: "longdan"})
	if s.Sanguosha.Players[1].Hegemony.Shown != [2]bool{true, false} || s.sgOwn(1, card, true) || s.Sanguosha.Players[1].HP != 3 {
		t.Fatal("valid Longdan response didn't reveal and defend")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyPrivateTriggerAndPass(t *testing.T) {
	for _, invoke := range []bool{false, true} {
		s := sgHegState(4)
		sgHegSetPair(s, 0, [2]string{"heg_tianfeng", "heg_jiling"})
		card := sgGive(t, s, 0, "jink")
		victim := sgGive(t, s, 1, "slash")
		s.sgSpent(0, []int{card})
		s.sgRun()
		for _, viewer := range []int{-1, 0, 1, 2} {
			v := s.sgView(viewer)["sanguosha"].(map[string]any)["pending"].(map[string]any)
			want := "hegemony_response"
			if viewer == 0 {
				want = "heg_sijian"
			}
			if v["kind"] != want {
				t.Fatal("private trigger leaked", viewer, v)
			}
		}
		if !invoke {
			sgDo(t, s, 0, Action{Choice: "pass"})
			if s.sgHegShown(0) || s.Sanguosha.Hegemony.FirstClaimed {
				t.Fatal("passing must keep the general hidden")
			}
		} else {
			sgGodIllegal(t, s, 0, Action{Targets: []int{2}})
			sgDo(t, s, 0, Action{Targets: []int{1}})
			if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_reward" {
				t.Fatal("first-show reward must precede the saved target-card choice", q)
			}
			sgRestoreMountain(t, s)
			sgDo(t, s, 0, Action{Choice: "pass"})
			if q := s.Sanguosha.Pending; q == nil || q.Kind != "steal" || q.Event.Kind != "heg_sijian" {
				t.Fatal("saved follow-up prompt was not restored", q)
			}
			sgDo(t, s, 0, Action{Choice: "hand"})
			if s.sgOwn(1, victim, true) || !s.sgHegShown(0) {
				t.Fatal("chosen skill did not resolve after reveal")
			}
		}
		sgHegConservation(t, s)
	}
}

func TestSanguoshaHegemonyHiddenMingshi(t *testing.T) {
	for _, invoke := range []bool{false, true} {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_kongrong", "heg_mateng"})
		s.sgPush(SGEvent{Type: "damage", Actor: 0, Target: 1, Kind: "slash", Amount: 1})
		s.sgRun()
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_mingshi" || s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("hidden compulsory skill must offer a reveal before damage", q)
		}
		a := Action{Choice: "pass"}
		if invoke {
			a.Choice = "yes"
		}
		sgDo(t, s, 1, a)
		sgPassAll(t, s)
		want := 2
		if invoke {
			want = 3
		}
		if s.sgHegShown(1) != invoke || s.Sanguosha.Players[1].HP != want {
			t.Fatal("Mingshi reveal decision")
		}
		sgHegConservation(t, s)
	}
}

func TestSanguoshaHegemonyHiddenShushenUsesOnlyOwnerKnowledge(t *testing.T) {
	s := sgHegState(6)
	sgHegSetPair(s, 0, [2]string{"heg_ganfuren", "heg_liubei"})
	s.sgHegShow(2, []int{0}, false)
	s.Sanguosha.Players[0].HP = 2
	s.sgHeal(0, 1)
	s.sgRun()
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_shushen" || !reflect.DeepEqual(q.Targets, []int{2}) {
		t.Fatal("hidden owner should be able to reveal and benefit a public future ally", q)
	}
	sgDo(t, s, 0, Action{Targets: []int{2}})
	sgPassAll(t, s)
	if !s.sgHegFriend(0, 2) || len(s.Sanguosha.Players[2].Hand) != 1 {
		t.Fatal("Shushen did not use the newly declared public faction")
	}
	sgHegConservation(t, s)
}
