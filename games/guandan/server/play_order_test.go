package guandan

// Failure cases: full-house wings precede the body; wild card placement follows
// its printed level instead of declared role; ace-low runs wrap at the wrong end.
import (
	"reflect"
	"testing"
)

func TestPlayedCardsRespectDeclaredWildOrder(t *testing.T) {
	cases := []struct {
		spec, kind string
		high       int
		want       []int
	}{
		{"C8 D8 C3 D3 H3", "full_house", 3, []int{3, 3, 3, 8, 8}},
		{"C8 D8 C3 D3 H7", "full_house", 3, []int{3, 3, 3, 8, 8}},
		{"CA D2 H3 S4 C5", "straight", 5, []int{5, 4, 3, 2, 14}},
	}
	for _, tc := range cases {
		ids := cards(tc.spec)
		before := append([]int{}, ids...)
		cs, e := interpret(ids, 7)
		if e != nil {
			t.Fatal(e)
		}
		var chosen *Combination
		for i := range cs {
			if cs[i].Kind == tc.kind && cs[i].High == tc.high {
				chosen = &cs[i]
				break
			}
		}
		if chosen == nil {
			t.Fatal("missing interpretation")
		}
		got := orderedPlay(ids, *chosen, 7)
		if !reflect.DeepEqual(before, ids) {
			t.Fatal("mutated input")
		}
		for i, id := range got {
			n := natural(id)
			for _, w := range chosen.Wild {
				if w.Card == id {
					n = w.Rank
				}
			}
			if n != tc.want[i] {
				t.Fatalf("%s: %v", tc.spec, got)
			}
		}
	}
}
