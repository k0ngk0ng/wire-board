package game

import "testing"

func TestSanguoshaHegemonyRevisedActiveSkills(t *testing.T) {
	t.Run("Rende heals at three given cards only", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 0, [2]string{"heg_liubei", "heg_zhangfei"})
		s.sgHegShow(0, []int{0}, false)
		s.Sanguosha.Players[0].HP = 2
		s.sgDraw(0, 5)
		ids := clone(s.Sanguosha.Players[0].Hand)
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_rende", Cards: ids[:2], Targets: []int{1}})
		if s.Sanguosha.Players[0].HP != 2 {
			t.Fatal("used identity Rende threshold")
		}
		sgRestoreMountain(t, s)
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_rende", Cards: ids[2:3], Targets: []int{1}})
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_rende", Cards: ids[3:], Targets: []int{1}})
		if s.Sanguosha.Players[0].HP != 3 || len(s.Sanguosha.Players[1].Hand) != 5 {
			t.Fatal("Rende repeated healing or bad delivery")
		}
	})
	t.Run("Zhiheng uses max HP and remains once per phase", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 0, [2]string{"heg_sunquan", "heg_lusu"})
		s.sgHegShow(0, []int{0}, false)
		s.sgDraw(0, 4)
		ids := clone(s.Sanguosha.Players[0].Hand)
		sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "heg_zhiheng", Cards: ids})
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_zhiheng", Cards: ids[:3]})
		sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "heg_zhiheng", Cards: ids[3:]})
		if len(s.Sanguosha.Players[0].Hand) != 4 || len(s.Sanguosha.Discard) != 3 {
			t.Fatal("Zhiheng exchange")
		}
	})
	t.Run("Lijian is nullifiable and cannot target unknown gender", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 0, [2]string{"heg_diaochan", "heg_lvbu"})
		s.sgHegShow(0, []int{0}, false)
		cost := sgGive(t, s, 0, "jink")
		a := Action{Type: "sg_skill", Skill: "heg_lijian", Cards: []int{cost}, Targets: []int{1, 2}}
		sgGodIllegal(t, s, 0, a)
		s.sgHegShow(1, []int{0}, false)
		s.sgHegShow(2, []int{0}, false)
		sgDo(t, s, 0, a)
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "nullification" || q.Event.Kind != "duel" {
			t.Fatal("national-war Lijian bypassed counterspell", q)
		}
	})
	t.Run("Fenxun is one-directional and lasts this turn", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 0, [2]string{"heg_dingfeng", "heg_ganning"})
		s.sgHegShow(0, []int{0}, false)
		cost := sgGive(t, s, 0, "jink")
		sgWear(t, s, 2, "jueying")
		if s.sgDistance(0, 2) != 3 || s.sgDistance(2, 0) != 2 {
			t.Fatal("range fixture")
		}
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_fenxun", Cards: []int{cost}, Targets: []int{2}})
		if s.sgDistance(0, 2) != 1 || s.sgDistance(2, 0) != 2 {
			t.Fatal("fixed directional distance")
		}
		sgRestoreMountain(t, s)
		s.Sanguosha.ActivePhase = "inactive"
		if s.sgDistance(0, 2) != 3 {
			t.Fatal("Fenxun outlived the turn")
		}
	})
	t.Run("Xiongyi excludes hidden allies and is limited", func(t *testing.T) {
		s := sgHegState(6)
		sgHegSetPair(s, 0, [2]string{"heg_mateng", "heg_pangde"})
		sgHegSetPair(s, 1, [2]string{"heg_kongrong", "heg_tianfeng"})
		s.sgHegShow(0, []int{0}, false)
		s.sgHegShow(3, []int{0}, false)
		s.sgHegShow(2, []int{0}, false)
		s.Sanguosha.Players[0].HP = 2
		sgDo(t, s, 0, Action{Type: "sg_skill", Skill: "heg_xiongyi"})
		if len(s.Sanguosha.Players[0].Hand) != 3 || len(s.Sanguosha.Players[3].Hand) != 3 || len(s.Sanguosha.Players[1].Hand) != 0 || s.Sanguosha.Players[0].HP != 2 {
			t.Fatal("Xiongyi faction or smallest-faction rule")
		}
		sgGodIllegal(t, s, 0, Action{Type: "sg_skill", Skill: "heg_xiongyi"})
	})
}
