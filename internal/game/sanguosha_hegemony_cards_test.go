package game

import (
	"slices"
	"testing"
)

func sgHegConservation(t *testing.T, s *State) {
	t.Helper()
	g := s.Sanguosha
	ids := append(append(append(clone(g.Deck), g.Discard...), g.Table...), g.Grace...)
	for _, p := range g.Players {
		ids = append(ids, p.Hand...)
		ids = append(ids, p.Equip...)
		ids = append(ids, p.Buqu...)
		for _, d := range p.Judgment {
			ids = append(ids, d.Card)
		}
	}
	if q := g.Pending; q != nil && (q.Kind == "guanxing" || q.Kind == "yiji") {
		ids = append(ids, q.Cards...)
	}
	slices.Sort(ids)
	if len(ids) != 108 {
		t.Fatal("national-war physical card count", len(ids), g.Pending)
	}
	for j, id := range ids {
		if id != j+1001 {
			t.Fatal("duplicate or missing card", id, j+1001)
		}
	}
}

func TestSanguoshaHegemonyAwaitOrderAndCounterScope(t *testing.T) {
	for counters := 0; counters <= 3; counters++ {
		s := sgHegState(6)
		s.sgHegShow(0, []int{0}, false)
		s.sgHegShow(4, []int{0}, false)
		card := sgGive(t, s, 0, "await_exhausted")
		heg := sgGive(t, s, 1, "heg_nullification")
		normal := sgGive(t, s, 2, "nullification")
		last := sgGive(t, s, 3, "heg_nullification")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{card}})
		if counters > 0 {
			sgDo(t, s, 1, Action{Cards: []int{heg}, Choice: "faction"})
		}
		if counters > 1 {
			sgGodIllegal(t, s, 2, Action{Cards: []int{normal}, Choice: "faction"})
			sgDo(t, s, 2, Action{Cards: []int{normal}})
		}
		if counters > 2 {
			sgGodIllegal(t, s, 3, Action{Cards: []int{last}, Choice: "faction"})
			sgDo(t, s, 3, Action{Cards: []int{last}, Choice: "single"})
		}
		sgRestoreMountain(t, s)
		sgPassNull(t, s)
		if counters%2 == 1 {
			if s.Sanguosha.Pending != nil || len(s.Sanguosha.Players[0].Hand)+len(s.Sanguosha.Players[4].Hand) != 0 {
				t.Fatal("faction counter did not cancel all upcoming allied targets", counters, s.Sanguosha.Pending)
			}
		} else {
			q := s.Sanguosha.Pending
			if q == nil || q.Kind != "heg_await_discard" || q.Player != 0 || len(s.Sanguosha.Players[4].Hand) != 2 {
				t.Fatal("discard started before all allied draws", counters, q)
			}
			sgDo(t, s, 0, Action{Cards: clone(s.Sanguosha.Players[0].Hand)})
			sgRestoreMountain(t, s)
			sgDo(t, s, 4, Action{Cards: clone(s.Sanguosha.Players[4].Hand)})
		}
		if len(s.Sanguosha.Hegemony.AwaitEffects) != 0 {
			t.Fatal("await flags survived card cleanup")
		}
		sgHegConservation(t, s)
	}
}

func TestSanguoshaHegemonyKnownBothPrivacy(t *testing.T) {
	for _, choice := range []string{"hand", "head", "deputy"} {
		s := sgHegState(4)
		card := sgGive(t, s, 0, "known_both")
		secret := sgGive(t, s, 1, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{card}, Targets: []int{1}})
		sgPassNull(t, s)
		sgDo(t, s, 0, Action{Choice: choice})
		sgRestoreMountain(t, s)
		for _, viewer := range []int{-1, 0, 1, 2, 3} {
			v := s.sgView(viewer)["sanguosha"].(map[string]any)
			q := v["pending"].(map[string]any)
			if _, ok := q["general"]; ok != (viewer == 0 && choice != "hand") {
				t.Fatal("known general leaked in prompt", choice, viewer, q)
			}
			if cards, _ := q["cards"].([]int); slices.Contains(cards, secret) != (viewer == 0 && choice == "hand") {
				t.Fatal("known hand leaked", choice, viewer, q)
			}
			seat := v["players"].([]map[string]any)[0]
			if _, ok := seat["knownGenerals"]; ok != (viewer == 0) {
				t.Fatal("saved private knowledge leaked", viewer)
			}
		}
		if s.sgHegShown(1) {
			t.Fatal("private inspection revealed target's faction")
		}
		sgDo(t, s, 0, Action{Choice: "ok"})
		if choice != "hand" && len(s.Sanguosha.Players[0].Hegemony.KnownGenerals) != 1 {
			t.Fatal("private general knowledge lost")
		}
		sgHegConservation(t, s)
	}
	t.Run("recast doesn't trigger Jizhi", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 0, [2]string{"heg_huangyueying", "heg_zhugeliang"})
		s.sgHegShow(0, []int{0}, false)
		card := sgGive(t, s, 0, "known_both")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{card}})
		if s.Sanguosha.Pending != nil || len(s.Sanguosha.Players[0].Hand) != 1 {
			t.Fatal("recast counted as trick use")
		}
		sgHegConservation(t, s)
	})
}

func TestSanguoshaHegemonyBefriendAndWeapons(t *testing.T) {
	t.Run("Befriend requires both public factions", func(t *testing.T) {
		s := sgHegState(4)
		card := sgGive(t, s, 0, "befriend_attacking")
		a := Action{Type: "sg_play", Cards: []int{card}, Targets: []int{1}}
		sgGodIllegal(t, s, 0, a)
		s.sgHegShow(0, []int{0}, false)
		sgGodIllegal(t, s, 0, a)
		s.sgHegShow(1, []int{0}, false)
		sgDo(t, s, 0, a)
		sgPassNull(t, s)
		if len(s.Sanguosha.Players[0].Hand) != 3 || len(s.Sanguosha.Players[1].Hand) != 1 {
			t.Fatal("Befriend draw counts")
		}
		sgHegConservation(t, s)
	})
	t.Run("SixSwords aids only other public allies", func(t *testing.T) {
		s := sgHegState(6)
		sgWear(t, s, 0, "six_swords")
		s.sgHegShow(0, []int{0}, false)
		if s.sgRange(0) != 2 || s.sgRange(4) != 1 {
			t.Fatal("sword affected self or secret ally")
		}
		s.sgHegShow(4, []int{0}, false)
		if s.sgRange(4) != 2 || s.sgRange(1) != 1 {
			t.Fatal("ally range bonus")
		}
	})
	t.Run("Triblade affects a third seat and cannot chain", func(t *testing.T) {
		s := sgHegState(4)
		sgWear(t, s, 0, "triblade")
		slash := sgGive(t, s, 0, "slash")
		cost := sgGive(t, s, 0, "jink")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{slash}, Targets: []int{1}})
		sgDo(t, s, 1, Action{Choice: "pass"})
		q := s.Sanguosha.Pending
		if q == nil || q.Kind != "heg_triblade" || !slices.Equal(q.Targets, []int{2}) {
			t.Fatal("third-seat weapon target", q)
		}
		sgGodIllegal(t, s, 0, Action{Cards: []int{cost}, Targets: []int{1}})
		sgRestoreMountain(t, s)
		sgDo(t, s, 0, Action{Cards: []int{cost}, Targets: []int{2}})
		if s.Sanguosha.Players[2].HP != 3 || s.Sanguosha.Pending != nil {
			t.Fatal("weapon damage or recursion")
		}
		sgHegConservation(t, s)
	})
}

func TestSanguoshaHegemonyDuoshiAndJizhi(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_huangyueying", "heg_wolong"})
	s.sgHegShow(0, []int{0, 1}, false)
	card := sgGive(t, s, 0, "known_both")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{card}, Targets: []int{1}})
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "invoke" || q.Event.Kind != "heg_jizhi" {
		t.Fatal("physical trick didn't trigger national-war Jizhi", q)
	}
	sgDo(t, s, 0, Action{Choice: "pass"})
	sgPassNull(t, s)
	sgDo(t, s, 0, Action{Choice: "head"})
	sgDo(t, s, 0, Action{Choice: "ok"})
	red := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 1 && c.Kind != "fire_attack" })
	sgGive(t, s, 1, "jink")
	sgDo(t, s, 0, Action{Type: "sg_play", Skill: "huoji", Cards: []int{red}, Targets: []int{1}})
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "nullification" {
		t.Fatal("converted trick triggered Jizhi", q)
	}
	sgHegConservation(t, s)
	s = sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_luxun", "heg_ganning"})
	s.sgHegShow(0, []int{0}, false)
	for use := 0; use < 5; use++ {
		red := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == 1 || c.Suit == 3 })
		a := Action{Type: "sg_play", Skill: "heg_duoshi", Cards: []int{red}}
		if use == 4 {
			sgGodIllegal(t, s, 0, a)
			break
		}
		sgDo(t, s, 0, a)
		sgPassNull(t, s)
		sgDo(t, s, 0, Action{Cards: clone(s.Sanguosha.Players[0].Hand[:2])})
		sgHegConservation(t, s)
	}
}
