package bubbleparty_test

// Failure scenarios precede implementation in REFACTOR_SCENARIOS.md.
import (
	"encoding/json"
	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"testing"
	"time"
)

type continuousPlayer struct {
	X       int     `json:"x"`
	Y       int     `json:"y"`
	PX      float64 `json:"px"`
	PY      float64 `json:"py"`
	SpeedMS int     `json:"speedMs"`
}

func continuousRead(t *testing.T, m gameapi.Match, seat int) continuousPlayer {
	t.Helper()
	v, err := bubbleparty.Module().View(m, seat)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Players []continuousPlayer `json:"players"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result.Players[seat]
}
func continuousInputAt(t *testing.T, m gameapi.Match, seat int, move string, at int64) gameapi.Match {
	t.Helper()
	v := refactorRead(t, m, seat)
	raw, _ := json.Marshal(bubbleparty.Action{Type: "input", Epoch: v.Epoch, Seq: v.Players[seat].Seq + 1, Move: move})
	next, err := bubbleparty.Module().Apply(gameapi.Context{Actor: seat, Players: 4, Now: time.UnixMilli(at)}, m, raw)
	if err != nil {
		t.Fatal(err)
	}
	return next
}
func TestContinuousShortTapBetweenFrames(t *testing.T) {
	m := refactorArena(t, nil)
	at := refactorRead(t, m, 0).ServerAt
	m = continuousInputAt(t, m, 0, "right", at)
	m = continuousInputAt(t, m, 0, "none", at+40)
	m = refactorTick(t, m, at+100)
	p := continuousRead(t, m, 0)
	if p.PX <= 5 || p.PX >= 5.5 || p.PY != 5 || p.X != 5 {
		t.Fatalf("40ms tap did not create bounded fractional movement: %+v", p)
	}
	if p.SpeedMS != 240 {
		t.Fatalf("missing speed for interpolation: %+v", p)
	}
	m = refactorTick(t, m, at+500)
	if q := continuousRead(t, m, 0); math.Abs(q.PX-p.PX) > 0.000001 {
		t.Fatalf("released character kept moving: before=%+v after=%+v", p, q)
	}
}
func TestContinuousWallBoundaryAndTurn(t *testing.T) {
	m := refactorArena(t, func(s *bubbleparty.State) { s.Cells[5*bubbleparty.Cols+6] = bubbleparty.CellWall })
	at := refactorRead(t, m, 0).ServerAt
	m = continuousInputAt(t, m, 0, "right", at)
	m = refactorTick(t, m, at+300)
	p := continuousRead(t, m, 0)
	if p.PX <= 5 || p.PX > 5.23 || p.X != 5 {
		t.Fatalf("wall should permit approach but block body crossing: %+v", p)
	}
	m = continuousInputAt(t, m, 0, "up", at+300)
	m = refactorTick(t, m, at+600)
	p = continuousRead(t, m, 0)
	if p.PY >= 5 || math.Abs(p.PX-5) > 0.000001 {
		t.Fatalf("turn did not align to column before moving: %+v", p)
	}
}
func TestContinuousRestorePreservesPosition(t *testing.T) {
	m := refactorArena(t, nil)
	at := refactorRead(t, m, 0).ServerAt
	old := continuousRead(t, m, 0)
	if old.PX != 5 || old.PY != 5 {
		t.Fatalf("old zero-offset save did not initialize center: %+v", old)
	}
	m = continuousInputAt(t, m, 0, "right", at)
	m = continuousInputAt(t, m, 0, "none", at+40)
	before := continuousRead(t, m, 0)
	restored, err := bubbleparty.Module().Restore(gameapi.Context{Now: time.UnixMilli(at + 40)}, m)
	if err != nil {
		t.Fatal(err)
	}
	after := continuousRead(t, restored, 0)
	if before.PX <= 5 || before != after {
		t.Fatalf("fractional save changed: before=%+v after=%+v", before, after)
	}
}

func TestContinuousOffCenterBombCanExitEitherWay(t *testing.T) {
	m := refactorArena(t, nil)
	at := refactorRead(t, m, 0).ServerAt
	m = continuousInputAt(t, m, 0, "right", at)
	m = refactorTick(t, m, at+20)
	m = refactorInput(t, m, 0, "left", true)
	m = refactorTick(t, m, at+120)
	p := continuousRead(t, m, 0)
	if p.PX >= 5 || len(refactorRead(t, m, 0).Bombs) != 1 {
		t.Fatalf("new bomb prevented choosing an exit before leaving its cell: %+v", p)
	}
}
