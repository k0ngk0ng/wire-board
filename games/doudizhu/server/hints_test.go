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
			{"8 8 8 8", "5 5", "bomb", 8},
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
					if len(m.Cards) == len(before) {
						continue
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
	intact := []Move{}
	held := map[int]int{}
	for _, id := range s.Hands[0] {
		held[rank(id)]++
	}
	for _, m := range ms {
		if m.Kind != "bomb" || len(m.Cards) == held[m.High] {
			intact = append(intact, m)
		}
	}
	for i := 1; i < len(intact); i++ {
		if !beats(intact[i].Combination, intact[i-1].Combination) {
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

// Failure cases before implementation: a full house breaks three eights to
// make its pair despite an intact pair of fives; physical suit IDs put a pair
// of nines before a lower intact pair of fives; legal alternatives disappear.
func TestFullHouseHintsPreserveTriplesAndUseLowestPair(t *testing.T) {
	for _, hand := range [][]int{
		{2, 15, 28, 29, 42, 6, 19, 32}, // 444, H5 S5, 888
		{2, 15, 28, 29, 42, 7, 20},     // 444, H5 S5, C9 D9
	} {
		s := State{Ruleset: huanleRuleset, Players: 4, Landlord: 0, Hands: [][]int{hand}}
		c, e := s.classify(cards("3 3 3 6 6"))
		if e != nil {
			t.Fatal(e)
		}
		s.LastPlay = &Play{Seat: 1, Combination: c}
		moves := s.legalMoves(0)
		if len(moves) == 0 {
			t.Fatal("no response")
		}
		counts := map[int]int{}
		for _, id := range moves[0].Cards {
			counts[rank(id)]++
		}
		if moves[0].High != 4 || counts[4] != 3 || counts[5] != 2 {
			t.Fatalf("want 444+55, got %+v", moves[0])
		}
		if len(moves) < 2 {
			t.Fatal("other legal responses removed")
		}
	}
}

// A held five-card bomb should not be suggested as four cards plus a singleton.
// Keep partial bombs available for deliberate manual/alternative choices.
func TestHintsKeepFiveCardBombWhole(t *testing.T) {
	s := State{Ruleset: huanleRuleset, Players: 4, Landlord: 0, Hands: [][]int{cards("9 9 9 9 9 3 6")}}
	c, e := s.classify(cards("A"))
	if e != nil {
		t.Fatal(e)
	}
	s.LastPlay = &Play{Seat: 1, Combination: c}
	moves := s.legalMoves(0)
	if len(moves) == 0 || moves[0].Kind != "bomb" || len(moves[0].Cards) != 5 {
		t.Fatalf("split held bomb: %+v", moves)
	}
	found := false
	for _, m := range moves {
		if m.Kind == "bomb" && len(m.Cards) == 4 {
			found = true
		}
	}
	if !found {
		t.Fatal("legal split option was removed")
	}
}

// Finishing the entire hand takes precedence over saving a bomb, both when
// following and when leading; a partial suggestion must not leave one card.
func TestHintsFinishWholeBombOnLeadAndReply(t *testing.T) {
	for _, reply := range []bool{false, true} {
		s := State{Ruleset: huanleRuleset, Players: 4, Landlord: 0, Hands: [][]int{cards("9 9 9 9 9")}}
		if reply {
			c, e := s.classify(cards("3"))
			if e != nil {
				t.Fatal(e)
			}
			s.LastPlay = &Play{Seat: 1, Combination: c}
		}
		moves := s.legalMoves(0)
		if len(moves) == 0 || moves[0].Kind != "bomb" || len(moves[0].Cards) != 5 {
			t.Fatalf("missed finish (reply %v): %+v", reply, moves)
		}
	}
}
