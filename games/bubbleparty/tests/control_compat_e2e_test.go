package bubbleparty_test

// Confirmed review scenarios are recorded before implementation in REFACTOR_SCENARIOS.md.
import (
	"encoding/json"
	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"testing"
	"time"
)

func TestControlFailedBombEdgeIsNotQueued(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, reason := range []string{"capacity", "cooldown"} {
			label := reason
			if legacy {
				label += "-legacy"
			}
			t.Run(label, func(t *testing.T) {
				m := refactorArena(t, func(s *bubbleparty.State) {
					s.SuddenAt = s.ServerAt + 180000
					if reason == "cooldown" {
						s.Players[0].NextBombAt = s.ServerAt + 300
					} else {
						s.Bombs = []bubbleparty.Bomb{{ID: 1, Seat: 0, Team: 0, X: 1, Y: 1, Range: 1, At: s.ServerAt, ExplodeAt: s.ServerAt + 300}}
						s.BombID = 1
						s.Cells[bubbleparty.Cols+1] = bubbleparty.CellBomb
					}
				})
				if legacy {
					m = classicLegacy(t, m)
				}
				at := refactorRead(t, m, 0).ServerAt
				m = refactorInput(t, m, 0, "none", true)
				m = refactorTick(t, m, at+100)
				m = refactorInput(t, m, 0, "right", false)
				m = refactorTick(t, m, at+200)
				m = refactorInput(t, m, 0, "none", false)
				m = refactorTick(t, m, at+600)
				if bombs := refactorRead(t, m, 0).Bombs; len(bombs) != 0 {
					t.Fatalf("failed edge later placed an unsolicited bomb: %+v", bombs)
				}
				m = refactorInput(t, m, 0, "none", true)
				m = refactorTick(t, m, at+700)
				if bombs := refactorRead(t, m, 0).Bombs; len(bombs) != 1 {
					t.Fatalf("fresh post-failure edge did not place bomb: %+v", bombs)
				}
			})
		}
	}
}

func TestControlRescueDoesNotResumeTrappedInputs(t *testing.T) {
	for _, source := range []string{"applied", "queued", "while-trapped"} {
		t.Run(source, func(t *testing.T) {
			m := refactorArena(t, func(s *bubbleparty.State) {
				s.Players[0].Applied = bubbleparty.Input{At: s.ServerAt, Move: "up"}
				s.Players[1].Shield = true
				if source == "queued" {
					s.Players[0].Pending = []bubbleparty.Input{{At: s.ServerAt + 60, Move: "right", Bomb: true}}
				}
				s.Bombs = []bubbleparty.Bomb{{ID: 1, Seat: 2, Team: 1, X: 6, Y: 5, Range: 1, At: s.ServerAt - 2000, ExplodeAt: s.ServerAt + 20}}
				s.BombID = 1
				s.Cells[5*bubbleparty.Cols+6] = bubbleparty.CellBomb
			})
			at := refactorRead(t, m, 0).ServerAt
			m = refactorTick(t, m, at+20)
			before := continuousRead(t, m, 0)
			if refactorRead(t, m, 0).Players[0].TrappedUntil == 0 {
				t.Fatal("fixture did not trap moving player")
			}
			if source == "while-trapped" {
				m = refactorInput(t, m, 0, "right", true)
				if refactorRead(t, m, 0).Players[0].Seq != 1 {
					t.Fatal("trapped input did not acknowledge sequence")
				}
			}
			m = refactorInput(t, m, 1, "right", false)
			m = refactorTick(t, m, at+200)
			v := refactorRead(t, m, 0)
			after := continuousRead(t, m, 0)
			if !v.Players[0].Alive || v.Players[0].TrappedUntil > 0 {
				t.Fatal("shielded teammate did not rescue quickly")
			}
			if math.Abs(after.PX-before.PX) > 0.000001 || math.Abs(after.PY-before.PY) > 0.000001 || len(v.Bombs) != 0 {
				t.Fatalf("rescued player resumed stale controls: before=%+v after=%+v bombs=%+v", before, after, v.Bombs)
			}
			m = refactorInput(t, m, 0, "up", false)
			m = refactorTick(t, m, at+300)
			if continuousRead(t, m, 0).PY >= after.PY {
				t.Fatal("fresh movement after rescue was incorrectly disabled")
			}
		})
	}
}

func TestLegacyWaterAndShieldKeepOriginalRules(t *testing.T) {
	for _, shield := range []bool{false, true} {
		name := "direct-elimination"
		if shield {
			name = "shield-one-hit"
		}
		t.Run(name, func(t *testing.T) {
			m := refactorArena(t, func(s *bubbleparty.State) {
				s.SuddenAt = s.ServerAt + 180000
				s.Players[0].Shield = shield
				s.Bombs = []bubbleparty.Bomb{{ID: 1, Seat: 2, Team: 1, X: 6, Y: 5, Range: 1, At: s.ServerAt - 2000, ExplodeAt: s.ServerAt + 20}}
				s.BombID = 1
				s.Cells[5*bubbleparty.Cols+6] = bubbleparty.CellBomb
			})
			m = classicLegacy(t, m)
			at := refactorRead(t, m, 0).ServerAt
			m = refactorTick(t, m, at+100)
			p := refactorRead(t, m, 0).Players[0]
			if p.Alive != shield || p.TrappedUntil != 0 || p.InvulnerableUntil != 0 {
				t.Fatalf("legacy water gained classic trap/protection: shield=%v player=%+v", shield, p)
			}
			if shield {
				m = refactorTick(t, m, at+500)
				if !refactorRead(t, m, 0).Players[0].Alive {
					t.Fatal("same lingering blast consumed shield twice")
				}
				var s bubbleparty.State
				if err := json.Unmarshal(m.Data, &s); err != nil {
					t.Fatal(err)
				}
				s.Bombs = []bubbleparty.Bomb{{ID: 2, Seat: 2, Team: 1, X: 6, Y: 5, Range: 1, At: s.ServerAt - 2000, ExplodeAt: s.ServerAt + 20}}
				s.BombID = 2
				s.Cells[5*bubbleparty.Cols+6] = bubbleparty.CellBomb
				var err error
				m.Data, err = json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				m = refactorTick(t, m, at+600)
				if refactorRead(t, m, 0).Players[0].Alive {
					t.Fatal("second distinct legacy blast did not eliminate after shield was spent")
				}
			}
		})
	}
}

func TestLegacyIntegerMovementKeepsSavedDeadline(t *testing.T) {
	m := refactorArena(t, func(s *bubbleparty.State) {
		s.SuddenAt = s.ServerAt + 180000
		s.Players[0].NextMoveAt = s.ServerAt + 300
	})
	m = classicLegacy(t, m)
	at := refactorRead(t, m, 0).ServerAt
	before := classicState(t, m)
	restored, err := bubbleparty.Module().Restore(gameapi.Context{Now: time.UnixMilli(at)}, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"epoch", "serverAt", "startAt", "matchEndsAt", "suddenAt", "players"} {
		if string(classicState(t, restored)[key]) != string(before[key]) {
			t.Fatalf("legacy restore changed %s", key)
		}
	}
	m = refactorInput(t, restored, 0, "right", false)
	for _, checkpoint := range []struct {
		dt       int64
		x        int
		serverDt int64
	}{{20, 5, 0}, {100, 5, 100}, {300, 6, 300}, {400, 6, 400}, {500, 7, 500}} {
		m = refactorTick(t, m, at+checkpoint.dt)
		p := continuousRead(t, m, 0)
		if p.X != checkpoint.x || p.PX != float64(checkpoint.x) || p.PY != 5 || refactorRead(t, m, 0).ServerAt != at+checkpoint.serverDt {
			t.Fatalf("legacy step at %d ignored saved integer cadence: %+v serverAt=%d", checkpoint.dt, p, refactorRead(t, m, 0).ServerAt)
		}
	}
}
