package game

import (
	"slices"
	"testing"
)

func TestSanguoshaHegemonyHuoshouClaimTiming(t *testing.T) {
	for _, claim := range []bool{false, true} {
		s := sgHegState(4)
		sgHegSetPair(s, 1, [2]string{"heg_menghuo", "heg_liubei"})
		id := sgGive(t, s, 0, "savage_assault")
		sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}})
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_invoke" || q.Event.Kind != "huoshou" {
			t.Fatal("Huoshou must be offered before target counter windows", q)
		}
		choice := "pass"
		if claim {
			choice = "yes"
		}
		sgDo(t, s, 1, Action{Choice: choice})
		if claim {
			sgDo(t, s, 1, Action{Choice: "pass"}) // first reveal reward
		}
		sgHegPassCounters(t, s)
		if !claim {
			// A separate immunity trigger is legal even if source replacement
			// was declined. It must not retroactively replace the damage source.
			sgDo(t, s, 1, Action{Choice: "yes"})
			sgDo(t, s, 1, Action{Choice: "pass"})
			sgHegPassCounters(t, s)
		}
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "card" || q.Player != 2 || (q.Event.SavageSource == 2) != claim {
			t.Fatal("source declaration changed at the wrong timing", q)
		}
		sgRestoreMountain(t, s)
		sgPassAll(t, s)
		if s.Sanguosha.Players[1].HP != 4 || s.Sanguosha.Players[2].HP != 3 || s.Sanguosha.Players[3].HP != 3 {
			t.Fatal("Savage Assault immunity and damage")
		}
		sgHegConservation(t, s)
	}
}

func TestSanguoshaHegemonyJuxiangCanRevealAtCleanup(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 1, [2]string{"heg_zhurong", "heg_liubei"})
	id := sgGive(t, s, 0, "savage_assault")
	sgDo(t, s, 0, Action{Type: "sg_play", Cards: []int{id}})
	sgHegPassCounters(t, s)
	sgDo(t, s, 1, Action{Choice: "pass"}) // decline the immunity, stay hidden
	sgDo(t, s, 1, Action{Choice: "pass"}) // decline the Slash response
	for n := 0; n < 32; n++ {
		q := s.Sanguosha.Pending
		if q == nil || q.Kind == "heg_invoke" {
			break
		}
		sgDo(t, s, s.SanguoshaActor(), Action{Choice: "pass"})
	}
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_invoke" || q.Event.Kind != "juxiang" || !slices.Contains(s.Sanguosha.Discard, id) {
		t.Fatal("cleanup should allow a new Juxiang reveal decision", q)
	}
	// The reveal reward must not lose/duplicate the known physical card if
	// drawing forces the discard pile, including that card, to reshuffle.
	s.Sanguosha.Discard = append(s.Sanguosha.Discard, s.Sanguosha.Deck...)
	s.Sanguosha.Deck = nil
	sgRestoreMountain(t, s)
	sgDo(t, s, 1, Action{Choice: "yes"})
	sgDo(t, s, 1, Action{Choice: "draw"})
	if !s.sgOwn(1, id, true) || s.Sanguosha.Players[1].HP != 3 {
		t.Fatal("Juxiang card missing after reveal/reward reshuffle")
	}
	sgHegConservation(t, s)
}
