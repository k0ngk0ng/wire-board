package guandan

// Failure inventory before implementation: ordinary replies trail bombs/flushes;
// hints break four eights before offering a held pair of nines; level strength,
// wildcard declarations or bomb hierarchy are lost; sorting mutates the hand.
import (
	"reflect"
	"slices"
	"testing"
)

func TestResponseHintsPreferOrdinaryIntactCards(t *testing.T) {
	for _, tc := range []struct {
		hand, lead, kind string
		level, high      int
	}{
		{"C8 D8 H8 S8 C9 D9", "C5 D5", "pair", 2, 9},
		{"C8 D8 H8 S8", "C5 D5", "bomb", 2, 8},
		{"C9 D9 C6 D6 C10 D10", "C5 D5", "pair", 2, 6},
		{"C8 D8 C2 D2", "CA DA", "pair", 8, 15},
		{"C8 D8 H8 S8", "C3 D4 H5 S6 C7", "bomb", 2, 8},
		{"C8 H2 C9 D9", "C5 D5", "pair", 2, 8},
	} {
		t.Run(tc.hand+"/"+tc.lead, func(t *testing.T) {
			cs, e := interpret(cards(tc.lead), tc.level)
			if e != nil {
				t.Fatal(e)
			}
			s := State{Phase: "play", Turn: 0, Level: tc.level, Hands: [][]int{cards(tc.hand)}, Due: []int64{100}, LastPlay: &Play{Seat: 1, Cards: cards(tc.lead), Combination: cs[0]}}
			before := slices.Clone(s.Hands[0])
			ms := s.controls(0).Moves
			if len(ms) == 0 || ms[0].Kind != tc.kind || ms[0].High != tc.high {
				t.Fatalf("first hint = %+v, want %s %d", ms, tc.kind, tc.high)
			}
			bombSeen := false
			for _, m := range ms {
				if !beats(m.Combination, cs[0]) {
					t.Fatal("illegal hint")
				}
				if len(m.Cards) == len(before) {
					continue
				}
				if category(m.Combination) > 0 {
					bombSeen = true
				} else if bombSeen {
					t.Fatal("ordinary response after bomb")
				}
			}
			if !reflect.DeepEqual(before, s.Hands[0]) {
				t.Fatal("mutated hand")
			}
			slices.Reverse(s.Hands[0])
			if !reflect.DeepEqual(ms, s.controls(0).Moves) {
				t.Fatal("hand order changed suggestions")
			}
		})
	}
}
func TestResponseHintsUseGuandanBombHierarchy(t *testing.T) {
	cs, _ := interpret(cards("C3 D3 H3 S3"), 2)
	s := State{Phase: "play", Turn: 0, Level: 2, Hands: [][]int{cards("C8 D8 H8 S8 C8 C9 D9 H9 S9 C9 D9 D4 D5 D6 D7 D8 BJ BJ RJ RJ")}, Due: []int64{100}, LastPlay: &Play{Seat: 1, Combination: cs[0]}}
	ms := s.controls(0).Moves
	if len(ms) == 0 {
		t.Fatal("missing bomb hints")
	}
	intact := []Move{}
	held := map[int]int{}
	for _, id := range s.Hands[0] {
		held[natural(id)]++
	}
	for _, m := range ms {
		if m.Kind != "bomb" || len(m.Cards) == held[m.High] {
			intact = append(intact, m)
		}
	}
	for i := 1; i < len(intact); i++ {
		a, b := intact[i-1], intact[i]
		if category(a.Combination) > category(b.Combination) || (category(a.Combination) == category(b.Combination) && a.High > b.High) {
			t.Fatal("bomb hierarchy changed", ms)
		}
	}
	if ms[len(ms)-1].Kind != "rocket" {
		t.Fatal("rocket not last")
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
		s := State{Phase: "play", Due: []int64{100}, Level: 2, Hands: [][]int{hand}}
		cs, e := interpret(cards("C3 D3 H3 C6 D6"), 2)
		if e != nil {
			t.Fatal(e)
		}
		s.LastPlay = &Play{Seat: 1, Combination: cs[0]}
		moves := s.controls(0).Moves
		if len(moves) == 0 {
			t.Fatal("no response")
		}
		counts := map[int]int{}
		for _, id := range moves[0].Cards {
			counts[natural(id)]++
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
	s := State{Phase: "play", Due: []int64{100}, Level: 2, Hands: [][]int{cards("C9 D9 H9 S9 C9 C3 D6")}}
	cs, e := interpret(cards("CA"), 2)
	if e != nil {
		t.Fatal(e)
	}
	s.LastPlay = &Play{Seat: 1, Combination: cs[0]}
	moves := s.controls(0).Moves
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
		s := State{Phase: "play", Due: []int64{100}, Level: 2, Hands: [][]int{cards("C9 D9 H9 S9 C9")}}
		if reply {
			cs, e := interpret(cards("C3"), 2)
			if e != nil {
				t.Fatal(e)
			}
			s.LastPlay = &Play{Seat: 1, Combination: cs[0]}
		}
		moves := s.controls(0).Moves
		if len(moves) == 0 || moves[0].Kind != "bomb" || len(moves[0].Cards) != 5 {
			t.Fatalf("missed finish (reply %v): %+v", reply, moves)
		}
	}
}
