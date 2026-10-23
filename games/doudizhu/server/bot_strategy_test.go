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

// Before implementation: bots exhaust bombs in a mid-game escalation; spend
// a bomb to beat an ordinary high single; refuse a necessary last-card block;
// fail to use a bomb for a clear two-play finish; inspect hidden identities.
func TestBotBombRequiresPurpose(t *testing.T) {
	for _, tc := range []struct {
		hand, lead string
		enemy      int
		want       bool
	}{
		{"8 8 8 8 3 4 6 9", "7 7 7 7", 12, false},
		{"8 8 8 8 3 4 6 9", "A", 12, false},
		{"8 8 8 8 3 4 6 9", "A", 1, true},
		{"8 8 8 8 9 9", "A", 12, true},
		{"8 8 8 8", "A", 12, true},
	} {
		s := State{Ruleset: huanleRuleset, Players: 4, Landlord: 1, Hands: [][]int{cards(tc.hand), make([]int, tc.enemy), make([]int, 12), make([]int, 12)}}
		c, e := s.classify(cards(tc.lead))
		if e != nil {
			t.Fatal(e)
		}
		s.LastPlay = &Play{Seat: 1, Combination: c}
		b, e := s.classify(cards("8 8 8 8"))
		if e != nil {
			t.Fatal(e)
		}
		bomb := Move{Cards: cards("8 8 8 8"), Combination: b}
		if got := s.shouldSpendBomb(0, bomb); got != tc.want {
			t.Fatalf("%s against %s, enemy=%d: spend=%v want=%v", tc.hand, tc.lead, tc.enemy, got, tc.want)
		}
		// Same public counts, entirely different hidden identities.
		for i := range s.Hands[1] {
			s.Hands[1][i] = 100 - i
		}
		if got := s.shouldSpendBomb(0, bomb); got != tc.want {
			t.Fatal("decision used hidden identities")
		}
	}
}

// A necessary block must use the weakest sufficient intact bomb, not a larger
// six-card bomb just because its printed rank is lower.
func TestBotUsesSmallestSufficientBomb(t *testing.T) {
	s := State{Ruleset: huanleRuleset, Players: 4, Landlord: 0, Hands: [][]int{cards("4 4 4 4 4 4 A A A A 3 7")}}
	c, e := s.classify(cards("3 3 3 3"))
	if e != nil {
		t.Fatal(e)
	}
	s.LastPlay = &Play{Seat: 1, Combination: c}
	m := chooseBotMove(s.Hands[0], s.legalMoves(0), true, 1, 4)
	if m.Kind != "bomb" || len(m.Cards) != 4 || m.High != 14 {
		t.Fatalf("overspent control: %+v", m)
	}
}

// Saving strength must not make the leader break an intact bomb into singles.
func TestBotLeadsAnIntactBombWhenOnlyBombsRemain(t *testing.T) {
	s := State{Ruleset: huanleRuleset, Players: 4, Landlord: 0, Hands: [][]int{cards("4 4 4 4 4 4 A A A A")}}
	m := chooseBotMove(s.Hands[0], s.legalMoves(0), false, 12, 4)
	if m.Kind != "bomb" || len(m.Cards) != 4 || m.High != 14 {
		t.Fatalf("poor control lead: %+v", m)
	}
}
