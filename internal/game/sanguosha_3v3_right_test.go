package game

import (
	"slices"
	"strings"
	"testing"
)

// Isolated failure scenarios for the 3v3 action right, listed before the check:
//   - the leader's chosen vanguard is ignored and the camp's other vanguard goes
//     first instead;
//   - the second vanguard never gets its turn, so the camp wastes an action unit;
//   - the right does not move to the other camp once both vanguards acted;
//   - a flipped (翻面) vanguard burns the camp's whole unit instead of handing
//     the turn to its partner while still counting as acted for the round.
//
// The natural HTTP run stays in internal/server/sanguosha_3v3_test.go; this
// fixture is deterministic and does not depend on the draft or on bot choices.
func TestSanguoshaThreeV3VanguardOrder(t *testing.T) {
	t.Run("the chosen vanguard opens and its partner follows", func(t *testing.T) {
		s := sgThreeBattleState(t)
		s.sgThreeBeginRight(sgThreeCold, false)
		if q := s.Sanguosha.Pending; q == nil || q.Kind != "sg_3v3_side" || q.Player != 1 {
			t.Fatalf("action right prompt %+v", q)
		}
		sgDo(t, s, 1, Action{Choice: "vanguards", Target: 2})
		if s.Turn != 2 || s.Sanguosha.TurnSequence != 1 {
			t.Fatalf("chosen vanguard did not open, turn=%d seq=%d", s.Turn, s.Sanguosha.TurnSequence)
		}
		sgDo(t, s, 2, Action{Type: "sg_end"})
		if s.Turn != 0 || s.Sanguosha.ThreeV3.Acted[2] != true {
			t.Fatalf("partner vanguard did not follow, turn=%d acted=%v", s.Turn, s.Sanguosha.ThreeV3.Acted)
		}
		sgDo(t, s, 0, Action{Type: "sg_end"})
		q := s.Sanguosha.Pending
		if q == nil || q.Kind != "sg_3v3_side" || q.Player != 4 || s.Sanguosha.ThreeV3.Side != sgThreeWarm {
			t.Fatalf("right did not move to the other camp: %+v side=%d", q, s.Sanguosha.ThreeV3.Side)
		}
		if !slices.Contains(q.Choices, "leader") || !slices.Contains(q.Choices, "vanguards") {
			t.Fatalf("warm leader choices %v", q.Choices)
		}
		if s.Sanguosha.ThreeV3.Acted[1] {
			t.Fatal("the cold leader was marked acted without playing its own unit")
		}
	})
	t.Run("a flipped vanguard is skipped and still counts as acted", func(t *testing.T) {
		s := sgThreeBattleState(t)
		s.Sanguosha.Players[2].Flipped = true
		s.sgThreeBeginRight(sgThreeCold, false)
		sgDo(t, s, 1, Action{Choice: "vanguards", Target: 2})
		if s.Turn != 0 {
			t.Fatalf("flipped vanguard still opened, turn=%d", s.Turn)
		}
		if s.Sanguosha.Players[2].Flipped {
			t.Fatal("the skipped vanguard stayed face down")
		}
		if !s.Sanguosha.ThreeV3.Acted[2] {
			t.Fatalf("skipped vanguard did not count for the round: %v", s.Sanguosha.ThreeV3.Acted)
		}
		if !slices.ContainsFunc(s.Log, func(line string) bool { return strings.Contains(line, "跳过本回合") }) {
			t.Fatalf("skip was not logged: %v", s.Log)
		}
	})
}
