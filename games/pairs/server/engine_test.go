package pairs

import (
	"reflect"
	"testing"
)

// Failure inventory: README.md. These tests precede the server-side port.
func TestPathsAndObstacles(t *testing.T) {
	b := Board{Rows: 2, Cols: 3, Cells: []*Cell{{Kind: "tile", Icon: 1}, {Kind: "stone"}, {Kind: "tile", Icon: 1}, {Kind: "tile", Icon: 2}, {Kind: "tile", Icon: 3}, {Kind: "tile", Icon: 4}}}
	path := FindPath(b, 0, 2, false)
	if len(path) != 4 || path[1].Y != -1 {
		t.Fatalf("outside route missing: %v", path)
	}
	for _, pair := range [][2]int{{0, 0}, {-1, 2}, {0, 6}, {0, 3}, {0, 1}} {
		if FindPath(b, pair[0], pair[1], false) != nil {
			t.Fatal("illegal pair", pair)
		}
	}
	b.Cells[2].Ice = true
	if FindPath(b, 0, 2, false) != nil {
		t.Fatal("frozen tile selectable")
	}
	b = Board{Rows: 2, Cols: 3, Cells: []*Cell{{Kind: "tile", Icon: 1}, {Kind: "tile", Icon: 1}, {Kind: "crate"}, {Kind: "tile", Icon: 2, Ice: true}, {Kind: "tile", Icon: 2}, nil}}
	original := b.Clone()
	next, ok := RemovePair(b, 0, 1)
	if !ok || next.Cells[2] != nil || next.Cells[3].Ice || !reflect.DeepEqual(b, original) {
		t.Fatal("adjacency effect or input mutation")
	}
	if _, ok = RemovePair(next, 3, 4); !ok {
		t.Fatal("thawed pair cannot clear")
	}
	blocked := Board{Rows: 3, Cols: 3, Cells: []*Cell{{Kind: "stone"}, {Kind: "stone"}, {Kind: "tile", Icon: 1}, {Kind: "stone"}, {Kind: "tile", Icon: 1}, {Kind: "stone"}, {Kind: "stone"}, {Kind: "stone"}, {Kind: "stone"}}}
	if FindPath(blocked, 2, 4, false) != nil {
		t.Fatal("route passed through obstacle")
	}
}
func TestGenerationAndRecovery(t *testing.T) {
	mechanics := map[string]bool{}
	for n := 1; n <= 70; n++ {
		for seed := uint32(1); seed <= 3; seed++ {
			l := GenerateLevel(n, seed*7919)
			mechanics[l.Mechanic] = true
			if !VerifySolution(l.Board, l.Solution) {
				t.Fatalf("unsolvable n=%d seed=%d", n, seed)
			}
			if !reflect.DeepEqual(l, GenerateLevel(n, seed*7919)) {
				t.Fatal("nondeterministic generator")
			}
			shuffled, _ := Reshuffle(l.Board, seed)
			if TileCount(shuffled) != TileCount(l.Board) || FindMove(shuffled) == nil {
				t.Fatal("invalid reshuffle")
			}
		}
	}
	if len(mechanics) != 7 {
		t.Fatal("lost source mechanics", mechanics)
	}
	b := Board{Rows: 2, Cols: 2, Cells: []*Cell{{Kind: "tile", Icon: 1, Ice: true}, {Kind: "tile", Icon: 2, Ice: true}, {Kind: "tile", Icon: 1, Ice: true}, {Kind: "tile", Icon: 2, Ice: true}}}
	next, relaxed := Reshuffle(b, 1)
	if !relaxed || TileCount(next) != 4 || FindMove(next) == nil {
		t.Fatal("deadlock fallback")
	}
}
