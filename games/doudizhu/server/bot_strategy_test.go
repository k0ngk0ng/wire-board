package doudizhu

// Failure cases: split a held bomb/pair for a cheap response; waste a royal
// bomb; play a low single ahead of an opponent with one card; ignore a complete
// finishing combination; inspect hidden card identities to estimate danger.
import "testing"

func TestBotMovePreservesGroupsAndBlocksEndgame(t *testing.T) {
	for _, tc := range []struct {
		hand, lead, kind string
		high, threat     int
	}{
		{"8 8 8 8 9 9", "5 5", "pair", 9, 10},
		{"6 6 9 Q", "5", "single", 9, 10},
		{"6 9 A", "5", "single", 14, 1},
	} {
		s := State{Ruleset: huanleRuleset, Players: 4, Options: Options{AllowStraight: true}, Landlord: 0, Hands: [][]int{cards(tc.hand)}}
		c, e := s.classify(cards(tc.lead))
		if e != nil {
			t.Fatal(e)
		}
		s.LastPlay = &Play{Seat: 1, Combination: c}
		ms := s.legalMoves(0)
		m := chooseBotMove(s.Hands[0], ms, true, tc.threat, 4)
		if m.Kind != tc.kind || m.High != tc.high {
			t.Fatalf("%s vs %s: %+v", tc.hand, tc.lead, m)
		}
	}
}
