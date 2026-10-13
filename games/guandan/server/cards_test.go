package guandan

// Failure cases first: wilds replace jokers; levels break natural sequences;
// a chosen wildcard interpretation is silently changed; five-card straight
// length, three-pair/two-triple limits and bomb hierarchy borrow DDZ rules.
import (
	"strconv"
	"strings"
	"testing"
)

func cards(spec string) []int {
	used := map[int]int{}
	rankUsed := map[string]int{}
	out := []int{}
	for _, token := range strings.Fields(spec) {
		base := 0
		if token == "BJ" {
			base = 52
		} else if token == "RJ" {
			base = 53
		} else {
			suit := -1
			r := token
			if len(token) > 1 && strings.ContainsRune("CDHS", rune(token[0])) {
				suit = strings.IndexByte("CDHS", token[0])
				r = token[1:]
			}
			n, e := strconv.Atoi(r)
			if e != nil {
				n = map[string]int{"J": 11, "Q": 12, "K": 13, "A": 14}[r]
			}
			if suit < 0 {
				suit = rankUsed[r] % 4
				rankUsed[r]++
			}
			base = suit*13 + n - 2
		}
		id := base + 54*used[base]
		used[base]++
		out = append(out, id)
	}
	return out
}
func containsKind(a []Combination, kind string, high int) bool {
	for _, v := range a {
		if v.Kind == kind && (high == 0 || v.High == high) {
			return true
		}
	}
	return false
}
func TestJiangsuCombinations(t *testing.T) {
	cases := []struct {
		spec, kind  string
		level, high int
	}{
		{"7", "single", 7, 15}, {"H7", "single", 7, 15}, {"A", "single", 7, 14}, {"2", "single", 7, 2},
		{"BJ BJ", "pair", 7, 16}, {"BJ RJ", "", 7, 0}, {"BJ H7", "", 7, 0}, {"BJ BJ H7", "", 7, 0}, {"H7 H7", "pair", 7, 15},
		{"3 3 3", "triple", 7, 3}, {"3 3 3 5 5", "full_house", 7, 3}, {"3 3 3 4 5", "", 7, 0}, {"3 3 3 4", "", 7, 0},
		{"C3 D4 H5 S6 C7", "straight", 7, 7}, {"CA D2 H3 S4 C5", "straight", 7, 5}, {"C10 DJ HQ SK CA", "straight", 7, 14}, {"J Q K A 2", "", 7, 0}, {"3 4 5 6 7 8", "", 9, 0},
		{"3 3 4 4 5 5", "tube", 7, 5}, {"A A 2 2 3 3", "tube", 7, 3}, {"K K A A 2 2", "", 7, 0}, {"3 3 4 4", "", 7, 0},
		{"3 3 3 4 4 4", "plate", 7, 4}, {"A A A 2 2 2", "plate", 7, 2}, {"3 3 3 4 4 4 5 5 5", "", 7, 0},
		{"C3 C4 C5 C6 C7", "straight_flush", 9, 7}, {"C3 C4 C5 C6 H7", "straight_flush", 7, 7},
		{"9 9 H7 H7", "bomb", 7, 9}, {"9 9 9 9 9 9 9 9 H7 H7", "bomb", 7, 9},
		{"BJ BJ RJ RJ", "rocket", 7, 17}, {"BJ BJ RJ H7", "", 7, 0},
	}
	for _, c := range cases {
		t.Run(c.spec+"/"+c.kind, func(t *testing.T) {
			v, e := interpret(cards(c.spec), c.level)
			if c.kind == "" {
				if e == nil && len(v) > 0 {
					t.Fatalf("accepted: %+v", v)
				}
				return
			}
			if e != nil || !containsKind(v, c.kind, c.high) {
				t.Fatalf("got %+v %v, expected %s/%d", v, e, c.kind, c.high)
			}
		})
	}
	for _, ids := range [][]int{{}, {1, 1}, {-1}, {108}} {
		if v, e := interpret(ids, 7); e == nil && len(v) > 0 {
			t.Fatal("accepted invalid IDs", ids)
		}
	}
}
func TestWildInterpretationsRemainExplicit(t *testing.T) {
	v, e := interpret(cards("H4 H4 8 8 9 9"), 4)
	if e != nil {
		t.Fatal(e)
	}
	if !containsKind(v, "tube", 9) || !containsKind(v, "tube", 10) || !containsKind(v, "plate", 9) {
		t.Fatalf("lost legal interpretations: %+v", v)
	}
	seen := map[string]bool{}
	for _, c := range v {
		if c.Key == "" || seen[c.Key] {
			t.Fatal("ambiguous/nonunique declaration key")
		}
		seen[c.Key] = true
		for _, w := range c.Wild {
			if w.Rank < 2 || w.Rank > 14 || w.Suit < 0 || w.Suit > 3 {
				t.Fatal("wild replaced joker or invalid suit")
			}
		}
	}
}
func TestJiangsuBombOrder(t *testing.T) {
	pick := func(spec, kind string) Combination {
		v, e := interpret(cards(spec), 7)
		if e != nil {
			t.Fatal(e)
		}
		for _, c := range v {
			if c.Kind == kind {
				return c
			}
		}
		t.Fatal(spec, kind)
		return Combination{}
	}
	four := pick("7 7 7 7", "bomb")
	five := pick("3 3 3 3 3", "bomb")
	flush := pick("C3 C4 C5 C6 C7", "straight_flush")
	six := pick("2 2 2 2 2 2", "bomb")
	ten := pick("9 9 9 9 9 9 9 9 H7 H7", "bomb")
	rocket := pick("BJ BJ RJ RJ", "rocket")
	for _, pair := range [][2]Combination{{five, four}, {flush, five}, {six, flush}, {ten, six}, {rocket, ten}} {
		if !beats(pair[0], pair[1]) || beats(pair[1], pair[0]) {
			t.Fatal("wrong bomb order", pair)
		}
	}
	if beats(rocket, rocket) {
		t.Fatal("equal rocket beats itself")
	}
}
func TestGeneratedMovesHaveValidDeclarations(t *testing.T) {
	h := cards("3 3 3 4 4 5 5 6 6 7 7 8 8 9 9 10 J Q K A 2 BJ BJ RJ RJ H7 H7")
	// Avoid manufacturing a third physical heart-seven: generic 7s use C and D.
	ms := movesFor(h, 7)
	if len(ms) < 20 {
		t.Fatalf("insufficient candidate coverage %d", len(ms))
	}
	for _, m := range ms {
		v, e := interpret(m.Cards, 7)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, c := range v {
			if c.Key == m.Key {
				found = true
			}
		}
		if !found {
			t.Fatal("generator returned a different declaration", m)
		}
	}
}
