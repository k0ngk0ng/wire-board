package game

import "testing"

func TestSanguoshaHegemonyHiddenRangeAndCounts(t *testing.T) {
	t.Run("Mashu only reveals when needed", func(t *testing.T) {
		s := sgHegState(6)
		sgHegSetPair(s, 0, [2]string{"heg_machao", "heg_zhangfei"})
		near := sgGive(t, s, 0, "slash")
		far := sgGive(t, s, 0, "slash")
		if s.sgDistance(0, 2) != 2 {
			t.Fatal("hidden Mashu leaked into public distance")
		}
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{near}, Targets: []int{1}})
		sgPassAll(t, s)
		if s.sgHegShown(0) {
			t.Fatal("unneeded range skill was exposed")
		}
		sgGodIllegal(t, s, 0, Action{Type: "sg_play", Cards: []int{far}, Targets: []int{3}})
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{far}, Targets: []int{2}})
		sgPassAll(t, s)
		if !s.sgHas(0, "mashu") || !s.sgHas(0, "paoxiao") || s.Sanguosha.Players[2].HP != 3 {
			t.Fatal("needed range/repeated-Slash skills were not revealed")
		}
		sgHegConservation(t, s)
	})
	t.Run("Qicai enables distant snatch", func(t *testing.T) {
		s := sgHegState(6)
		sgHegSetPair(s, 0, [2]string{"heg_huangyueying", "heg_zhaoyun"})
		id := sgGive(t, s, 0, "snatch")
		sgGive(t, s, 3, "slash")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{3}})
		if !s.sgHas(0, "qicai") || s.Sanguosha.Players[0].Hegemony.Shown[1] {
			t.Fatal("Qicai reveal exposed the wrong slot")
		}
		for n := 0; n < 32 && s.Sanguosha.Pending != nil; n++ {
			q := s.Sanguosha.Pending
			a := Action{Choice: "pass"}
			if q.Kind == "steal" {
				a.Choice = "hand"
			}
			sgDo(t, s, s.SanguoshaActor(), a)
		}
		if len(s.Sanguosha.Players[3].Hand) != 0 {
			t.Fatal("distant snatch failed")
		}
		sgHegConservation(t, s)
	})
	t.Run("Duanbing reveal follows valid extra target", func(t *testing.T) {
		s := sgHegState(4)
		sgHegSetPair(s, 0, [2]string{"heg_dingfeng", "heg_ganning"})
		id := sgGive(t, s, 0, "slash")
		sgGodIllegal(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1, 2}})
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}, Targets: []int{1, 3}})
		sgPassAll(t, s)
		if !s.sgHas(0, "heg_duanbing") || s.Sanguosha.Players[1].HP != 2 || s.Sanguosha.Players[3].HP != 3 {
			t.Fatal("hidden extra-target modifier")
		}
		sgHegConservation(t, s)
	})
}
