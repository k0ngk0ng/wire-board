package shengji

// Failure inventory (before implementation): false identical pairs, trump
// level gaps and equal-strength off-suit levels, missed mandatory tractors or
// partial tractors, off-suit fillers hiding held cards, false mixed ruffs,
// cancellable failed throws, Shanghai single-deck four-suit leads, incorrect
// single/double score boundaries and bottom multipliers.
import (
	"strconv"
	"strings"
	"testing"
)

func cards(spec string) []int {
	used := map[int]int{}
	out := []int{}
	for _, s := range strings.Fields(spec) {
		b := 52
		if s == "RJ" {
			b = 53
		} else if s != "BJ" {
			suit := strings.IndexByte("CDHS", s[0])
			n, e := strconv.Atoi(s[1:])
			if e != nil {
				n = map[string]int{"J": 11, "Q": 12, "K": 13, "A": 14}[s[1:]]
			}
			if suit < 0 || n < 2 {
				panic(s)
			}
			b = suit*13 + n - 2
		}
		out = append(out, b+used[b]*54)
		used[b]++
	}
	return out
}
func TestTrumpAndIdenticalPairs(t *testing.T) {
	r := Rules{Decks: 2, Level: 4, Trump: 1}
	for _, spec := range []string{"C3 C3 C5 C5", "DA DA C4 C4", "C4 C4 D4 D4", "D4 D4 BJ BJ", "BJ BJ RJ RJ"} {
		s, e := leadShape(cards(spec), r)
		if e != nil || len(s.Parts) != 1 || s.Parts[0].Kind != "tractor" {
			t.Fatalf("%s: %+v %v", spec, s, e)
		}
	}
	for _, spec := range []string{"C4 C4 H4 H4", "C3 C3 C6 C6"} {
		s, e := leadShape(cards(spec), r)
		if e != nil || len(s.Parts) == 1 {
			t.Fatalf("false tractor %s: %+v %v", spec, s, e)
		}
	}
	s, _ := leadShape(cards("C4 H4"), r)
	if len(s.Parts) != 2 {
		t.Fatal("equal strengths are not identical pairs")
	}
	if _, e := leadShape(cards("C3 D3"), r); e == nil {
		t.Fatal("mixed suit ordinary lead accepted")
	}
	n := Rules{Decks: 2, Level: 4, Trump: -1}
	if suitOf(cards("H4")[0], n) != 4 || power(cards("C4")[0], n) != power(cards("H4")[0], n) {
		t.Fatal("no-trump level cards")
	}
}
func TestObligatoryStructure(t *testing.T) {
	r := Rules{Decks: 2, Level: 2, Trump: 3}
	cases := []struct {
		lead, hand, chosen string
		valid              bool
	}{
		{"C5 C5", "C6 C6 CA CK", "CA CK", false},
		{"C5 C5", "C6 C6 CA CK", "C6 C6", true},
		{"C5 C5 C6 C6", "C7 C7 C8 C8 CJ CJ", "C7 C7 CJ CJ", false},
		{"C5 C5 C6 C6", "C7 C7 C8 C8 CJ CJ", "C7 C7 C8 C8", true},
		{"C5 C5 C6 C6 C7 C7", "C8 C8 C9 C9 CJ CJ CA CK", "C8 C8 CJ CJ CA CK", false},
		{"C5 C5 C6 C6 C7 C7", "C8 C8 C9 C9 CJ CJ CA CK", "C8 C8 C9 C9 CJ CJ", true},
		{"C5 C5 C6 C6", "C8 C8 CJ CA D3 D4", "C8 C8 D3 D4", false},
		{"C5 C5 C6 C6", "C8 C8 D3 D4 D5 D6", "C8 C8 D3 D4", true},
		{"C5 C5", "D3 D3 D4 D5", "D4 D5", true},
	}
	for _, c := range cases {
		l, e := leadShape(cards(c.lead), r)
		if e != nil {
			t.Fatal(e)
		}
		e = validateFollow(cards(c.hand), cards(c.chosen), l, r)
		if (e == nil) != c.valid {
			t.Fatalf("lead %s hand %s chosen %s: %v", c.lead, c.hand, c.chosen, e)
		}
	}
}
func TestTrickWinningStructureAndTie(t *testing.T) {
	r := Rules{Decks: 2, Level: 2, Trump: 3}
	cases := []struct {
		lead, current, next string
		win                 bool
	}{
		{"C5 C5", "C5 C5", "SA SK", false},
		{"C5 C5", "C5 C5", "S3 S3", true},
		{"C5 C5 C6 C6", "C5 C5 C6 C6", "S7 S7 S9 S9", false},
		{"C5 C5 C6 C6", "C5 C5 C6 C6", "S7 S7 S8 S8", true},
		{"C2", "C2", "H2", false},
		{"C2", "C2", "S2", true},
		{"C5 C5", "S3 S3", "CA CA", false},
		{"CA CK", "S3 SA", "S4 SA", false},
	}
	for _, c := range cases {
		l, e := leadShape(cards(c.lead), r)
		if e != nil {
			t.Fatal(e)
		}
		if got := beatsTrick(cards(c.next), cards(c.current), l, r); got != c.win {
			t.Fatal(c, got)
		}
	}
}
func TestFailedThrowIsForcedAndIncludesPartner(t *testing.T) {
	r := Rules{Decks: 2, Level: 2, Trump: 3}
	ids := cards("CA CQ CQ CJ")
	hands := [][]int{ids, cards("C3"), cards("CK"), cards("D3")}
	forced, breaker, e := resolveThrow(ids, hands, 0, r)
	if e != nil || len(forced) != 1 || forced[0] != cards("CJ")[0] || breaker != 2 {
		t.Fatal(forced, breaker, e)
	}
	hands[2] = cards("C10")
	forced, breaker, e = resolveThrow(ids, hands, 0, r)
	if e != nil || len(forced) != 4 || breaker != -1 {
		t.Fatal("legal top throw failed", forced, breaker, e)
	}
}
func TestShanghaiFourSuits(t *testing.T) {
	r := Rules{Decks: 1, Level: 2, Trump: 3}
	l, e := leadShape(cards("C6 D6 H6 S6"), r)
	if e != nil || !l.Four {
		t.Fatal(l, e)
	}
	h := cards("C8 D8 H8 S8 C3 D3 H3 S3")
	if validateFollow(h, cards("C3 D3 H3 S3"), l, r) != nil {
		t.Fatal("four-suit following")
	}
	if validateFollow(h, cards("C3 C8 D3 H3"), l, r) == nil {
		t.Fatal("ignored held spade")
	}
	if !beatsTrick(cards("C8 D8 H8 S8"), cards("C6 D6 H6 S6"), l, r) {
		t.Fatal("higher four suits did not win")
	}
	if beatsTrick(cards("S3 S4 S5 S7"), cards("C6 D6 H6 S6"), l, r) {
		t.Fatal("four random trumps won four suits")
	}
}
func TestShanghaiScoreBoundaries(t *testing.T) {
	for _, c := range []struct {
		d, p int
		a    bool
		n    int
	}{
		{1, 0, false, 2}, {1, 5, false, 1}, {1, 35, false, 1}, {1, 40, true, 0}, {1, 75, true, 0}, {1, 80, true, 1}, {1, 95, true, 1}, {1, 100, true, 2}, {1, 200, true, 2},
		{2, 0, false, 3}, {2, 5, false, 2}, {2, 35, false, 2}, {2, 40, false, 1}, {2, 75, false, 1}, {2, 80, true, 0}, {2, 115, true, 0}, {2, 120, true, 1}, {2, 195, true, 2}, {2, 240, true, 4},
	} {
		a, n := scoreRound(c.p, c.d)
		if a != c.a || n != c.n {
			t.Fatal(c, a, n)
		}
	}
	r := Rules{Decks: 2, Level: 2, Trump: 3}
	s, _ := leadShape(cards("C5 C5 C6 C6 C7 C7"), r)
	if bottomFactor(s, "cards", 2) != 12 || bottomFactor(s, "structure", 2) != 16 || bottomFactor(s, "cards", 1) != 2 {
		t.Fatal("bottom multiplier")
	}
}
