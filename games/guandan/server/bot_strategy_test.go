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

// Before implementation: bots exhaust bombs in a mid-game escalation; spend
// a bomb to beat an ordinary high single; refuse a necessary last-card block;
// fail to use a bomb for a clear two-play finish; inspect hidden identities.
func TestBotBombRequiresPurpose(t *testing.T) {
	for _, tc := range []struct {
		hand, lead string
		enemy      int
		want       bool
	}{
		{"C8 D8 H8 S8 C3 D4 H6 S9", "C7 D7 H7 S7", 12, false},
		{"C8 D8 H8 S8 C3 D4 H6 S9", "CA", 12, false},
		{"C8 D8 H8 S8 C3 D4 H6 S9", "CA", 1, true},
		{"C8 D8 H8 S8 C9 D9", "CA", 12, true},
		{"C8 D8 H8 S8", "CA", 12, true},
	} {
		s := State{Level: 2, Hands: [][]int{cards(tc.hand), make([]int, tc.enemy), make([]int, 12), make([]int, 12)}}
		cs, e := interpret(cards(tc.lead), 2)
		if e != nil {
			t.Fatal(e)
		}
		s.LastPlay = &Play{Seat: 1, Combination: cs[0]}
		bs, e := interpret(cards("C8 D8 H8 S8"), 2)
		if e != nil {
			t.Fatal(e)
		}
		bomb := Move{Cards: cards("C8 D8 H8 S8"), Combination: bs[0]}
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
	s := State{Phase: "play", Due: []int64{100}, Level: 2, Hands: [][]int{cards("C4 D4 H4 S4 C4 D4 CA DA HA SA C3 D7")}}
	cs, e := interpret(cards("C3 D3 H3 S3"), 2)
	if e != nil {
		t.Fatal(e)
	}
	s.LastPlay = &Play{Seat: 1, Combination: cs[0]}
	m := chooseBotMove(s.Hands[0], s.controls(0).Moves, true, 1, 2)
	if m.Kind != "bomb" || len(m.Cards) != 4 || m.High != 14 {
		t.Fatalf("overspent control: %+v", m)
	}
}

// Saving strength must not make the leader break an intact bomb into singles.
func TestBotLeadsAnIntactBombWhenOnlyBombsRemain(t *testing.T) {
	s := State{Phase: "play", Due: []int64{100}, Level: 2, Hands: [][]int{cards("C4 D4 H4 S4 C4 D4 CA DA HA SA")}}
	m := chooseBotMove(s.Hands[0], s.controls(0).Moves, false, 12, 2)
	if m.Kind != "bomb" || len(m.Cards) != 4 || m.High != 14 {
		t.Fatalf("poor control lead: %+v", m)
	}
}
