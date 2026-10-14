package shengji

// Failure cases: trump cards are at the end; low ranks are on the left;
// physical pairs get split by equal-strength off-suit levels; no-trump hides
// level cards among their printed suits.
import (
	"testing"
)

func TestHandPresentationTrumpFirstAndDescending(t *testing.T) {
	for _, trump := range []int{-1, 1} {
		r := Rules{Decks: 2, Level: 7, Trump: trump}
		ids := cards("C3 DA H7 D7 S7 BJ RJ CA D3 H7 C7 D7")
		sortHand(ids, r)
		if rank(ids[0]) != 17 || rank(ids[1]) != 16 {
			t.Fatal("jokers must lead", ids)
		}
		sawSide := false
		prev := 100
		for _, id := range ids {
			if suitOf(id, r) != 4 {
				sawSide = true
			} else {
				if sawSide {
					t.Fatal("trump after side suit")
				}
				if power(id, r) > prev {
					t.Fatal("ascending trumps")
				}
				prev = power(id, r)
			}
		}
		for i := 1; i < len(ids); i++ {
			if suitOf(ids[i], r) == suitOf(ids[i-1], r) && power(ids[i], r) > power(ids[i-1], r) {
				t.Fatal("same-suit order ascending")
			}
		}
		for base := 0; base < 54; base++ {
			at := []int{}
			for i, id := range ids {
				if id%54 == base {
					at = append(at, i)
				}
			}
			if len(at) == 2 && at[1] != at[0]+1 {
				t.Fatal("identical pair separated")
			}
		}
	}
}

// A legal follower can be unable to match the lead's pair/tractor. Its sound
// must describe its actual cards, not repeat the leader's combination.
func TestPlaySoundUsesActualFollowStructure(t *testing.T) {
	r := Rules{Decks: 2, Level: 2, Trump: 3}
	for _, tc := range []struct {
		spec   string
		leader bool
		kind   string
		value  int
	}{{"C3 D4", false, "follow", 0}, {"DJ", false, "single", 11}, {"RJ RJ", false, "pair", 17}, {"C3 C3 C4 C4", false, "tractor", 0}, {"C3 C5", true, "throw", 0}, {"C3 C5", false, "follow", 0}} {
		kind, value := playSound(cards(tc.spec), tc.leader, r)
		if kind != tc.kind || value != tc.value {
			t.Fatal(tc, kind, value)
		}
	}
}
