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
		{"C8 D8 H8 S8", "C5 D5", "pair", 2, 8},
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
	for i := 1; i < len(ms); i++ {
		a, b := ms[i-1], ms[i]
		if category(a.Combination) > category(b.Combination) || (category(a.Combination) == category(b.Combination) && a.High > b.High) {
			t.Fatal("bomb hierarchy changed", ms)
		}
	}
	if ms[len(ms)-1].Kind != "rocket" {
		t.Fatal("rocket not last")
	}
}
