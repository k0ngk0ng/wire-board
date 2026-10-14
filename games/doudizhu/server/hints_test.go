package doudizhu

// Failure inventory before implementation: bombs precede ordinary responses;
// a lower pair breaks a held bomb despite an intact higher pair; legal split
// responses disappear; bomb size/rocket ordering is wrong; quota is bypassed;
// input hands mutate or map iteration changes the recommendation.
import (
	"reflect"
	"slices"
	"testing"
)

func TestResponseHintsPreferOrdinaryIntactCards(t *testing.T) {
	for _, ruleset := range []string{huanleRuleset, rulesetFor(4), rulesetFor(3)} {
		for _, tc := range []struct {
			hand, lead, kind string
			high             int
		}{
			{"8 8 8 8 9 9", "5 5", "pair", 9},
			{"8 8 8 8", "5 5", "pair", 8},
			{"9 9 6 6 10 10", "5 5", "pair", 6},
			{"8 8 8 8", "3 4 5 6 7", "bomb", 8},
			{"8 8 8 8 4 4 4 4", "3 3 3 3", "bomb", 4},
		} {
			t.Run(ruleset+"/"+tc.hand+"/"+tc.lead, func(t *testing.T) {
				n := 4
				if ruleset == rulesetFor(3) {
					n = 3
				}
				s := State{Ruleset: ruleset, Players: n, Options: Options{AllowStraight: true}, Landlord: 0, Hands: [][]int{cards(tc.hand)}}
				combo, e := s.classify(cards(tc.lead))
				if e != nil {
					t.Fatal(e)
				}
				s.LastPlay = &Play{Seat: 1, Cards: cards(tc.lead), Combination: combo}
				before := slices.Clone(s.Hands[0])
				got := s.legalMoves(0)
				if len(got) == 0 || got[0].Kind != tc.kind || got[0].High != tc.high {
					t.Fatalf("first hint = %+v, want %s %d", got, tc.kind, tc.high)
				}
				bombSeen := false
				for _, m := range got {
					if !beats(m.Combination, combo) {
						t.Fatal("illegal hint")
					}
					if m.Kind == "bomb" || m.Kind == "rocket" {
						bombSeen = true
					} else if bombSeen {
						t.Fatal("ordinary response after bomb")
					}
				}
				if !reflect.DeepEqual(before, s.Hands[0]) {
					t.Fatal("mutated hand")
				}
				slices.Reverse(s.Hands[0])
				if !reflect.DeepEqual(got, s.legalMoves(0)) {
					t.Fatal("hand order changed suggestions")
				}
			})
		}
	}
}
func TestResponseHintsKeepRocketLastAndRespectQuota(t *testing.T) {
	s := State{Ruleset: huanleRuleset, Players: 4, Landlord: 0, Hands: [][]int{cards("8 8 8 8 8 4 4 4 4 BJ BJ RJ RJ")}}
	c, _ := s.classify(cards("3 3 3 3"))
	s.LastPlay = &Play{Seat: 1, Combination: c}
	ms := s.legalMoves(0)
	for i := 1; i < len(ms); i++ {
		if !beats(ms[i].Combination, ms[i-1].Combination) {
			t.Fatal("bombs must ascend", ms)
		}
	}
	if ms[len(ms)-1].Kind != "rocket" {
		t.Fatal("rocket is not last")
	}
	s.Landlord = 1
	s.Bids = []int{0, 3, 0, 0}
	s.BidDone = []bool{true, true, false, false}
	s.BombCounts = []int{1, 0, 0, 0}
	if len(s.legalMoves(0)) != 0 {
		t.Fatal("exhausted bomb quota bypassed")
	}
}
