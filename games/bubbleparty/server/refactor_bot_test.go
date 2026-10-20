package bubbleparty

// Additional failure scenarios are recorded in ../tests/REFACTOR_SCENARIOS.md.
import (
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
	"time"
)

func botArena(t *testing.T) *State {
	t.Helper()
	s, _ := newMatch(t, 4, "community10", 44)
	beginPlaying(t, s)
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			c := byte(CellEmpty)
			if x == 0 || y == 0 || x == Cols-1 || y == Rows-1 {
				c = CellWall
			}
			s.setCell(x, y, c)
		}
	}
	for i, pos := range [][2]int{{5, 5}, {6, 5}, {10, 9}, {11, 9}} {
		s.Players[i].X, s.Players[i].Y = pos[0], pos[1]
	}
	return s
}

func TestRefactorBotTouchesTrappedPlayers(t *testing.T) {
	for _, enemy := range []bool{false, true} {
		s := botArena(t)
		target := 1
		if enemy {
			target = 2
			s.Players[1].X, s.Players[1].Y = 4, 9
			s.Players[2].X, s.Players[2].Y = 6, 5
		}
		s.Players[target].TrappedUntil = s.ServerAt + 5000
		move, bomb := botDecision(s, &s.Players[0], s.ServerAt)
		if move != "right" || bomb {
			t.Fatalf("enemy=%v touch action=%q bomb=%v", enemy, move, bomb)
		}
	}
}

func TestRefactorBotRefusesBombWithoutEscape(t *testing.T) {
	s := botArena(t)
	for _, d := range directions {
		s.setCell(5+d[0], 5+d[1], CellCrate)
	}
	_, bomb := botDecision(s, &s.Players[0], s.ServerAt)
	if bomb {
		t.Fatal("bot placed a bomb in a sealed dead end")
	}
}

func TestRefactorBotEscapesAlongFutureBlastLane(t *testing.T) {
	s := botArena(t)
	p := &s.Players[0]
	s.setCell(5, 4, CellWall)
	s.setCell(5, 6, CellWall)
	s.setCell(4, 5, CellWall)
	s.placeBomb(p, s.ServerAt)
	s.Bombs[0].Range = 2
	move, bomb := botDecision(s, p, s.ServerAt)
	if move != "right" || bomb {
		t.Fatalf("escape through future blast lane=%q bomb=%v", move, bomb)
	}
}

func TestRefactorReclaimPreservesTrappedDeadline(t *testing.T) {
	s := botArena(t)
	s.Players[0].TrappedUntil = s.ServerAt + 5000
	m, _ := s.match()
	ctx := gameapi.Context{Actor: 0, Now: time.UnixMilli(s.ServerAt)}
	restored, err := reclaim(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decode(restored)
	if err != nil {
		t.Fatal(err)
	}
	if got.Players[0].TrappedUntil != s.Players[0].TrappedUntil || got.Players[1].IdleAt != s.Players[1].IdleAt {
		t.Fatal("reclaim changed trap or another player's deadline")
	}
}

func TestRefactorBotClosesFractionalRescueDistance(t *testing.T) {
	s := botArena(t)
	s.Players[0].OffsetX = -0.45
	s.Players[1].X = 5
	s.Players[1].OffsetX = 0.45
	s.Players[1].TrappedUntil = s.ServerAt + 5000
	move, bomb := botDecision(s, &s.Players[0], s.ServerAt)
	if move != "right" || bomb {
		t.Fatalf("same logical cell rescue still needs contact: %q bomb=%v", move, bomb)
	}
}
