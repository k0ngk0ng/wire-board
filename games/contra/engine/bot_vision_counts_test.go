package contra

// Before color-bound pruning: expanded tiles must never undercount an actual
// candidate rectangle, including narrow/unaligned edges and a full-color screen.
// Rejection must preserve exact-only and late partial matches.
import "testing"

func TestBotPixelCountsAreConservativeAtEdges(t *testing.T) {
	locations := map[uint32][]int{1: {0, 7*256 + 7, 239*256 + 255}, 2: {8*256 + 8}}
	counts := newBotPixelCounts(locations)
	for _, c := range []struct {
		color            uint32
		x, y, w, h, want int
	}{{1, 0, 0, 1, 1, 2}, {1, 0, 0, 8, 8, 2}, {1, 7, 7, 2, 2, 2}, {2, 7, 7, 2, 2, 1}, {1, 255, 239, 1, 1, 1}, {1, 0, 0, 256, 240, 3}, {9, 0, 0, 256, 240, 0}} {
		if got := counts.upper(c.color, c.x, c.y, c.w, c.h); got != c.want {
			t.Fatal(c, got)
		}
	}
	all := make([]int, 256*240)
	for i := range all {
		all[i] = i
	}
	dense := newBotPixelCounts(map[uint32][]int{3: all})
	if dense.upper(3, 0, 0, 256, 240) != 61440 || dense.upper(3, 8, 8, 8, 8) != 64 {
		t.Fatal("integral count overflow")
	}
	seed := uint32(42)
	for n := 0; n < 1000; n++ {
		seed = 1664525*seed + 1013904223
		x := int(seed % 256)
		seed = 1664525*seed + 1013904223
		y := int(seed % 240)
		seed = 1664525*seed + 1013904223
		w := 1 + int(seed%uint32(256-x))
		seed = 1664525*seed + 1013904223
		h := 1 + int(seed%uint32(240-y))
		for color, points := range locations {
			exact := 0
			for _, p := range points {
				xx, yy := p%256, p/256
				if xx >= x && xx < x+w && yy >= y && yy < y+h {
					exact++
				}
			}
			if counts.upper(color, x, y, w, h) < exact {
				t.Fatal("visible color bound undercounted")
			}
		}
	}
}

func TestBotCandidateColorBoundPreservesPossibleMatches(t *testing.T) {
	pattern, pixels := matchFixture(t, 192, []uint32{0x410000, 0x004100}, func(_, y, _ int) bool { return y >= 4 }, 110, 80)
	locations := map[uint32][]int{}
	for i, color := range pixels {
		locations[color] = append(locations[color], i)
	}
	counts := newBotPixelCounts(locations)
	if !counts.possible(pattern, 110, 80, true) {
		t.Fatal("accepted partial match pruned")
	}
	if counts.possible(pattern, 10, 10, true) {
		t.Fatal("blank candidate not pruned")
	}
	pattern, pixels = matchFixture(t, 192, []uint32{0x410000}, func(int, int, int) bool { return true }, 0, 0)
	locations = map[uint32][]int{}
	for i, color := range pixels {
		locations[color] = append(locations[color], i)
	}
	if !newBotPixelCounts(locations).possible(pattern, 0, 0, true) {
		t.Fatal("exact single-color image pruned")
	}
}
