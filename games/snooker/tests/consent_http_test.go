package snooker_test

// Before independent-vote concurrency handling: the first confirmation makes
// every other player's already-prepared command stale, so teammates must click
// again. Conflicting decisions must still be rejected after the window changes.
import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"testing"
)

func TestSnookerDoublesConcurrentForfeitConsentHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 4, game.Module(), "snooker")
	h.Start(nil)
	d := gametest.Data(h.People[0].Room())
	first := h.People[0].ActionRequest(map[string]any{"type": "resign", "window": d["window"]})
	second := h.People[2].ActionRequest(map[string]any{"type": "resign", "window": d["window"]})
	id := h.People[0].Room()["id"].(string)
	h.People[0].Request("POST", "/api/rooms/"+id, first, 200)
	if h.People[0].Room()["status"] != "playing" {
		t.Fatal("one partner forfeited whole team")
	}
	h.People[2].Request("POST", "/api/rooms/"+id, second, 200)
	if h.People[0].Room()["status"] != "finished" {
		t.Fatal("simultaneous teammate confirmation rejected")
	}
	h.Record("both teammate confirmations accepted from same version")
}
func TestSnookerConcurrentNextFrameReadinessHTTP(t *testing.T) {
	mod := game.Module()
	start := mod.Start
	mod.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		m, e := start(ctx, raw)
		if e != nil {
			return m, e
		}
		var s game.State
		_ = json.Unmarshal(m.Data, &s)
		s.Phase = "aim"
		s.Turn = 0
		s.TurnIndex = 0
		s.Target = "clear"
		s.Clear = 7
		s.InHand = false
		for i := range s.Balls {
			s.Balls[i].Down = i != 0 && i != 21
		}
		s.Balls[0].Pos = game.Vec{X: game.TableWidth / 2, Y: .6}
		s.Balls[21].Pos = game.Vec{X: game.TableWidth / 2, Y: .2}
		s.Logs = append(s.Logs, "并发准备夹具：黑球结束第一局")
		m.Data, _ = json.Marshal(s)
		m.Actors = []int{0, 1, 2, 3}
		m.Attention = []int{0}
		return m, nil
	}
	h := gametest.NewHTTP(t, 4, mod, "snooker")
	h.Start(map[string]any{"bestOf": 3})
	d := gametest.Data(h.People[0].Room())
	h.People[0].Action(map[string]any{"type": "shoot", "angle": -1.5707963267948966, "power": .25, "window": d["window"]}, 200)
	room := awaitStopped(t, h)
	d = gametest.Data(room)
	if d["phase"] != "frame_end" {
		t.Fatal("fixture didn't finish frame")
	}
	requests := []map[string]any{}
	for _, p := range h.People {
		requests = append(requests, p.ActionRequest(map[string]any{"type": "ready", "window": d["window"]}))
	}
	id := room["id"].(string)
	for p, request := range requests {
		h.People[p].Request("POST", "/api/rooms/"+id, request, 200)
	}
	after := gametest.Data(h.People[0].Room())
	if after["frame"] != float64(2) || after["phase"] != "aim" {
		t.Fatal("same-version readiness stalled")
	}
	h.Record("four ready commands from one version started next frame")
}
