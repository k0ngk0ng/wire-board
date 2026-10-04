package game

import "testing"

func TestSanguoshaHegemonyYingziHaoshiStack(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_zhouyu", "heg_lusu"})
	s.Sanguosha.Hegemony.FirstClaimed = true // Isolate the normal draw count.
	s.sgDraw(0, 1)
	s.sgPush(SGEvent{Type: "draw_phase", Actor: 0})
	s.sgRun()
	sgGodIllegal(t, s, 0, Action{Choice: "yingzi+yingzi"})
	sgDo(t, s, 0, Action{Choice: "yingzi+haoshi"})
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "haoshi_give" || len(s.Sanguosha.Players[0].Hand) != 6 {
		t.Fatal("dual-general draw bonuses must stack before Haoshi distribution", q)
	}
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Cards: s.Sanguosha.Players[0].Hand[:3], Targets: []int{1}})
	if len(s.Sanguosha.Players[0].Hand) != 3 || len(s.Sanguosha.Players[1].Hand) != 3 || s.Sanguosha.Players[0].Hegemony.Shown != [2]bool{true, true} {
		t.Fatal("Haoshi mandatory half-hand distribution")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyDrawReplacementExcludesLuoyi(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_zhangliao", "heg_xuchu"})
	s.Sanguosha.Hegemony.FirstClaimed = true
	sgGive(t, s, 1, "slash")
	sgGive(t, s, 2, "jink")
	s.sgPush(SGEvent{Type: "draw_phase", Actor: 0})
	s.sgRun()
	sgGodIllegal(t, s, 0, Action{Choice: "tuxi+luoyi", Targets: []int{1, 2}})
	sgDo(t, s, 0, Action{Choice: "tuxi", Targets: []int{1, 2}})
	if len(s.Sanguosha.Players[0].Hand) != 2 || s.Sanguosha.Players[0].Used["luoyi"] != 0 || s.Sanguosha.Players[0].Hegemony.Shown[1] {
		t.Fatal("Tuxi replaces normal draw and cannot grant a Luoyi bonus")
	}
	sgHegConservation(t, s)
}
