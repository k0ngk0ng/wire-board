package doudizhu

// Failure scenarios are recorded before implementation in docs/poker-implementation.md.
// These cases isolate the combinatorial boundary; complete matches use module/HTTP E2E.
import (
	"strings"
	"testing"
)

func cards(ranks string) []int {
	used := map[string]int{}
	out := []int{}
	for _, r := range strings.Fields(ranks) {
		n := used[r]
		used[r]++
		rank := map[string]int{"2": 0, "3": 1, "4": 2, "5": 3, "6": 4, "7": 5, "8": 6, "9": 7, "10": 8, "J": 9, "Q": 10, "K": 11, "A": 12}[r]
		id := (n%4)*13 + rank + (n/4)*54
		if r == "BJ" {
			id = 52 + 54*n
		}
		if r == "RJ" {
			id = 53 + 54*n
		}
		out = append(out, id)
	}
	return out
}
func TestCombinations(t *testing.T) {
	cases := []struct {
		ranks, kind string
		n           int
		straight    bool
	}{
		{"3", "single", 3, true}, {"2 2", "pair", 3, true}, {"A A A", "triple", 3, true},
		{"3 3 3 4", "trio_single", 3, true}, {"3 3 3 4 4", "trio_pair", 3, true},
		{"3 4 5 6 7", "straight", 3, true}, {"10 J Q K A", "straight", 3, true},
		{"A 2 3 4 5", "", 3, true}, {"J Q K A 2", "", 3, true}, {"3 4 5 6", "", 3, true},
		{"3 3 4 4 5 5", "pairs", 3, true}, {"3 3 4 4", "", 3, true},
		{"3 3 3 4 4 4", "triples", 3, true}, {"3 3 3 4 4 4 5 5", "plane_single", 3, true},
		{"3 3 3 4 4 4 6 6 7 7", "plane_pair", 3, true},
		{"3 3 3 4 4 4 3 4", "", 3, true}, {"3 3 3 4 4 4 BJ RJ", "", 3, true},
		{"3 3 3 4 4 4 5 5 5 9 9 9", "plane_single", 3, true},
		{"3 3 3 4 4 4 5 5 5 6 6 6", "triples", 3, true},
		{"3 3 3 3 4 4", "four_single", 3, true},
		{"3 3 3 3 4 4 5 5", "four_pair", 3, true},
		{"3 3 3 3 4 4 4 4", "", 3, true}, {"3 3 3 3 BJ RJ", "", 3, true},
		{"3 3 3 3", "bomb", 3, true}, {"BJ RJ", "rocket", 3, true},
		{"3 3 3 3 3", "", 3, true}, {"BJ BJ", "pair", 4, true}, {"BJ RJ", "", 4, true},
		{"RJ RJ", "pair", 4, true}, {"BJ BJ RJ RJ", "rocket", 4, true},
		{"3 3 3 3 3 3 3 3", "bomb", 4, true},
		{"3 3 3 4", "", 4, true}, {"3 3 3 4 4", "trio_pair", 4, true},
		{"3 3 3 4 4 4 5 5", "", 4, true}, {"3 3 3 4 4 4 6 6 7 7", "plane_pair", 4, true},
		{"3 3 3 3 4 4", "", 4, true}, {"3 3 3 3 4 4 5 5", "", 4, true},
		{"3 4 5 6 7", "straight", 4, true}, {"3 4 5 6 7", "", 4, false},
		{"2 2 2", "triple", 4, true}, {"BJ BJ BJ", "", 4, true},
	}
	for _, c := range cases {
		t.Run(c.ranks+"/"+c.kind+"/"+string(rune('0'+c.n)), func(t *testing.T) {
			v, err := classify(cards(c.ranks), c.n, c.straight)
			if c.kind == "" {
				if err == nil {
					t.Fatalf("accepted invalid combination: %+v", v)
				}
				return
			}
			if err != nil || v.Kind != c.kind {
				t.Fatalf("got %+v %v, want %s", v, err, c.kind)
			}
		})
	}
	for _, cs := range [][]int{{}, {0, 0}, {-1}, {54}, {108}} {
		if _, err := classify(cs, 3, true); err == nil {
			t.Fatalf("accepted invalid card IDs: %v", cs)
		}
	}
}
func TestFourPlayerRocketIsHighest(t *testing.T) {
	rocket, _ := classify(cards("BJ BJ RJ RJ"), 4, true)
	eight, _ := classify(cards("2 2 2 2 2 2 2 2"), 4, true)
	seven, _ := classify(cards("A A A A A A A"), 4, true)
	six, _ := classify(cards("2 2 2 2 2 2"), 4, true)
	if !beats(rocket, eight) || beats(eight, rocket) || !beats(eight, seven) || !beats(seven, six) {
		t.Fatal("four kings must beat every bomb; bomb length precedes rank")
	}
	a, _ := classify(cards("3 4 5 6 7"), 3, true)
	b, _ := classify(cards("4 5 6 7 8 9"), 3, true)
	if beats(b, a) {
		t.Fatal("different-length straights cannot beat one another")
	}
	if beats(rocket, rocket) {
		t.Fatal("equal combinations do not beat themselves")
	}
}
func TestGeneratedMovesAreLegalAndDoNotLeakInputMutation(t *testing.T) {
	for _, n := range []int{3, 4} {
		h := cards("3 3 3 3 4 4 4 5 5 5 6 6 7 7 8 9 10 J Q K A 2 BJ RJ")
		if n == 4 {
			h = append(h, cards("BJ RJ")[0]+54, cards("BJ RJ")[1]+54)
		}
		original := append([]int{}, h...)
		moves := movesFor(h, n, true)
		if len(moves) < 15 {
			t.Fatalf("generator omitted basic combinations: %d", len(moves))
		}
		for _, m := range moves {
			if _, e := classify(m.Cards, n, true); e != nil {
				t.Fatal(m, e)
			}
			used := map[int]bool{}
			for _, id := range m.Cards {
				found := false
				for _, c := range h {
					if id == c {
						found = true
					}
				}
				if !found || used[id] {
					t.Fatal("invalid generated card", id)
				}
				used[id] = true
			}
		}
		for i, c := range h {
			if c != original[i] {
				t.Fatal("move generation mutated private hand")
			}
		}
	}
}
