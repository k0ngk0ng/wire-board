package tetris

import (
	"testing"
)

func TestSevenBagCoversEachKindOnce(t *testing.T) {
	p := newPlayer(0, 0x123456789abcdef)
	for round := 0; round < 20; round++ {
		seen := map[Kind]int{}
		for _, k := range nextBag(&p) {
			if k < I || k > Z {
				t.Fatalf("invalid kind %d", k)
			}
			seen[k]++
		}
		if len(seen) != 7 {
			t.Fatalf("bag %d has %d kinds", round, len(seen))
		}
		for k, n := range seen {
			if n != 1 {
				t.Fatalf("bag %d kind %d count %d", round, k, n)
			}
		}
	}
}

func TestQueueAlwaysHasPreview(t *testing.T) {
	p := newPlayer(0, 42)
	for i := 0; i < 80; i++ {
		if len(p.Queue) < 5 {
			t.Fatalf("queue shrank to %d", len(p.Queue))
		}
		if !spawn(&p, int64(i)) {
			t.Fatalf("spawn failed at %d", i)
		}
		p.Active = nil
	}
}

func TestRotationsAndWallKicksStayInBounds(t *testing.T) {
	cases := []struct {
		kind Kind
		x    int
	}{
		{T, 0}, {J, 0}, {L, 0}, {S, 0}, {Z, 0}, {I, 0},
	}
	for _, c := range cases {
		p := newPlayer(0, 1)
		p.Board = emptyBoard()
		p.Active = &Piece{ID: 1, Kind: c.kind, Rot: 0, X: c.x, Y: 10}
		if !fits(p.Board, p.Active.Kind, p.Active.Rot, p.Active.X, p.Active.Y) {
			t.Fatalf("%d does not fit at start", c.kind)
		}
		for i := 0; i < 4; i++ {
			if !tryRotate(&p, 1) && c.kind != O {
				t.Fatalf("%d rotate failed at rotation %d x=%d", c.kind, i, p.Active.X)
			}
			for _, pt := range shapeCells(p.Active.Kind, p.Active.Rot) {
				x := p.Active.X + pt.X
				if x < 0 || x >= BoardWidth {
					t.Fatalf("%d left board after rotate: x=%d", c.kind, x)
				}
			}
		}
	}
}

func TestHardDropLandsOnStack(t *testing.T) {
	p := newPlayer(0, 7)
	p.Board = emptyBoard()
	for x := 0; x < BoardWidth; x++ {
		setCell(&p.Board, x, BoardRows-1, Garbage)
	}
	p.Active = &Piece{ID: 1, Kind: O, Rot: 0, X: 4, Y: 1}
	y := dropY(p.Board, p.Active.Kind, p.Active.Rot, p.Active.X, p.Active.Y)
	if y != BoardRows-3 {
		t.Fatalf("drop y=%d want %d", y, BoardRows-3)
	}
}

func TestClearFullRowsAndScoring(t *testing.T) {
	b := emptyBoard()
	for x := 0; x < BoardWidth; x++ {
		setCell(&b, x, BoardRows-1, T)
		setCell(&b, x, BoardRows-2, T)
	}
	setCell(&b, 0, BoardRows-3, I)
	cleared := clearFullRows(&b)
	if len(cleared) != 2 {
		t.Fatalf("cleared=%v", cleared)
	}
	if got := cellAt(b, 0, BoardRows-1); got != I {
		t.Fatalf("stack cell should fall two rows: %v", got)
	}
	if got := cellAt(b, 0, BoardRows-3); got != Empty {
		t.Fatalf("old stack cell should be empty: %v", got)
	}
	if got := scoreForLines(1, 1); got != 100 {
		t.Fatalf("single score=%d", got)
	}
	if got := scoreForLines(4, 1); got != 800 {
		t.Fatalf("tetris score=%d", got)
	}
	if got := scoreForLines(4, 2); got != 1600 {
		t.Fatalf("level score=%d", got)
	}
}

func TestGarbageInsertAndTopOut(t *testing.T) {
	b := emptyBoard()
	if insertGarbage(&b, []int{3, 4}) {
		t.Fatal("unexpected top out")
	}
	if got := cellAt(b, 4, BoardRows-1); got != Empty {
		t.Fatalf("newest hole column not empty: %v", got)
	}
	if got := cellAt(b, 3, BoardRows-2); got != Empty {
		t.Fatalf("older hole column not empty: %v", got)
	}
	if got := cellAt(b, 0, BoardRows-2); got != Garbage {
		t.Fatalf("garbage missing: %v", got)
	}
	b = emptyBoard()
	for x := 0; x < BoardWidth; x++ {
		setCell(&b, x, 0, T)
	}
	if !insertGarbage(&b, []int{0}) {
		t.Fatal("top row should top out")
	}
}

func TestGravityAndLevel(t *testing.T) {
	if got := gravityFor(1); got != 800 {
		t.Fatalf("level 1 gravity=%d", got)
	}
	if got := gravityFor(5); got != 560 {
		t.Fatalf("level 5 gravity=%d", got)
	}
	if got := gravityFor(20); got != 100 {
		t.Fatalf("level 20 gravity=%d", got)
	}
	if got := levelForLines(0); got != 1 {
		t.Fatalf("level 0 lines=%d", got)
	}
	if got := levelForLines(19); got != 2 {
		t.Fatalf("level 19 lines=%d", got)
	}
	if got := levelForLines(20); got != 3 {
		t.Fatalf("level 20 lines=%d", got)
	}
}

func TestLockDelayResetsAreCapped(t *testing.T) {
	p := newPlayer(0, 9)
	p.Board = emptyBoard()
	p.Active = &Piece{ID: 1, Kind: O, Rot: 0, X: 4, Y: BoardRows - 2}
	p.LockAt = 1000
	for i := 0; i < 30; i++ {
		resetLock(&p, int64(1000+i))
	}
	if p.LockResets > MaxLockResets {
		t.Fatalf("lock resets=%d", p.LockResets)
	}
	if p.LockAt == 0 {
		t.Fatal("lock delay was cleared")
	}
}

func TestTargetsSkipEliminatedAndSelf(t *testing.T) {
	s := &State{Options: Options{Mode: ModeSurvival}, Players: []Player{
		{Seat: 0, Alive: true}, {Seat: 1, Alive: false}, {Seat: 2, Alive: true}, {Seat: 3, Alive: true},
	}}
	normalizeTargets(s)
	if s.Players[0].TargetSeat != 2 || s.Players[2].TargetSeat != 3 || s.Players[3].TargetSeat != 0 {
		t.Fatalf("targets: %d %d %d", s.Players[0].TargetSeat, s.Players[2].TargetSeat, s.Players[3].TargetSeat)
	}
}

func TestRankingCompetitionAndDraw(t *testing.T) {
	s := &State{Options: Options{Mode: ModeSprint}, Players: []Player{
		{Seat: 0, Lines: 40, Score: 4000, Alive: true},
		{Seat: 1, Lines: 20, Score: 2000, Alive: true},
		{Seat: 2, Lines: 20, Score: 2000, Alive: true},
	}}
	r := resultFor(s, 1000, "sprint_finished")
	if r == nil || len(r.Ranks) != 3 {
		t.Fatalf("result=%+v", r)
	}
	if r.Ranks[0] != 1 || r.Ranks[1] != 2 || r.Ranks[2] != 2 {
		t.Fatalf("ranks=%v", r.Ranks)
	}
	if len(r.Winners) != 1 || r.Winners[0] != 0 {
		t.Fatalf("winners=%v", r.Winners)
	}
	s2 := &State{Options: Options{Mode: ModeSurvival}, Players: []Player{
		{Seat: 0, Lines: 3, Score: 300, Alive: false, EliminatedAt: 500},
		{Seat: 1, Lines: 3, Score: 300, Alive: false, EliminatedAt: 500},
	}}
	r2 := resultFor(s2, 1000, "time_limit")
	if r2 == nil || !r2.Draw || len(r2.Winners) != 0 || r2.Ranks[0] != 1 || r2.Ranks[1] != 1 {
		t.Fatalf("draw result=%+v", r2)
	}
}
