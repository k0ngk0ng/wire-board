package tetris_test

// Failure inventory before implementation: a legal J top kick produces y=-1
// and breaks views, subsequent input, timers, bots or SQLite restart; a legal
// horizontal I at y=-2 is rejected; accepting negative origins also accepts
// occupied cells above/below/outside the board or overlapping settled cells.
import (
	"encoding/json"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/tetris/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestTetrisActivePieceCellBounds(t *testing.T) {
	mod, _, m := started(t, 1, "survival")
	for _, tc := range []struct {
		name      string
		kind      game.Kind
		rot, x, y int
		blocked   bool
		valid     bool
	}{
		{"J negative origin", game.J, 2, 2, -1, false, true},
		{"I negative origin", game.I, 2, 3, -2, false, true},
		{"I left origin", game.I, 1, -2, 1, false, true},
		{"above top", game.J, 2, 2, -2, false, false},
		{"below bottom", game.O, 0, 4, game.BoardRows - 1, false, false},
		{"outside left", game.J, 2, -1, -1, false, false},
		{"outside right", game.J, 2, 8, -1, false, false},
		{"settled collision", game.J, 2, 2, -1, true, false},
		{"invalid kind", game.Empty, 0, 3, 1, false, false},
		{"invalid rotation", game.J, 4, 3, 1, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s game.State
			if err := json.Unmarshal(m.Data, &s); err != nil {
				t.Fatal(err)
			}
			s.Players[0].Active = &game.Piece{ID: 1, Kind: tc.kind, Rot: tc.rot, X: tc.x, Y: tc.y}
			if tc.blocked {
				s.Players[0].Board[0] = "0010000000"
			}
			fixture := m
			fixture.Data = raw(s)
			_, err := mod.View(fixture, 0)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}

func TestTetrisHTTPTopKickAndSQLiteRestart(t *testing.T) {
	mod := game.Module()
	start := mod.Start
	mod.Start = func(ctx gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
		m, err := start(ctx, options)
		if err != nil {
			return m, err
		}
		// Explicit synthetic opening only. Rotation, HTTP input, timers, views,
		// persistence and restart all use the production implementation.
		var s game.State
		if err = json.Unmarshal(m.Data, &s); err != nil {
			return m, err
		}
		now := ctx.Now.UnixMilli()
		s.Phase, s.StartAt, s.ServerAt = "playing", now, now
		s.MatchEndsAt = now + game.MatchCapMS
		p := &s.Players[0]
		p.Active = &game.Piece{ID: 1, Kind: game.J, Rot: 1, X: 2, Y: 1, SpawnedAt: now}
		p.PieceID, p.FallAt, p.IdleAt = 1, now+800, now+game.IdleMS
		p.Board[3] = "0000550000"
		m.Data, m.CheckpointKey = raw(s), "playing"
		m.Actors, m.TimeoutActors = []int{0}, []int{0}
		m.Deadline, m.SystemDeadline = p.IdleAt, now+game.FrameMS
		return m, nil
	}
	h := gametest.NewHTTP(t, 1, mod, "tetris")
	h.Start(map[string]any{"mode": "survival"})
	c := h.People[0]
	before := tetrisData(t, c)
	epoch, end := before["epoch"], before["matchEndsAt"]
	c.Action(tetrisBatch(epoch.(string), 1, "rotateCW"), 200)
	after := tetrisData(t, c)
	active := tetrisPlayer(t, after, 0)["active"].(map[string]any)
	if active["y"] != float64(-1) || active["rot"] != float64(2) {
		t.Fatalf("top kick not exercised: %v", active)
	}
	h.Record("legal top kick has negative origin and remains readable")
	h.Restart()
	after = tetrisData(t, c)
	if after["epoch"] != epoch || after["matchEndsAt"] != end {
		t.Fatal("restart changed match identity or deadline")
	}
	clock := after["serverAt"].(float64)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && tetrisData(t, c)["serverAt"].(float64) <= clock {
		time.Sleep(25 * time.Millisecond)
	}
	if tetrisData(t, c)["serverAt"].(float64) <= clock {
		t.Fatal("timer stalled after top kick and restart")
	}
	c.Action(tetrisBatch(epoch.(string), 2, "left", "left", "softDrop", "hardDrop"), 200)
	if tetrisPlayer(t, tetrisData(t, c), 0)["score"].(float64) <= 0 {
		t.Fatal("subsequent drop did not score")
	}
	c.Command("autoplay", map[string]any{"enabled": true}, 200)
	c.Command("autoplay", map[string]any{"enabled": false}, 200)
	h.Record("SQLite restart, advancing timer, drop and manual reclaim succeed")
}
