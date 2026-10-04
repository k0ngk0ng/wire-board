package game

import (
	"slices"
	"testing"
)

func TestSanguoshaHegemonyGuzhengOptionalObtain(t *testing.T) {
	for _, choice := range []string{"yes", "pass", "timeout", "bot"} {
		t.Run(choice, func(t *testing.T) {
			s := sgHegState(4)
			sgHegSetPair(s, 1, [2]string{"heg_erzhang", "heg_sunquan"})
			s.sgHegShow(1, []int{0}, false)
			returned := sgGive(t, s, 0, "jink")
			remaining := sgGive(t, s, 0, "slash")
			other := sgGive(t, s, 2, "peach")
			s.Sanguosha.DiscardPhase = true
			s.sgDiscard(0, []int{returned, remaining})
			s.sgDiscard(2, []int{other})
			s.sgPush(SGEvent{Type: "guzheng", Actor: 0})
			s.sgRun()
			sgGodIllegal(t, s, 1, Action{Card: other})
			sgDo(t, s, 1, Action{Card: returned})
			q := s.Sanguosha.Pending
			if q == nil || q.Kind != "heg_guzheng_obtain" || !sgSameIDs(q.Cards, []int{remaining, other}) || !s.sgOwn(0, returned, true) || len(s.Sanguosha.Players[1].Hand) != 0 {
				t.Fatal("return must finish before optional obtain", q)
			}
			sgRestoreMountain(t, s)
			sgGodIllegal(t, s, 1, Action{Choice: "invalid"})
			switch choice {
			case "timeout":
				if err := s.SanguoshaTimeout(); err != nil {
					t.Fatal(err)
				}
			case "bot":
				a, err := s.sgBot(1)
				if err != nil || a.Choice != "yes" {
					t.Fatal("bot should accept remaining cards", a, err)
				}
				sgDo(t, s, 1, a)
			default:
				sgDo(t, s, 1, Action{Choice: choice})
			}
			accepted := choice == "yes" || choice == "bot"
			for _, id := range []int{remaining, other} {
				if s.sgOwn(1, id, true) != accepted || slices.Contains(s.Sanguosha.Discard, id) == accepted {
					t.Fatal("optional acquisition ignored", id, choice)
				}
			}
			if s.Sanguosha.Pending != nil || len(s.Sanguosha.DiscardedHand)+len(s.Sanguosha.DiscardedOther) != 0 {
				t.Fatal("discard-phase continuation not resumed")
			}
			sgHegConservation(t, s)
		})
	}
}

func TestSanguoshaHegemonyGuzhengRevealRewardReshuffle(t *testing.T) {
	s := sgHegState(4)
	sgHegSetPair(s, 1, [2]string{"heg_erzhang", "heg_sunquan"})
	returned := sgGive(t, s, 0, "jink")
	remaining := sgGive(t, s, 0, "slash")
	s.Sanguosha.DiscardPhase = true
	s.sgDiscard(0, []int{returned, remaining})
	// The reveal reward will reshuffle and draw the only remaining discard.
	s.Sanguosha.Players[3].Hand = append(s.Sanguosha.Players[3].Hand, s.Sanguosha.Deck...)
	s.Sanguosha.Deck = nil
	s.sgPush(SGEvent{Type: "guzheng", Actor: 0})
	s.sgRun()
	sgDo(t, s, 1, Action{Card: returned})
	if q := s.Sanguosha.Pending; q == nil || q.Kind != "heg_reward" || !s.sgOwn(0, returned, true) {
		t.Fatal("first reveal must follow return, before acquire", q)
	}
	sgRestoreMountain(t, s)
	sgDo(t, s, 1, Action{Choice: "draw"})
	if !s.sgOwn(1, remaining, true) || s.Sanguosha.Pending != nil || len(s.Sanguosha.Discard) != 0 {
		t.Fatal("must not acquire a stale reshuffled card or open empty choice")
	}
	sgHegConservation(t, s)
}
