package game

import (
	"slices"
	"testing"
)

func sgHegLuoshenTop(t *testing.T, s *State) []int {
	t.Helper()
	ids := []int{}
	for _, suit := range []int{0, 2, 1} {
		id := sgFindGive(t, s, 0, func(c SGCard) bool { return c.Suit == suit })
		s.Sanguosha.Players[0].Hand = sgRemove(s.Sanguosha.Players[0].Hand, id)
		ids = append(ids, id)
	}
	s.Sanguosha.Deck = append(clone(ids), s.Sanguosha.Deck...)
	return ids
}

func TestSanguoshaHegemonyLuoshenBatchGain(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_zhenji", "heg_caocao"})
	s.sgHegShow(0, []int{0}, false)
	ids := sgHegLuoshenTop(t, s)
	s.sgPush(SGEvent{Type: "luoshen", Actor: 0})
	s.sgRun()
	for j := 0; j < 2; j++ {
		sgDo(t, s, 0, Action{Choice: "yes"})
		if len(s.Sanguosha.Players[0].Hand) != 0 || !slices.Contains(s.Sanguosha.Table, ids[j]) || len(s.Sanguosha.Players[0].Hegemony.Luoshen) != j+1 {
			t.Fatal("black judgment gained before the sequence ended")
		}
		sgHegConservation(t, s)
		sgRestoreMountain(t, s)
	}
	sgDo(t, s, 0, Action{Choice: "yes"}) // red ends the sequence
	if !slices.Equal(s.Sanguosha.Players[0].Hand, ids[:2]) || !slices.Contains(s.Sanguosha.Discard, ids[2]) || len(s.Sanguosha.Players[0].Hegemony.Luoshen) != 0 {
		t.Fatal("batch collection or red-card cleanup")
	}
	sgHegConservation(t, s)
}

func TestSanguoshaHegemonyLuoshenTianduAndStop(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 0, [2]string{"heg_zhenji", "heg_guojia"})
	s.sgHegShow(0, []int{0, 1}, false)
	ids := sgHegLuoshenTop(t, s)
	s.sgPush(SGEvent{Type: "luoshen", Actor: 0})
	s.sgRun()
	sgDo(t, s, 0, Action{Choice: "yes"})
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_luoshen_tiandu" {
		t.Fatal("Tiandu opportunity", q)
	}
	sgDo(t, s, 0, Action{Choice: "yes"}) // take first card through Tiandu
	sgDo(t, s, 0, Action{Choice: "yes"}) // second black judgment
	sgRestoreMountain(t, s)
	sgDo(t, s, 0, Action{Choice: "pass"}) // leave second in processing
	if len(s.Sanguosha.Players[0].Hand) != 1 || !slices.Contains(s.Sanguosha.Table, ids[1]) {
		t.Fatal("declining Tiandu discarded a reserved Luoshen black card")
	}
	sgDo(t, s, 0, Action{Choice: "pass"}) // stop Luoshen and collect only remaining card
	if !slices.Equal(s.Sanguosha.Players[0].Hand, ids[:2]) || s.Sanguosha.Deck[0] != ids[2] {
		t.Fatal("duplicate Tiandu card or consumed another judgment")
	}
	sgHegConservation(t, s)
}
