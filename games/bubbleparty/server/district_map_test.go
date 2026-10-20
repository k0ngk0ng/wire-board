package bubbleparty

// Failure scenarios and provisional collision assumptions precede this test in
// ../tests/REFACTOR_SCENARIOS.md. The reference is a visual, not collision, dump.
import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func districtReference(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../../docs/research/bubbleparty/reference-evidence/district-10-visual-grid.json")
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Width  int      `json:"width"`
		Height int      `json:"height"`
		Rows   []string `json:"rows"`
	}
	if err = json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.Width != Cols || ref.Height != Rows || len(ref.Rows) != Rows {
		t.Fatal("invalid source dimensions")
	}
	return ref.Rows
}

func TestDistrictReferenceLayout(t *testing.T) {
	ref := districtReference(t)
	cells := mapCells("community10", 1)
	if !reflect.DeepEqual(cells, mapCells("community10", 99999)) {
		t.Fatal("reference district varies with random seed")
	}
	for y, row := range ref {
		if len(row) != Cols {
			t.Fatalf("row %d width %d", y, len(row))
		}
		for x, c := range row {
			want := byte(CellEmpty)
			switch c {
			case 'H':
				want = CellWall
			case 'R', 'O', 'C':
				want = CellCrate
			}
			if got := cells[y*Cols+x]; got != want {
				t.Fatalf("reference (%d,%d) %c: collision %d want %d", x, y, c, got, want)
			}
		}
	}
	if cells[0] != CellEmpty || cells[len(cells)-1] != CellEmpty {
		t.Fatal("invented perimeter wall")
	}
}

func TestDistrictSpawnSafetyAllCounts(t *testing.T) {
	expected := [][][2]int{{{0, 0}, {1, 11}, {6, 6}, {7, 1}}, {{14, 12}, {13, 1}, {8, 7}, {7, 10}}}
	for _, players := range []int{2, 4, 6, 8} {
		s, _ := newMatch(t, players, "community10", 12)
		beginPlaying(t, s)
		seen := map[[2]int]bool{}
		for i, p := range s.Players {
			pos := [2]int{p.X, p.Y}
			want := expected[p.Team][i%s.TeamSize]
			if pos != want || seen[pos] || s.cell(p.X, p.Y) != CellEmpty {
				t.Fatalf("%d players seat %d unsafe/repeated spawn %v want %v", players, i, pos, want)
			}
			seen[pos] = true
			if !canEscapePlacedBomb(s, &s.Players[i], s.ServerAt) {
				t.Fatalf("%d players seat %d has no first-bomb escape", players, i)
			}
		}
		// Destructible bricks may be opened during play, but houses cannot isolate a spawn.
		reachable := map[[2]int]bool{{s.Players[0].X, s.Players[0].Y}: true}
		queue := [][2]int{{s.Players[0].X, s.Players[0].Y}}
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, d := range directions {
				q := [2]int{p[0] + d[0], p[1] + d[1]}
				if q[0] < 0 || q[0] >= Cols || q[1] < 0 || q[1] >= Rows || reachable[q] || s.cell(q[0], q[1]) == CellWall {
					continue
				}
				reachable[q] = true
				queue = append(queue, q)
			}
		}
		for _, p := range s.Players {
			if !reachable[[2]int{p.X, p.Y}] {
				t.Fatalf("seat %d isolated by hard walls", p.Seat)
			}
		}
	}
}

func TestDistrictTilesProjectAndDestruction(t *testing.T) {
	s, m := newMatch(t, 8, "community10", 22)
	v, err := project(m, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	var decoded struct {
		Tiles []int `json:"tiles"`
	}
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Tiles) != Cols*Rows {
		t.Fatalf("missing public tile metadata: %d", len(decoded.Tiles))
	}
	checks := map[[2]int]int{{7, 0}: 1, {5, 0}: 3, {5, 1}: 2, {9, 2}: 3, {9, 11}: 2, {9, 12}: 3, {0, 6}: 3, {1, 1}: 4, {0, 8}: 5, {10, 0}: 6, {2, 0}: 7, {1, 0}: 8, {8, 0}: 9}
	for pos, want := range checks {
		if got := decoded.Tiles[pos[1]*Cols+pos[0]]; got != want {
			t.Fatalf("tile %v=%d want%d", pos, got, want)
		}
	}
	// Remove an untouched reference road crate and a brick. Projection must
	// restore their distinct substrate rather than keeping destroyed art.
	s.setCell(8, 0, CellEmpty)
	s.setCell(1, 0, CellEmpty)
	m, err = s.match()
	if err != nil {
		t.Fatal(err)
	}
	v, err = project(m, -1)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(v)
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Tiles[8] != 1 || decoded.Tiles[1] != 0 {
		t.Fatalf("destroyed decoration remained: road=%d lawn=%d", decoded.Tiles[8], decoded.Tiles[1])
	}
}
