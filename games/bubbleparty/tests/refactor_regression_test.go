package bubbleparty_test

// Failure scenarios and expected behavior were recorded before this test in
// REFACTOR_SCENARIOS.md. All simulation advances through public module hooks.
import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type refactorPlayer struct {
	Seat              int   `json:"seat"`
	X                 int   `json:"x"`
	Y                 int   `json:"y"`
	Alive             bool  `json:"alive"`
	Seq               int64 `json:"seq"`
	TrappedUntil      int64 `json:"trappedUntil"`
	InvulnerableUntil int64 `json:"invulnerableUntil"`
	Eliminations      int   `json:"eliminations"`
}

type refactorView struct {
	Epoch    string             `json:"epoch"`
	ServerAt int64              `json:"serverAt"`
	StartAt  int64              `json:"startAt"`
	Players  []refactorPlayer   `json:"players"`
	Bombs    []bubbleparty.Bomb `json:"bombs"`
	Cells    json.RawMessage    `json:"cells"`
}

func refactorRead(t *testing.T, m gameapi.Match, seat int) refactorView {
	t.Helper()
	v, err := bubbleparty.Module().View(m, seat)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out refactorView
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func refactorTick(t *testing.T, m gameapi.Match, at int64) gameapi.Match {
	t.Helper()
	next, err := bubbleparty.Module().OnTimer(gameapi.Context{Now: time.UnixMilli(at)}, m)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func refactorStart(t *testing.T) gameapi.Match {
	t.Helper()
	m, err := bubbleparty.Module().Start(gameapi.Context{Players: 4, Now: time.UnixMilli(1_700_000_000_000)}, json.RawMessage(`{"map":"community10","roundSeconds":300}`))
	if err != nil {
		t.Fatal(err)
	}
	return refactorTick(t, m, refactorRead(t, m, 0).StartAt)
}

// A fully open, bounded arena gives deterministic geometry while preserving
// production initialization, clock, teams and all public lifecycle transitions.
func refactorArena(t *testing.T, mutate func(*bubbleparty.State)) gameapi.Match {
	t.Helper()
	m := refactorStart(t)
	var s bubbleparty.State
	if err := json.Unmarshal(m.Data, &s); err != nil {
		t.Fatal(err)
	}
	for y := 0; y < bubbleparty.Rows; y++ {
		for x := 0; x < bubbleparty.Cols; x++ {
			c := byte(bubbleparty.CellEmpty)
			if x == 0 || y == 0 || x == bubbleparty.Cols-1 || y == bubbleparty.Rows-1 {
				c = bubbleparty.CellWall
			}
			s.Cells[y*bubbleparty.Cols+x] = c
		}
	}
	for i, pos := range [][2]int{{5, 5}, {4, 5}, {10, 9}, {11, 9}} {
		s.Players[i].X, s.Players[i].Y = pos[0], pos[1]
	}
	if mutate != nil {
		mutate(&s)
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	m.Data = raw
	m, err = bubbleparty.Module().Restore(gameapi.Context{Now: time.UnixMilli(s.ServerAt)}, m)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func refactorInput(t *testing.T, m gameapi.Match, seat int, move string, bomb bool) gameapi.Match {
	t.Helper()
	v := refactorRead(t, m, seat)
	raw, err := json.Marshal(bubbleparty.Action{Type: "input", Epoch: v.Epoch, Seq: v.Players[seat].Seq + 1, Move: move, Bomb: bomb})
	if err != nil {
		t.Fatal(err)
	}
	next, err := bubbleparty.Module().Apply(gameapi.Context{Actor: seat, Players: 4, Now: time.UnixMilli(v.ServerAt)}, m, raw)
	if err != nil {
		t.Fatal(err)
	}
	return next
}

func TestRefactorMapIsNumericJSON(t *testing.T) {
	m := refactorStart(t)
	var previous []int
	for _, seat := range []int{0, -1} {
		var cells []int
		if err := json.Unmarshal(refactorRead(t, m, seat).Cells, &cells); err != nil {
			t.Fatalf("seat %d: cells must be a numeric JSON array, not Base64: %v", seat, err)
		}
		if len(cells) != bubbleparty.Cols*bubbleparty.Rows {
			t.Fatalf("cell count = %d", len(cells))
		}
		walls, crates := 0, 0
		for _, c := range cells {
			if c == bubbleparty.CellWall {
				walls++
			}
			if c == bubbleparty.CellCrate {
				crates++
			}
		}
		if walls == 0 || crates == 0 {
			t.Fatalf("real map geometry absent: walls=%d crates=%d", walls, crates)
		}
		if previous != nil && !reflect.DeepEqual(cells, previous) {
			t.Fatal("spectator and player map differ")
		}
		previous = cells
	}
}

func TestRefactorBombBlocksAfterOwnerLeaves(t *testing.T) {
	m := refactorArena(t, nil)
	at := refactorRead(t, m, 0).ServerAt
	m = refactorInput(t, m, 0, "right", true)
	m = refactorTick(t, m, at+100)
	v := refactorRead(t, m, 0)
	if v.Players[0].X != 6 || len(v.Bombs) != 1 {
		t.Fatalf("placement/escape failed: %+v", v)
	}
	m = refactorInput(t, m, 0, "left", false)
	m = refactorInput(t, m, 1, "right", false)
	m = refactorTick(t, m, at+400)
	v = refactorRead(t, m, 0)
	if v.Players[0].X != 6 || v.Players[1].X != 4 {
		t.Fatalf("live bomb became walkable after owner left: owner=%+v teammate=%+v", v.Players[0], v.Players[1])
	}
	if len(v.Bombs) != 1 {
		t.Fatal("live bomb disappeared before fuse")
	}
	var cells []int
	if err := json.Unmarshal(v.Cells, &cells); err != nil {
		t.Fatal(err)
	}
	if cells[5*bubbleparty.Cols+5] != bubbleparty.CellBomb {
		t.Fatal("bomb obstacle missing from projected map")
	}
}

func TestRefactorBombChainAfterOwnerLeaves(t *testing.T) {
	m := refactorArena(t, func(s *bubbleparty.State) {
		s.Bombs = []bubbleparty.Bomb{{ID: 1, Seat: 2, Team: 1, X: 5, Y: 7, Range: 2, At: s.ServerAt, ExplodeAt: s.ServerAt + 500}}
		s.BombID = 1
		s.Cells[7*bubbleparty.Cols+5] = bubbleparty.CellBomb
	})
	at := refactorRead(t, m, 0).ServerAt
	m = refactorInput(t, m, 0, "right", true)
	m = refactorTick(t, m, at+600)
	if bombs := refactorRead(t, m, 0).Bombs; len(bombs) != 0 {
		t.Fatalf("departed owner's bomb did not chain before its fuse: %+v", bombs)
	}
}

func TestRefactorStandingDoesNotEnableTakeover(t *testing.T) {
	for _, reclaim := range []bool{false, true} {
		t.Run(map[bool]string{false: "start", true: "reclaim"}[reclaim], func(t *testing.T) {
			m := refactorArena(t, nil)
			before := refactorRead(t, m, 0)
			if reclaim {
				var err error
				m, err = bubbleparty.Module().Reclaim(gameapi.Context{Actor: 0, Players: 4, Now: time.UnixMilli(before.ServerAt)}, m)
				if err != nil {
					t.Fatal(err)
				}
			}
			for offset := int64(100); offset <= 30000; offset += 100 {
				m = refactorTick(t, m, before.ServerAt+offset)
				if m.Deadline != 0 && m.Deadline <= before.ServerAt+offset && len(m.TimeoutActors) != 0 {
					t.Fatalf("normal standing became overdue for automatic takeover after %d ms: deadline=%d actors=%v", offset, m.Deadline, m.TimeoutActors)
				}
			}
			after := refactorRead(t, m, 0)
			if after.Players[0].X != before.Players[0].X || after.Players[0].Y != before.Players[0].Y || len(after.Bombs) != 0 {
				t.Fatal("standing generated movement or bombs")
			}
		})
	}
}

func refactorHit(t *testing.T) gameapi.Match {
	t.Helper()
	m := refactorArena(t, func(s *bubbleparty.State) {
		s.Bombs = []bubbleparty.Bomb{{ID: 1, Seat: 2, Team: 1, X: 6, Y: 5, Range: 1, At: s.ServerAt - 2000, ExplodeAt: s.ServerAt + 20}}
		s.Cells[5*bubbleparty.Cols+6] = bubbleparty.CellBomb
		s.BombID = 1
	})
	m = refactorTick(t, m, refactorRead(t, m, 0).ServerAt+100)
	v := refactorRead(t, m, 0)
	if !v.Players[0].Alive || v.Players[0].TrappedUntil <= v.ServerAt {
		t.Fatalf("water hit must trap before elimination: %+v", v.Players[0])
	}
	if m.Result != nil {
		t.Fatal("trapping prematurely ended match")
	}
	return m
}

func TestRefactorActiveWaterHitsLateEntrant(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "expired"}[expired], func(t *testing.T) {
			m := refactorArena(t, func(s *bubbleparty.State) {
				s.Players[0].X, s.Players[0].Y = 5, 4
				s.Blasts = []bubbleparty.Blast{{X: 5, Y: 5, Seat: 2, At: s.ServerAt - 20, EndAt: s.ServerAt + 300}}
			})
			at := refactorRead(t, m, 0).ServerAt
			if expired {
				m = refactorTick(t, m, at+400)
			}
			m = refactorInput(t, m, 0, "down", false)
			m = refactorTick(t, m, refactorRead(t, m, 0).ServerAt+100)
			v := refactorRead(t, m, 0)
			p := v.Players[0]
			if !p.Alive || (p.TrappedUntil > v.ServerAt) == expired {
				t.Fatalf("expired=%v late entrant=%+v", expired, p)
			}
		})
	}
}

func TestRefactorTrappedPlayerCannotAct(t *testing.T) {
	m := refactorHit(t)
	before := refactorRead(t, m, 0)
	raw, _ := json.Marshal(bubbleparty.Action{Type: "input", Epoch: before.Epoch, Seq: before.Players[0].Seq + 1, Move: "up", Bomb: true})
	if next, err := bubbleparty.Module().Apply(gameapi.Context{Actor: 0, Now: time.UnixMilli(before.ServerAt)}, m, raw); err == nil {
		m = next
	}
	m = refactorTick(t, m, before.ServerAt+200)
	after := refactorRead(t, m, 0)
	if after.Players[0].X != before.Players[0].X || after.Players[0].Y != before.Players[0].Y || len(after.Bombs) != 0 {
		t.Fatal("trapped player moved or placed a bomb")
	}
}

func TestRefactorTeammateRescuesAfterRestore(t *testing.T) {
	m := refactorHit(t)
	before := refactorRead(t, m, 0)
	var err error
	m, err = bubbleparty.Module().Restore(gameapi.Context{Now: time.UnixMilli(before.ServerAt)}, m)
	if err != nil {
		t.Fatal(err)
	}
	if got := refactorRead(t, m, 0).Players[0].TrappedUntil; got != before.Players[0].TrappedUntil {
		t.Fatalf("restore changed rescue deadline: %d -> %d", before.Players[0].TrappedUntil, got)
	}
	m = refactorTick(t, m, before.ServerAt+500)
	m = refactorInput(t, m, 1, "right", false)
	m = refactorTick(t, m, refactorRead(t, m, 0).ServerAt+100)
	v := refactorRead(t, m, 0)
	if !v.Players[0].Alive || v.Players[0].TrappedUntil > v.ServerAt || v.Players[0].InvulnerableUntil <= v.ServerAt {
		t.Fatalf("touch rescue did not restore protected control: %+v", v.Players[0])
	}
	m = refactorInput(t, m, 0, "up", false)
	m = refactorTick(t, m, v.ServerAt+200)
	if refactorRead(t, m, 0).Players[0].Y >= 5 {
		t.Fatal("rescued player still cannot move")
	}
}

func TestRefactorEnemyTouchEliminatesTrappedPlayer(t *testing.T) {
	m := refactorHit(t)
	v := refactorRead(t, m, 0)
	// Move the enemy to the adjacent approach square via the saved-state fixture;
	// the actual collision and elimination still run through input and timer.
	var s bubbleparty.State
	if err := json.Unmarshal(m.Data, &s); err != nil {
		t.Fatal(err)
	}
	s.Players[2].X, s.Players[2].Y = 5, 4
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	m.Data = raw
	m = refactorTick(t, m, v.ServerAt+500)
	m = refactorInput(t, m, 2, "down", false)
	m = refactorTick(t, m, refactorRead(t, m, 0).ServerAt+100)
	v = refactorRead(t, m, 0)
	if v.Players[0].Alive || v.Players[2].Eliminations != 1 {
		t.Fatalf("enemy touch failed: victim=%+v enemy=%+v", v.Players[0], v.Players[2])
	}
	found := false
	for _, seat := range m.Eliminated {
		if seat == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("elimination absent from host envelope")
	}
}

func TestRefactorTrappedTimeoutEliminates(t *testing.T) {
	m := refactorHit(t)
	v := refactorRead(t, m, 0)
	deadline := v.Players[0].TrappedUntil
	if deadline-v.ServerAt < 500 || deadline-v.ServerAt > 15000 {
		t.Fatalf("rescue window not usable/bounded: %d ms", deadline-v.ServerAt)
	}
	m = refactorTick(t, m, deadline-100)
	if !refactorRead(t, m, 0).Players[0].Alive {
		t.Fatal("trapped player eliminated before deadline")
	}
	m = refactorTick(t, m, deadline+100)
	if refactorRead(t, m, 0).Players[0].Alive {
		t.Fatal("trapped player survived deadline")
	}
}
