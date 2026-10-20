package bubbleparty

// Failure inventory: docs/research/realtime-efficiency/MOTION.md. Original
// object taxonomy and screenshot classification: geometry-original-evidence.md.
import (
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

var referencePines = map[[2]int]bool{{0, 6}: true, {2, 6}: true, {4, 6}: true, {10, 6}: true, {12, 6}: true, {14, 6}: true}

func TestDistrictPinesAndPassableHerbsAgreeWithCollision(t *testing.T) {
	cells := mapCells("community10", 1)
	tiles := mapTiles("community10", cells)
	trees, herbs := 0, 0
	for y, row := range districtReference(t) {
		for x, mark := range row {
			if mark != 'T' {
				continue
			}
			i := y*Cols + x
			if referencePines[[2]int{x, y}] {
				trees++
				if cells[i] != CellWall || tiles[i] != TileTree {
					t.Fatalf("pine (%d,%d) must be a visible solid tree, cell=%d tile=%d", x, y, cells[i], tiles[i])
				}
			} else {
				herbs++
				if cells[i] != CellEmpty || tiles[i] != TileHedge {
					t.Fatalf("fruit herb (%d,%d) incorrectly blocked or drawn as pine: %d/%d", x, y, cells[i], tiles[i])
				}
			}
		}
	}
	if trees != 6 || herbs < 20 {
		t.Fatal("incomplete visual classification", trees, herbs)
	}
}
func TestDistrictPineBlocksMovementBombAndBlast(t *testing.T) {
	s, _ := newMatch(t, 2, "community10", 13)
	beginPlaying(t, s)
	p := &s.Players[0]
	p.X, p.Y = 5, 6
	for i := 0; i < 10; i++ {
		s.movePlayer(p, -1, 0, .2, s.ServerAt+int64(i)*StepMS, "left")
	}
	if x := float64(p.X) + p.OffsetX; x < 4.78-0.000001 || p.X != 5 {
		t.Fatal("walked into pine", p.X, p.OffsetX)
	}
	if s.canWalk(p, 4, 6) {
		t.Fatal("pine accepts walking")
	}
	p.X, p.Y, p.OffsetX, p.OffsetY = 5, 6, 0, 0
	p.Range = 4
	s.placeBomb(p, s.ServerAt)
	if len(s.Bombs) != 1 {
		t.Fatal("fixture bomb not placed")
	}
	s.advance(s.Bombs[0].ExplodeAt + StepMS)
	if s.cell(4, 6) != CellWall {
		t.Fatal("pine was destroyed")
	}
	for _, b := range s.Blasts {
		if b.Y == 6 && b.X <= 4 {
			t.Fatal("water crossed solid pine", b)
		}
	}
}
func TestDistrictHerbAllowsWalkingAndLegacyEmptyTreeStaysConsistent(t *testing.T) {
	s, _ := newMatch(t, 2, "community10", 17)
	beginPlaying(t, s)
	p := &s.Players[0]
	p.X, p.Y = 5, 6
	for i := 0; i < 5; i++ {
		s.movePlayer(p, 0, 1, .2, s.ServerAt+int64(i)*StepMS, "down")
	}
	if p.Y != 7 {
		t.Fatal("passable herb blocked", p.X, p.Y, p.OffsetY)
	}
	// An old match may have a human on a formerly decorative pine. Restore must
	// not create a wall underneath them; the view must show a passable herb.
	s.setCell(4, 6, CellEmpty)
	p.X, p.Y, p.OffsetX, p.OffsetY = 4, 6, 0, 0
	before, err := s.match()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := restore(gameapi.Context{Players: 2, Now: time.UnixMilli(s.ServerAt)}, before)
	if err != nil {
		t.Fatal(err)
	}
	state, err := decode(restored)
	if err != nil {
		t.Fatal(err)
	}
	if state.cell(4, 6) != CellEmpty || state.Players[0].X != 4 || state.Epoch != s.Epoch || state.MatchEndsAt != s.MatchEndsAt {
		t.Fatal("legacy restoration changed gameplay")
	}
	if tile := mapTiles(state.MapID, state.Cells)[6*Cols+4]; tile != TileHedge {
		t.Fatal("legacy walkable cell still drawn as solid pine", tile)
	}
}
