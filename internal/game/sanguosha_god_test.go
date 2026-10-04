package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func sgGodState() *State {
	s := sgMountainState()
	s.Sanguosha.Options.Packs = append(s.Sanguosha.Options.Packs, "god")
	slices.Sort(s.Sanguosha.Deck)
	return s
}
func sgGodGeneral(s *State, i int, id string) {
	p := &s.Sanguosha.Players[i]
	p.General = id
	p.HP = sgGeneral(id).HP
	p.MaxHP = p.HP
	p.BaseKingdom = "qun"
}
func sgGodIllegal(t *testing.T, s *State, i int, a Action) {
	t.Helper()
	if s.Sanguosha.Pending != nil {
		a.Prompt = s.Sanguosha.Pending.ID
	}
	before, _ := json.Marshal(s)
	if err := s.Apply(i, a); err == nil {
		t.Fatal("expected rejected action", a)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("rejected action changed state")
	}
}
func sgGodStars(t *testing.T, s *State, i, n int) []int {
	t.Helper()
	ids := s.sgDrawIDs(n)
	s.Sanguosha.Players[i].Stars = append(s.Sanguosha.Players[i].Stars, ids...)
	return ids
}
func TestSanguoshaGodWushen(t *testing.T) {
	s := sgGodState()
	sgGodGeneral(s, 0, "shenguanyu")
	heart := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Kind == "peach" && c.Suit == 1 })
	if k, err := s.sgAs(0, []int{heart}, "", "slash"); err != nil || k != "slash" {
		t.Fatal(k, err)
	}
	if _, err := s.sgAs(0, []int{heart}, "", "peach"); err == nil {
		t.Fatal("heart peach bypassed Wushen")
	}
	if s.sgDistance(0, 2) != 2 {
		t.Fatal("fixture")
	}
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{heart}, Targets: []int{2}})
	if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "card" {
		t.Fatal("unlimited slash range", s.Sanguosha.Pending)
	}
	sgRestoreMountain(t, s)
	cards := s.sgVisibleCatalog(-1)
	if cards[heart-1].Kind != "slash" {
		t.Fatal("processing card lost filtered kind")
	}
	sgDo(t, s, 2, Action{Choice: "pass"})
	if s.sgVisibleCatalog(-1)[heart-1].Kind != "peach" {
		t.Fatal("filter persisted after discard")
	}
	another := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 1 })
	sgGodIllegal(t, s, 0, Action{Type: "sg_play", Cards: []int{another}, Targets: []int{2}})
	sgWindConservation(t, s)
}
func TestSanguoshaGodLonghunJuejing(t *testing.T) {
	s := sgGodState()
	sgGodGeneral(s, 0, "shenzhaoyun")
	heart1 := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 1 })
	heart2 := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 1 })
	if _, err := s.sgAs(0, []int{heart1}, "longhun", "peach"); err == nil {
		t.Fatal("wrong count")
	}
	if _, err := s.sgAs(0, []int{heart1, heart2}, "longhun", "peach"); err != nil {
		t.Fatal(err)
	}
	if s.sgHandLimit(0) != 4 {
		t.Fatal("hand limit")
	}
	s.Sanguosha.Players[0].HP = 1
	before := len(s.Sanguosha.Players[0].Hand)
	s.sgEvent(SGEvent{Type: "draw_phase", Actor: 0})
	if len(s.Sanguosha.Players[0].Hand) != before+3 {
		t.Fatal("normal draw bonus")
	}
	if _, err := s.sgAs(0, []int{heart1}, "longhun", "peach"); err != nil {
		t.Fatal(err)
	}
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{heart1}, Skill: "longhun"})
	if s.Sanguosha.Players[0].HP != 2 {
		t.Fatal("peach conversion")
	}
	// An equipment cost is paid as part of the converted card before its loss triggers.
	s.Sanguosha.Players[0].HP = 1
	equip := sgWear(t, s, 0, "silver_lion")
	if _, err := s.sgAs(0, []int{equip}, "longhun", "jink"); err != nil {
		t.Fatal(err)
	}
	s.sgAsk(0, "card", "test", SGEvent{Type: "response", Actor: 1, Target: 0, Kind: "slash", Count: 1})
	sgDo(t, s, 0, Action{Cards: []int{equip}, Skill: "longhun"})
	if s.Sanguosha.Players[0].HP != 2 || len(s.Sanguosha.Players[0].Equip) != 0 {
		t.Fatal("equipment response loss")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaGodShelieAndGongxinPrivacy(t *testing.T) {
	s := sgGodState()
	sgGodGeneral(s, 0, "shenlvmeng")
	for _, id := range []int{2, 79, 53, 27, 1} {
		s.Sanguosha.Deck = sgRemove(s.Sanguosha.Deck, id)
		s.Sanguosha.Deck = append([]int{id}, s.Sanguosha.Deck...)
	}
	s.sgEvent(SGEvent{Type: "draw_phase", Actor: 0})
	sgDo(t, s, 0, Action{Choice: "shelie"})
	q := s.Sanguosha.Pending
	if q == nil || q.Kind != "shelie" || len(q.Cards) != 5 {
		t.Fatal(q)
	}
	sgGodIllegal(t, s, 0, Action{Cards: nil})
	sgRestoreMountain(t, s)
	a, err := s.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	sgDo(t, s, 0, a)
	seen := map[int]bool{}
	for _, id := range s.Sanguosha.Players[0].Hand {
		if seen[sgCard(id).Suit] {
			t.Fatal("duplicate suit")
		}
		seen[sgCard(id).Suit] = true
	}
	sgGodGeneral(s, 1, "xiaoqiao")
	heart := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 0 })
	sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 1 })
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "gongxin", Targets: []int{1}})
	if len(s.Sanguosha.Revealed) > 0 {
		t.Fatal("private Gongxin published")
	}
	for _, viewer := range []int{-1, 0, 1, 2, 3} {
		v := s.sgView(viewer)["sanguosha"].(map[string]any)
		p := v["pending"].(map[string]any)
		if _, ok := p["cards"]; ok != (viewer == 0) {
			t.Fatal("prompt leak", viewer)
		}
		suit := s.sgVisibleCatalog(viewer)[heart-1].Suit
		if (suit == 1) != (viewer == 0 || viewer == 1) {
			t.Fatal("hidden effective suit leak", viewer, suit)
		}
	}
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Card: heart, Choice: "top"})
	if s.Sanguosha.Deck[0] != heart || sgCard(heart).Suit != 0 {
		t.Fatal("top deck or suit restoration")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaGodWuhun(t *testing.T) {
	t.Run("dead owner chooses and direct death bypasses rescue", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 1, "shenguanyu")
		s.Sanguosha.Players[1].HP = 1
		sgTop(t, s, func(c SGCard) bool { return c.Kind != "peach" && c.Kind != "god_salvation" })
		s.sgPush(SGEvent{Type: "damage", Actor: 3, Target: 1, Amount: 1})
		s.sgRun()
		if s.Sanguosha.Players[3].Marks["nightmare"] != 1 {
			t.Fatal("nightmare before dying")
		}
		for s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "peach" {
			sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
		}
		q := s.Sanguosha.Pending
		if q == nil || q.Kind != "wuhun_target" || q.Player != 1 {
			t.Fatal(q)
		}
		sgRestoreMountain(t, s)
		sgGodIllegal(t, s, 2, Action{Targets: []int{3}})
		a, err := s.BotAction(1)
		if err != nil {
			t.Fatal(err)
		}
		sgDo(t, s, 1, a)
		if !s.Sanguosha.Players[3].Dead || s.Sanguosha.Pending != nil {
			t.Fatal("did not die directly", s.Sanguosha.Pending)
		}
		if s.Sanguosha.Players[3].Marks["nightmare"] != 0 {
			t.Fatal("nightmare not cleared")
		}
		sgGodIllegal(t, s, 1, Action{Type: "sg_end"})
		sgWindConservation(t, s)
	})
	t.Run("terminal lord death does not reopen match", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenguanyu")
		s.sgGodAddMark(1, "nightmare", 5)
		s.sgDie(0, 1)
		s.sgRun()
		if !s.Finished || s.Sanguosha.Pending != nil || s.Sanguosha.Players[1].Dead {
			t.Fatal("terminal Wuhun")
		}
	})
	t.Run("successful judgment survives and mark count uses armor", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 1, "shenguanyu")
		sgWear(t, s, 1, "silver_lion")
		s.sgDamage(SGEvent{Actor: 3, Target: 1, Amount: 3})
		s.sgRun()
		if s.Sanguosha.Players[3].Marks["nightmare"] != 1 {
			t.Fatal("unmitigated nightmare")
		}
		sgTop(t, s, func(c SGCard) bool { return c.Kind == "god_salvation" })
		s.sgDie(1, 3)
		s.sgRun()
		sgDo(t, s, 1, Action{Targets: []int{3}})
		if s.Sanguosha.Players[3].Dead {
			t.Fatal("good Wuhun judgment")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaGodGuixin(t *testing.T) {
	s := sgGodState()
	sgGodGeneral(s, 0, "shencaocao")
	s.Sanguosha.Players[0].HP = 3
	first := sgGive(t, s, 1, "slash")
	second := sgWear(t, s, 2, "crossbow")
	third := sgGive(t, s, 3, "indulgence")
	s.Sanguosha.Players[3].Hand = nil
	s.Sanguosha.Players[3].Judgment = []SGDelayed{{Card: third, Kind: "indulgence"}}
	if s.sgDistance(1, 0) != 2 {
		t.Fatal("Feiying")
	}
	s.sgDamage(SGEvent{Actor: 1, Target: 0, Amount: 2})
	s.sgRun()
	sgDo(t, s, 0, Action{Choice: "yes"})
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Choice: "hand"})
	sgDo(t, s, 0, Action{Card: second})
	sgDo(t, s, 0, Action{Card: third})
	if !s.Sanguosha.Players[0].Flipped {
		t.Fatal("first point flip")
	}
	sgDo(t, s, 0, Action{Choice: "yes"})
	if s.Sanguosha.Players[0].Flipped || !sgSubset([]int{first, second, third}, s.Sanguosha.Players[0].Hand) {
		t.Fatal("second point or loot")
	}
	sgWindConservation(t, s)
}

func TestSanguoshaGodQixing(t *testing.T) {
	s := sgGodState()
	sgGodGeneral(s, 0, "shenzhugeliang")
	s.Sanguosha.Players[0].BaseKingdom = ""
	s.sgPush(SGEvent{Type: "god_kingdom", Actor: 0}, SGEvent{Type: "god_initial_draw", Actor: 0})
	s.sgRun()
	sgGodIllegal(t, s, 0, Action{Choice: "god"})
	sgDo(t, s, 0, Action{Choice: "shu"})
	if s.sgKingdom(0) != "shu" || len(s.Sanguosha.Players[0].Hand) != 11 {
		t.Fatal("kingdom or initial count")
	}
	sgRestoreMountain(t, s)
	a, err := s.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	sgDo(t, s, 0, a)
	p := s.Sanguosha.Players[0]
	if len(p.Stars) != 7 || len(p.Hand) != 4 {
		t.Fatal("Qixing deal")
	}
	s.sgEvent(SGEvent{Type: "qixing_exchange", Actor: 0})
	oldHand, oldStars := clone(p.Hand), clone(p.Stars)
	sgGodIllegal(t, s, 0, Action{Cards: oldHand[:1], Take: oldStars[:2]})
	for _, viewer := range []int{-1, 0, 1} {
		v := s.sgView(viewer)["sanguosha"].(map[string]any)
		seat := v["players"].([]map[string]any)[0]
		if _, ok := seat["stars"]; ok != (viewer == 0) {
			t.Fatal("stars leak", viewer)
		}
		if seat["starCount"].(int) != 7 {
			t.Fatal("public count")
		}
	}
	sgDo(t, s, 0, Action{Cards: oldHand[:2], Take: oldStars[:2]})
	if !sgSubset(oldHand[:2], s.Sanguosha.Players[0].Stars) || !sgSubset(oldStars[:2], s.Sanguosha.Players[0].Hand) {
		t.Fatal("exchange")
	}
	sgRestoreMountain(t, s)
	s.sgPush(SGEvent{Type: "god_finish", Actor: 0})
	s.sgRun()
	sgDo(t, s, 0, Action{Cards: s.Sanguosha.Players[0].Stars[:1], Targets: []int{1}})
	sgDo(t, s, 0, Action{Cards: s.Sanguosha.Players[0].Stars[:2], Targets: []int{0, 2}})
	if len(s.Sanguosha.Players[0].Stars) != 4 {
		t.Fatal("star costs")
	}
	sgWindConservation(t, s)
	s.Sanguosha.Players[0].Flipped = true
	s.sgEvent(SGEvent{Type: "begin", Actor: 0})
	if len(s.Sanguosha.Players[0].Gale) != 1 || len(s.Sanguosha.Players[0].Fog) != 2 {
		t.Fatal("face-down skip ended effects")
	}
	s.Sanguosha.Queue = nil
	s.sgEvent(SGEvent{Type: "begin", Actor: 0})
	if len(s.Sanguosha.Players[0].Gale)+len(s.Sanguosha.Players[0].Fog) != 0 {
		t.Fatal("effects not expired")
	}
	s.Sanguosha.Queue = nil
	s.sgLoseSkills(0)
	if len(s.Sanguosha.Players[0].Stars) != 0 {
		t.Fatal("lost Qixing retained stars")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaGodWeather(t *testing.T) {
	t.Run("fog before Tianxiang and armor, thunder bypass", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenzhugeliang")
		sgGodGeneral(s, 1, "xiaoqiao")
		s.Sanguosha.Players[0].Fog = []int{1}
		sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 1 })
		s.sgDamage(SGEvent{Actor: 2, Target: 1, Amount: 1, Nature: "fire"})
		s.sgRun()
		if s.Sanguosha.Pending != nil || s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("fog didn't preempt damage")
		}
		s.sgDamage(SGEvent{Actor: 2, Target: 1, Amount: 1, Nature: "thunder"})
		s.sgRun()
		if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "tianxiang" {
			t.Fatal("thunder did not reach Tianxiang")
		}
		sgDo(t, s, 1, Action{Choice: "pass"})
		if s.Sanguosha.Players[1].HP != 2 {
			t.Fatal("thunder damage")
		}
	})
	t.Run("gale not doubled after response but applied at new recipient", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenzhugeliang")
		sgGodGeneral(s, 1, "xiaoqiao")
		s.Sanguosha.Players[0].Gale = []int{1, 2}
		heart := sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 1 })
		s.sgDamage(SGEvent{Actor: 3, Target: 1, Amount: 1, Nature: "fire"})
		s.sgRun()
		sgRestoreMountain(t, s)
		sgDo(t, s, 1, Action{Cards: []int{heart}, Targets: []int{2}})
		if s.Sanguosha.Players[2].HP != 1 || s.Sanguosha.Players[1].HP != 3 {
			t.Fatal("transfer gale")
		}
		s.sgDamage(SGEvent{Actor: 3, Target: 1, Amount: 1, Nature: "fire"})
		s.sgRun()
		// Tianxiang no longer has a heart to discard.
		if s.Sanguosha.Players[1].HP != 1 {
			t.Fatal("recipient gale")
		}
	})
	t.Run("Tianxiang pass resumes exactly once", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 1, "xiaoqiao")
		s.Sanguosha.Players[0].Gale = []int{1}
		sgFindGive(t, s, 1, func(c SGCard) bool { return c.Suit == 1 })
		s.sgDamage(SGEvent{Actor: 2, Target: 1, Amount: 1, Nature: "fire"})
		s.sgRun()
		sgRestoreMountain(t, s)
		sgDo(t, s, 1, Action{Choice: "pass"})
		if s.Sanguosha.Players[1].HP != 1 {
			t.Fatal("double gale after resume")
		}
	})
}
func TestSanguoshaGodWar(t *testing.T) {
	t.Run("Qinyin counts discards, not responses", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenzhouyu")
		s.Sanguosha.DiscardPhase = true
		ids := []int{sgGive(t, s, 0, "slash"), sgGive(t, s, 0, "jink")}
		response := sgGive(t, s, 0, "jink")
		s.sgSpent(0, []int{response})
		s.sgDiscard(0, ids)
		s.sgPush(SGEvent{Type: "qinyin", Actor: 0}, SGEvent{Type: "guzheng", Actor: 0})
		s.sgRun()
		if s.Sanguosha.DiscardedOwn != 2 || s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "qinyin" {
			t.Fatal("Qinyin counter")
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 0, Action{Choice: "lose"})
		for _, p := range s.Sanguosha.Players {
			if p.HP != 3 {
				t.Fatal("Qinyin loss")
			}
		}
		sgWindConservation(t, s)
	})
	t.Run("Great Yeyan completes rescue before fire", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenzhouyu")
		s.Sanguosha.Players[0].HP = 3
		ids := []int{}
		for suit := range 4 {
			ids = append(ids, sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == suit }))
		}
		sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "yeyan", Cards: ids[:3], Targets: []int{1}})
		peach := sgGive(t, s, 2, "peach")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "yeyan", Cards: ids, Targets: []int{1, 3}})
		if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "peach" || s.Sanguosha.Players[1].HP != 4 {
			t.Fatal("cost rescue order")
		}
		sgDo(t, s, 0, Action{Choice: "pass"})
		sgDo(t, s, 1, Action{Choice: "pass"})
		sgRestoreMountain(t, s)
		sgDo(t, s, 2, Action{Cards: []int{peach}})
		if s.Sanguosha.Players[0].HP != 1 || s.Sanguosha.Players[1].HP != 2 || s.Sanguosha.Players[3].HP != 3 {
			t.Fatal("Great Yeyan distribution")
		}
		sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "yeyan", Targets: []int{1}})
		sgWindConservation(t, s)
	})
	t.Run("Wumou pauses nullification and keeps effect after payment", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 1, "shenlvbu")
		s.sgGodAddMark(1, "wrath", 2)
		trick := sgGive(t, s, 0, "ex_nihilo")
		counter := sgGive(t, s, 1, "nullification")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{trick}})
		sgDo(t, s, 1, Action{Cards: []int{counter}})
		if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "wumou" {
			t.Fatal("nullification did not trigger Wumou")
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 1, Action{Choice: "wrath"})
		sgPassNull(t, s)
		if s.Sanguosha.Players[1].Marks["wrath"] != 1 || len(s.Sanguosha.Players[0].Hand) != 0 {
			t.Fatal("Wumou changed effect")
		}
		sgWindConservation(t, s)
	})
	t.Run("Wuqian disables armor globally and preserves acquired Wushuang", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenlvbu")
		s.sgAcquire(0, "wushuang")
		s.sgGodAddMark(0, "wrath", 4)
		lion := sgWear(t, s, 1, "silver_lion")
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "wuqian", Targets: []int{1}})
		s.sgDamage(SGEvent{Actor: 2, Target: 1, Amount: 2})
		s.sgRun()
		if s.Sanguosha.Players[1].HP != 2 {
			t.Fatal("armor still active for third player")
		}
		s.sgDiscard(1, []int{lion})
		s.sgRun()
		if s.Sanguosha.Players[1].HP != 2 {
			t.Fatal("disabled lion healed")
		}
		s.sgGodTurnCleanup()
		if s.sgGodArmorOff(1) || !s.sgHas(0, "wushuang") {
			t.Fatal("cleanup lost permanent skill")
		}
		sgWindConservation(t, s)
	})
	t.Run("Shenfen has damage, equipment, then hand batches", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenlvbu")
		s.sgGodAddMark(0, "wrath", 6)
		for j, i := range []int{1, 2, 3} {
			sgWear(t, s, i, []string{"crossbow", "spear", "blade"}[j])
		}
		for _, i := range []int{1, 2, 3} {
			s.sgDraw(i, 5)
		}
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "shenfen"})
		q := s.Sanguosha.Pending
		if q == nil || q.Kind != "shenfen_hand" || q.Player != 1 {
			t.Fatal(q)
		}
		for _, i := range []int{1, 2, 3} {
			if s.Sanguosha.Players[i].HP != 3 || len(s.Sanguosha.Players[i].Equip) != 0 || len(s.Sanguosha.Players[i].Hand) != 5 {
				t.Fatal("batch order", i)
			}
		}
		sgRestoreMountain(t, s)
		for s.Sanguosha.Pending != nil {
			a, err := s.BotAction(s.SanguoshaActor())
			if err != nil {
				t.Fatal(err)
			}
			sgDo(t, s, s.SanguoshaActor(), a)
		}
		if !s.Sanguosha.Players[0].Flipped || s.Sanguosha.Players[0].Marks["wrath"] != 3 {
			t.Fatal("flip or caused-damage wrath")
		}
		sgWindConservation(t, s)
	})
}
func TestSanguoshaGodSima(t *testing.T) {
	s := sgGodState()
	sgGodGeneral(s, 0, "shensimayi")
	if s.sgHas(0, "jilve") {
		t.Fatal("Jilve acquired before Baiyin")
	}
	s.Sanguosha.DiscardPhase = true
	s.sgDraw(0, 4)
	s.sgDiscard(0, clone(s.Sanguosha.Players[0].Hand))
	s.Sanguosha.DiscardPhase = false
	if s.Sanguosha.Players[0].Marks["bear"] != 4 {
		t.Fatal("Renjie discard")
	}
	s.sgEvent(SGEvent{Type: "god_start", Actor: 0})
	if !s.sgHas(0, "jilve") || s.Sanguosha.Players[0].MaxHP != 3 {
		t.Fatal("Baiyin")
	}
	card := sgGive(t, s, 0, "slash")
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jilve", Choice: "zhiheng", Cards: []int{card}})
	if s.Sanguosha.Players[0].Marks["bear"] != 3 || len(s.Sanguosha.Players[0].Hand) != 1 {
		t.Fatal("Jilve Zhiheng")
	}
	sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "jilve", Choice: "zhiheng", Cards: s.Sanguosha.Players[0].Hand})
	sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "jilve", Choice: "wansha"})
	if !s.sgHas(0, "wansha") {
		t.Fatal("temporary Wansha")
	}
	s.sgAcquire(0, "wansha")
	s.sgGodTurnCleanup()
	if !s.sgHas(0, "wansha") {
		t.Fatal("permanent Wansha removed")
	}
	// Jilve's pinned Jizhi is the reveal/exchange version, including delayed tricks.
	trick := sgGive(t, s, 0, "indulgence")
	revealed := sgTop(t, s, func(c SGCard) bool { return c.Kind == "jink" })
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{trick}, Targets: []int{1}})
	if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "jilve_jizhi" {
		t.Fatal("delayed trick Jizhi")
	}
	sgDo(t, s, 0, Action{Choice: "yes"})
	q := s.Sanguosha.Pending
	if q == nil || q.Kind != "jilve_jizhi_exchange" || q.Cards[0] != revealed {
		t.Fatal(q)
	}
	put := s.Sanguosha.Players[0].Hand[0]
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Cards: []int{put}})
	if s.Sanguosha.Deck[0] != put || !slices.Contains(s.Sanguosha.Players[0].Hand, revealed) || len(s.Sanguosha.Players[1].Judgment) != 1 {
		t.Fatal("Jizhi exchange or continuation")
	}
	sgWindConservation(t, s)
}
func TestSanguoshaGodLianpo(t *testing.T) {
	s := sgGodState()
	sgGodGeneral(s, 2, "shensimayi")
	s.sgDie(1, 2)
	s.sgRun()
	if s.Sanguosha.Players[2].TurnKills != 1 {
		t.Fatal("out of turn kill not counted")
	}
	s.sgPush(SGEvent{Type: "lianpo", Actor: 0}, SGEvent{Type: "next", Actor: 0})
	s.sgRun()
	if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "lianpo" || s.Sanguosha.Pending.Player != 2 {
		t.Fatal("out of turn Lianpo")
	}
	sgRestoreMountain(t, s)
	sgDo(t, s, 2, Action{Choice: "yes"})
	if s.Turn != 2 || s.Sanguosha.Players[2].TurnKills != 0 {
		t.Fatal("extra turn or counter reset")
	}
	// Normal seating resumes after the original actor, skipping the dead seat.
	sgDo(t, s, 2, Action{Type: "sg_end"})
	sgPassAll(t, s)
	if s.Turn != 2 || s.Sanguosha.TurnSequence != 2 {
		t.Fatal("extra then normal same seat", s.Turn, s.Sanguosha.TurnSequence)
	}
	sgWindConservation(t, s)
}

func TestSanguoshaGodBotsComplete(t *testing.T) {
	for offset := range 10 {
		s := sgGodState()
		if offset >= 8 {
			var err error
			s, err = NewSanguosha(8, SGOptions{Deck: "military", Packs: []string{"wind", "fire", "thicket", "mountain"}})
			if err != nil {
				t.Fatal(err)
			}
			s.Sanguosha.Pending = nil
			s.Sanguosha.Queue = nil
			s.Sanguosha.Selecting = false
			s.Sanguosha.Selected = 8
			s.Sanguosha.Lord = 0
			s.Turn = 0
			s.Sanguosha.Options.Packs = append(s.Sanguosha.Options.Packs, "god")
		}
		for i := range s.Sanguosha.Players {
			sgGodGeneral(s, i, sgGodGenerals[(offset+i)%8].ID)
			s.Sanguosha.Players[i].BaseKingdom = ""
			if offset >= 8 {
				s.Sanguosha.Players[i].Role = []string{"lord", "rebel", "loyalist", "rebel", "renegade", "loyalist", "rebel", "rebel"}[i]
			}
		}
		shuffle(s.Sanguosha.Deck)
		s.sgGodGameStart()
		s.sgRun()
		for step := 0; step < 12000 && !s.Finished; step++ {
			i := s.SanguoshaActor()
			a, err := s.BotAction(i)
			if err != nil {
				t.Fatalf("offset %d step %d prompt %+v: %v", offset, step, s.Sanguosha.Pending, err)
			}
			if err = s.Apply(i, a); err != nil {
				t.Fatal(a, err)
			}
			sgWindConservation(t, s)
			if step%97 == 0 {
				sgRestoreMountain(t, s)
			}
			if s.Finished {
				t.Logf("offset %d finished in %d actions", offset, step+1)
			}
		}
		if !s.Finished {
			t.Fatalf("offset %d stalled: turn %d pending %+v", offset, s.Turn, s.Sanguosha.Pending)
		}
	}
}

func TestSanguoshaGodInterruptionsAndDynamicSkills(t *testing.T) {
	t.Run("Jilve Guicai equipment cost survives save", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shensimayi")
		s.sgAcquire(0, "jilve")
		s.sgGodAddMark(0, "bear", 2)
		sgGive(t, s, 0, "slash")
		replacement := sgWear(t, s, 0, "crossbow")
		s.sgPush(SGEvent{Type: "judge", Actor: 1, Kind: "indulgence"})
		s.sgRun()
		q := s.Sanguosha.Pending
		if q == nil || q.Kind != "jilve_guicai" {
			t.Fatal(q)
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 0, Action{Cards: []int{replacement}})
		if len(s.Sanguosha.Players[0].Equip) != 0 || s.Sanguosha.Players[0].Marks["bear"] != 1 {
			t.Fatal("Jilve Guicai cost")
		}
		sgWindConservation(t, s)
	})
	t.Run("Renjie on damage funds Jilve Fangzhu", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shensimayi")
		s.sgAcquire(0, "jilve")
		s.sgDamage(SGEvent{Actor: 1, Target: 0, Amount: 2})
		s.sgRun()
		if s.Sanguosha.Players[0].Marks["bear"] != 2 || s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "jilve_fangzhu" {
			t.Fatal("Renjie/Fangzhu")
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 0, Action{Targets: []int{1}})
		if len(s.Sanguosha.Players[1].Hand) != 2 || !s.Sanguosha.Players[1].Flipped || s.Sanguosha.Players[0].Marks["bear"] != 1 {
			t.Fatal("Fangzhu effect")
		}
		sgWindConservation(t, s)
	})
	t.Run("Wumou dying completes before trick resolves", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenlvbu")
		s.Sanguosha.Players[0].HP = 1
		trick := sgGive(t, s, 0, "ex_nihilo")
		peach := sgGive(t, s, 2, "peach")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{trick}})
		if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "peach" || s.Sanguosha.Players[0].HP != 0 || len(s.Sanguosha.Players[0].Hand) != 0 {
			t.Fatal("Wumou dying order")
		}
		sgDo(t, s, 0, Action{Choice: "pass"})
		sgDo(t, s, 1, Action{Choice: "pass"})
		sgRestoreMountain(t, s)
		sgDo(t, s, 2, Action{Cards: []int{peach}})
		sgPassNull(t, s)
		if s.Sanguosha.Players[0].HP != 1 || len(s.Sanguosha.Players[0].Hand) != 2 {
			t.Fatal("trick did not resume")
		}
		sgWindConservation(t, s)
	})
	t.Run("dead current actor still reaches Lianpo", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 2, "shensimayi")
		s.Turn = 1
		s.sgDie(1, 2)
		s.sgRun()
		if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "lianpo" {
			t.Fatal("dead active player skipped Lianpo", s.Sanguosha.Pending)
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 2, Action{Choice: "yes"})
		if s.Turn != 2 {
			t.Fatal("extra turn")
		}
	})
	t.Run("god Huashen restores native kingdom and clears lost stars", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 0, "shenzhaoyun")
		s.Sanguosha.Players[0].BaseKingdom = "wei"
		s.sgAcquire(0, "huashen")
		s.Sanguosha.Players[0].Avatars = []string{"shenzhugeliang", "xiaoqiao"}
		if slices.Contains(sgAvatarSkills("shenzhouyu"), "yeyan") || slices.Contains(sgAvatarSkills("shensimayi"), "baiyin") {
			t.Fatal("limited or awakening copied")
		}
		s.sgAskHuashen(0, true)
		sgDo(t, s, 0, Action{Choice: "shenzhugeliang", Skill: "qixing"})
		if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "god_kingdom" {
			t.Fatal("god avatar kingdom")
		}
		sgDo(t, s, 0, Action{Choice: "wu"})
		if s.sgKingdom(0) != "wu" {
			t.Fatal("avatar kingdom")
		}
		s.sgEvent(SGEvent{Type: "god_initial_draw", Actor: 0})
		sgDo(t, s, 0, Action{Cards: s.Sanguosha.Players[0].Hand[:7]})
		s.sgAskHuashen(0, false)
		sgDo(t, s, 0, Action{Choice: "xiaoqiao", Skill: "hongyan"})
		if len(s.Sanguosha.Players[0].Stars) != 0 || !s.sgFemale(0) {
			t.Fatal("old Qixing pile or identity")
		}
		s.sgLoseSkills(0)
		if s.sgKingdom(0) != "wei" || s.sgFemale(0) {
			t.Fatal("native God kingdom not restored")
		}
		sgWindConservation(t, s)
	})
	t.Run("Wuhun nested death sees marks before final cleanup", func(t *testing.T) {
		s := sgGodState()
		sgGodGeneral(s, 1, "shenguanyu")
		sgGodGeneral(s, 3, "shenguanyu")
		s.sgGodAddMark(3, "nightmare", 3)
		s.sgGodAddMark(2, "nightmare", 1)
		sgTop(t, s, func(c SGCard) bool { return c.Kind == "slash" })
		s.sgDie(1, 0)
		s.sgRun()
		sgDo(t, s, 1, Action{Targets: []int{3}})
		if s.Sanguosha.Pending == nil || s.Sanguosha.Pending.Kind != "wuhun_target" || s.Sanguosha.Pending.Player != 3 {
			t.Fatal("nested Wuhun lost marks", s.Sanguosha.Pending)
		}
		sgRestoreMountain(t, s)
		sgTop(t, s, func(c SGCard) bool { return c.Kind == "god_salvation" })
		sgDo(t, s, 3, Action{Targets: []int{2}})
		if s.Sanguosha.Players[2].Dead || s.Sanguosha.Players[2].Marks["nightmare"] != 0 {
			t.Fatal("nested survival or cleanup")
		}
		sgWindConservation(t, s)
	})
}

func TestSanguoshaGodDeadTrickUser(t *testing.T) {
	for _, kind := range []string{"snatch", "dismantlement", "collateral"} {
		t.Run(kind, func(t *testing.T) {
			s := sgGodState()
			s.Turn = 1
			sgGodGeneral(s, 1, "shenlvbu")
			s.Sanguosha.Players[1].HP = 1
			trick := sgGive(t, s, 1, kind)
			weapon := sgWear(t, s, 2, "crossbow")
			targets := []int{2}
			if kind == "collateral" {
				targets = append(targets, 3)
			}
			sgDo(t, s, 1, Action{Type: "sg_play", Cards: []int{trick}, Targets: targets})
			for s.Sanguosha.Pending != nil && s.Sanguosha.Pending.Kind == "peach" {
				sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
			}
			if !s.Sanguosha.Players[1].Dead {
				t.Fatal("Wumou did not kill user")
			}
			sgRestoreMountain(t, s)
			sgPassNull(t, s)
			if kind == "collateral" {
				sgDo(t, s, 2, Action{Choice: "pass"})
			}
			if s.Sanguosha.Pending != nil || !slices.Contains(s.Sanguosha.Players[2].Equip, weapon) || len(s.Sanguosha.Players[1].Hand) > 0 {
				t.Fatal("dead user prompted or obtained cards", s.Sanguosha.Pending)
			}
			sgWindConservation(t, s)
		})
	}
}
