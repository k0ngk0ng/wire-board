package doudizhu

// Failure cases before changing presentation: manual click order is published;
// a higher attachment appears before the triple/airplane body; ordering mutates
// the submitted physical cards or changes the recognized combination.
import (
	"reflect"
	"testing"
)

func TestPlayedCardsAreGroupedAndDescending(t *testing.T) {
	for _, c := range []struct{ input, want string }{{"5 3 5 3 3", "3 3 3 5 5"}, {"3 7 4 6 5", "7 6 5 4 3"}, {"BJ RJ BJ RJ", "RJ RJ BJ BJ"}, {"8 3 4 8 3 4 9 3 9 4", "4 4 4 3 3 3 9 9 8 8"}} {
		ids := cards(c.input)
		before := append([]int{}, ids...)
		combo, e := classify(ids, 4, true)
		if e != nil {
			t.Fatal(e)
		}
		got := orderedPlay(ids, combo)
		if !reflect.DeepEqual(ids, before) {
			t.Fatal("mutated input")
		}
		want := cards(c.want)
		for i, id := range got {
			if rank(id) != rank(want[i]) {
				t.Fatalf("%s -> %v; want %s", c.input, got, c.want)
			}
		}
		after, e := classify(got, 4, true)
		if e != nil || after != combo {
			t.Fatal("presentation changed combination")
		}
	}
}
