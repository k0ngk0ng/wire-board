package bubbleparty

import (
	"reflect"
	"testing"
)

// A trapped-in-corridor bot must be able to request a legal push without bombs,
// and planning must never move the box or mutate shared state itself.
func TestNativeBotCanPlanLegalPush(t *testing.T) {
	s := pushFixture()
	p := &s.Players[0]
	p.MaxBombs = 0
	s.setCell(4, 4, CellWall)
	s.setCell(4, 6, CellWall)
	s.setCell(3, 5, CellWall)
	before := append([]byte(nil), s.Cells...)
	move, bomb := botDecision(s, p, s.ServerAt)
	if move != "right" || bomb {
		t.Fatal("bot could not leave via legal box push", move, bomb)
	}
	if !reflect.DeepEqual(before, s.Cells) || len(s.BoxMotions) != 0 {
		t.Fatal("bot planning mutated box state")
	}
}

func TestNativeBotDoesNotRouteThroughUnpushableBox(t *testing.T) {
	s := pushFixture()
	p := &s.Players[0]
	p.MaxBombs = 0
	for _, c := range [][2]int{{4, 4}, {4, 6}, {3, 5}, {6, 5}} {
		s.setCell(c[0], c[1], CellWall)
	}
	move, bomb := botDecision(s, p, s.ServerAt)
	if move != "none" || bomb {
		t.Fatal("bot routed through an occupied crate", move, bomb)
	}
}

// A flying bot above a crate cannot place a bomb there. Planning must keep
// moving toward an empty cell instead of repeating an impossible placement.
func TestNativeFlyingBotLeavesBoxBeforeBombing(t *testing.T) {
	s := pushFixture()
	p := &s.Players[0]
	p.X, p.Y, p.OffsetX, p.OffsetY = 5, 5, 0, 0
	p.Mount = "airship"
	s.setCell(6, 5, CellCrate)
	move, bomb := botDecision(s, p, s.ServerAt)
	if bomb || move == "none" {
		t.Fatal("flying bot stalls on impossible bomb", move, bomb)
	}
}
