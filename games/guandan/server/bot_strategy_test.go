package guandan

// Failure cases: break a natural bomb for a cheap pair, split pairs into
// singles, spend the level wildcard unnecessarily, fail to block a public
// one-card opponent, or invent an interpretation outside legal moves.
import "testing"

func TestBotMovePreservesGroupsAndBlocksEndgame(t *testing.T) {
	for _, tc := range []struct {
		hand, lead, kind string
		high, threat     int
	}{
		{"C8 D8 S8 H8 C9 D9", "C5 D5", "pair", 9, 10},
		{"C6 D6 C9 CQ", "C5", "single", 9, 10},
		{"C6 C9 CA", "C5", "single", 14, 1},
	} {
		s := State{Phase: "play", Due: []int64{100}, Level: 2, Hands: [][]int{cards(tc.hand)}}
		cs, e := interpret(cards(tc.lead), 2)
		if e != nil {
			t.Fatal(e)
		}
		lead := cs[0]
		s.LastPlay = &Play{Seat: 1, Cards: cards(tc.lead), Combination: lead}
		ms := s.controls(0).Moves
		m := chooseBotMove(s.Hands[0], ms, true, tc.threat, 2)
		if m.Kind != tc.kind || m.High != tc.high {
			t.Fatalf("%s vs %s: %+v", tc.hand, tc.lead, m)
		}
	}
}
