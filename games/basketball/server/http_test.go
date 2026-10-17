package basketball_test

// Real-host acceptance is specified before the game adapter. Scope and failure
// scenarios are in PLATFORM_CONTRACT.md; this uses no private server mutation.
import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// Observe errors without altering inputs, time, results or retry behavior.
// A temporarily failing frame cannot become a green test merely because a
// later timeout changes the AI path and happens to move past the missing rule.
func observedModule(t *testing.T) gameapi.Module {
	t.Helper()
	m := game.Module()
	// Four-seat development fixture exercises every original controller; the public module opens only 1/2 seats.
	m.MaxPlayers = 4
	timer := m.OnTimer
	var once sync.Once
	root, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		next := filepath.Dir(root)
		if next == root {
			t.Fatal("workspace root missing")
		}
		root = next
	}
	m.OnTimer = func(ctx gameapi.Context, before gameapi.Match) (gameapi.Match, error) {
		next, err := timer(ctx, before)
		if err != nil {
			once.Do(func() {
				dir := filepath.Join(root, ".local", "basketball", "server-http")
				if e := os.MkdirAll(dir, 0755); e != nil {
					t.Error(e)
				} else {
					record := map[string]any{"test": t.Name(), "error": err.Error(), "context": ctx, "before": before}
					var boundary *game.RuntimeAdvanceError
					if errors.As(err, &boundary) {
						record["runtimeBefore"] = boundary.Before
						record["held"] = boundary.Held
					}
					data, _ := json.MarshalIndent(record, "", "  ")
					if e := os.WriteFile(filepath.Join(dir, t.Name()+"-timer-failure.json"), data, 0600); e != nil {
						t.Error(e)
					}
				}
				t.Errorf("production basketball timer failed: %v", err)
			})
		}
		return next, err
	}
	return m
}

func view(t *testing.T, c *gametest.Client) game.View {
	t.Helper()
	data, err := json.Marshal(gametest.Data(c.Room()))
	if err != nil {
		t.Fatal(err)
	}
	var v game.View
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func await(t *testing.T, c *gametest.Client, check func(game.View) bool) game.View {
	t.Helper()
	var v game.View
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); {
		v = view(t, c)
		if check(v) {
			return v
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatalf("basketball state did not reach expected condition: frame=%d phase=%s", v.Runtime.VideoFrames, v.Phase)
	return v
}

func evidence(t *testing.T) func(string, game.View) {
	t.Helper()
	trace := []any{}
	t.Cleanup(func() {
		dir, _ := os.Getwd()
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				break
			}
			next := filepath.Dir(dir)
			if next == dir {
				t.Error("workspace root missing")
				return
			}
			dir = next
		}
		dir = filepath.Join(dir, ".local", "basketball", "server-http")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Error(err)
			return
		}
		data, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "real HTTP authentication, host timers and SQLite; not full-match HTTP or frontend acceptance", "trace": trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, t.Name()+".json"), data, 0600); err != nil {
			t.Error(err)
		}
	})
	return func(label string, v game.View) { trace = append(trace, map[string]any{"label": label, "view": v}) }
}

func buttons(v game.View, seat int, held engine.Buttons) map[string]any {
	return map[string]any{"type": "buttons", "epoch": v.Epoch, "sequence": v.Inputs[seat].Sequence + 1, "buttons": held}
}

func reject(t *testing.T, c *gametest.Client, action map[string]any) {
	t.Helper()
	r := c.Room()
	data, _ := json.Marshal(c.ActionRequest(action))
	req, err := http.NewRequest("POST", c.Base+"/api/rooms/"+r["id"].(string), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 400 && response.StatusCode != 409 {
		t.Fatal("invalid basketball input accepted", response.StatusCode)
	}
}

func assertPausedContinuation(t *testing.T, before, after engine.RuntimeState) {
	t.Helper()
	if after.VideoFrames < before.VideoFrames || after.VideoFrames-before.VideoFrames > 1200 {
		t.Fatal("paused snapshot counter was reset or jumped")
	}
	// Original pause still consumes main-loop RNG and projects frozen actors.
	// Align by actual video ticks; do not erase those fields from comparison.
	expected := before
	for expected.VideoFrames < after.VideoFrames {
		step, err := engine.StepVideo(expected, [4]engine.Buttons{})
		if err != nil {
			t.Fatal(err)
		}
		expected = step.State
	}
	want, _ := json.Marshal(expected.Playing.World)
	got, _ := json.Marshal(after.Playing.World)
	if string(want) != string(got) || expected.Playing.Match != after.Playing.Match {
		t.Fatal("paused handoff/restore diverged from original runtime continuation")
	}
}

func TestBasketballHTTPPublicationAndIndependentInputs(t *testing.T) {
	h := gametest.NewHTTP(t, 4, observedModule(t), "basketball")
	record := evidence(t)
	h.AssertUnpublished()
	h.Start(map[string]any{})
	a := h.People[0]
	v := await(t, a, func(v game.View) bool { return v.Runtime.Playing != nil })
	record("original opening completed", v)
	if v.Epoch == "" || v.Options.Court != "new-york" || !v.CanInput || len(v.Runtime.Config.Match.ControllerAssignments) != 4 {
		t.Fatal("invalid initial host projection")
	}
	if view(t, h.Spectator).CanInput {
		t.Fatal("spectator can control basketball")
	}
	reject(t, h.Spectator, buttons(v, 0, engine.ButtonRight))
	room := a.Room()
	requests := make([]map[string]any, 4)
	for i, c := range h.People {
		requests[i] = c.ActionRequest(buttons(v, i, engine.ButtonRight))
		requests[i]["version"] = room["version"]
	}
	for i, c := range h.People {
		c.Request("POST", "/api/rooms/"+room["id"].(string), requests[i], 200)
	}
	v = view(t, a)
	for i := range v.Inputs {
		if v.Inputs[i].Sequence != 1 || v.Inputs[i].Buttons != engine.ButtonRight {
			t.Fatal("one seat invalidated another seat's input", i, v.Inputs)
		}
	}
	x := v.Runtime.Playing.World.Preparation.Actions.Actors[0].Player.Body.Position.X
	v = await(t, a, func(v game.View) bool {
		return v.Runtime.Playing.World.Preparation.Actions.Actors[0].Player.Body.Position.X > x
	})
	record("authoritative original movement", v)
	bad := buttons(v, 0, 0)
	bad["x"] = 99999
	reject(t, a, bad)
	bad = buttons(v, 0, 0)
	bad["epoch"] = "old-match"
	reject(t, a, bad)
	bad = buttons(v, 0, 0)
	bad["sequence"] = v.Inputs[0].Sequence
	reject(t, a, bad)
	v = await(t, a, func(v game.View) bool { return v.Inputs[0].Buttons == 0 })
	record("missing transport input releases held buttons", v)
}

func TestBasketballHTTPHandoffAndRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 4, observedModule(t), "basketball")
	record := evidence(t)
	h.Start(map[string]any{})
	a := h.People[0]
	v := await(t, a, func(v game.View) bool { return v.Runtime.Playing != nil })
	// Freeze original play through Start, so persistence can be compared while
	// the host clock still advances. This is a real player action, not a fixture.
	a.Action(buttons(v, 0, engine.ButtonStart), 200)
	v = await(t, a, func(v game.View) bool { return v.Runtime.Playing.Match.Phase == engine.MatchPaused })
	a.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": v.Inputs[0].Sequence + 1}, 200)
	h.People[1].Command("autoplay", map[string]any{"enabled": true}, 200)
	v = await(t, a, func(v game.View) bool { return v.Inputs[1].Automated })
	if v.Runtime.Playing.Context.Actions.Controls[1] != 128 {
		t.Fatal("platform autoplay did not select original AI")
	}
	beforeRuntime := v.Runtime
	otherIdle := v.Inputs[2].IdleAt
	h.People[1].Command("autoplay", map[string]any{"enabled": false}, 200)
	v = view(t, a)
	if v.Inputs[1].Automated || v.Runtime.Playing.Context.Actions.Controls[1] != 1 || v.Inputs[2].IdleAt != otherIdle {
		t.Fatal("manual reclaim altered another seat or retained AI")
	}
	assertPausedContinuation(t, beforeRuntime, v.Runtime)
	record("manual control restored without gameplay reset", v)
	epoch, scores := v.Epoch, v.Scores
	beforeRuntime = v.Runtime
	h.Restart()
	v = view(t, a)
	if v.Epoch != epoch || v.Scores != scores || v.Runtime.Playing.Match.Phase != engine.MatchPaused {
		t.Fatal("SQLite restore replaced the match")
	}
	assertPausedContinuation(t, beforeRuntime, v.Runtime)
	record("restored through real server restart", v)
}

func TestBasketballHTTPComputersAndTimeoutAutoplay(t *testing.T) {
	h := gametest.NewHTTP(t, 1, observedModule(t), "basketball")
	record := evidence(t)
	c := h.People[0]
	h.AssertUnpublished()
	c.Request("POST", "/api/admin/games/basketball", map[string]bool{"enabled": true}, 200)
	r := c.Request("POST", "/api/rooms", map[string]any{"kind": "basketball", "name": "篮球电脑与超时", "capacity": 4}, 201)
	for i := 0; i < 3; i++ {
		c.Command("add_bot", nil, 200)
	}
	c.Command("ready", nil, 200)
	c.Command("start", nil, 200)
	h.Spectator.Request("POST", "/api/rooms/"+r["id"].(string)+"/watch", map[string]any{}, 200)
	v := await(t, c, func(v game.View) bool { return v.Runtime.Playing != nil })
	if v.Inputs[0].Automated || v.Runtime.Config.Match.ControllerAssignments != ([4]int8{0, -1, -1, -1}) {
		t.Fatal("original computer seats were not selected")
	}
	record("one human and three platform computers", v)
	epoch := v.Epoch
	frames := v.Runtime.VideoFrames
	for deadline := time.Now().Add(18 * time.Second); time.Now().Before(deadline) && !v.Inputs[0].Automated && !t.Failed(); {
		time.Sleep(100 * time.Millisecond)
		v = view(t, c)
	}
	record("latest state at timeout verification", v)
	if !v.Inputs[0].Automated || v.Runtime.Config.Match.ControllerAssignments != ([4]int8{-1, -1, -1, -1}) || v.Runtime.VideoFrames <= frames {
		t.Fatal("real host timeout did not transfer the human seat to original AI")
	}
	seat := c.Room()["seats"].([]any)[0].(map[string]any)
	if seat["autoPlay"] != true || seat["timeoutAutoPlay"] != true {
		t.Fatal("game bypassed platform autoplay metadata", seat)
	}
	record("host deadline selected original AI", v)
	frames = v.Runtime.VideoFrames
	c.Command("autoplay", map[string]any{"enabled": false}, 200)
	v = view(t, c)
	if v.Epoch != epoch || v.Inputs[0].Automated || v.Runtime.Config.Match.ControllerAssignments[0] != 0 || v.Runtime.VideoFrames < frames {
		t.Fatal("manual takeover restarted or failed to reclaim the existing match")
	}
	for i := 1; i < 4; i++ {
		if !v.Inputs[i].Automated || v.Runtime.Config.Match.ControllerAssignments[i] != -1 {
			t.Fatal("manual takeover changed another computer")
		}
	}
	c.Action(buttons(v, 0, engine.ButtonRight), 200)
	record("human reclaimed only its original seat", view(t, c))
}

// Reliable batches cross the actual authentication and stale-version boundary.
// HTTP may arrive out of order; only accepted per-seat sequences order input.
func TestBasketballHTTPReliableInputBatches(t *testing.T) {
	h := gametest.NewHTTP(t, 4, observedModule(t), "basketball")
	record := evidence(t)
	h.Start(map[string]any{})
	a := h.People[0]
	v := await(t, a, func(v game.View) bool { return v.Runtime.Playing != nil })
	batch := func(v game.View, seat int, seq int64, events ...map[string]any) map[string]any {
		return map[string]any{"type": "inputs", "epoch": v.Epoch, "seat": seat, "basedOn": v.ServerAt, "sequence": seq, "events": events}
	}
	oldRoom := a.Room()
	down := batch(v, 0, 1, inputEvent(1, engine.ButtonB, 0, false))
	reject(t, h.Spectator, down)
	reject(t, h.People[1], down)
	a.Action(down, 200)
	// Release overtakes the first request. The subsequent gesture includes the
	// unacknowledged release and arrives before the separate release retry.
	fresh := batch(v, 0, 3, inputEvent(2, 0, 0, true), inputEvent(3, engine.ButtonRight, 20, false))
	req := a.ActionRequest(fresh)
	req["version"] = oldRoom["version"]
	a.Request("POST", "/api/rooms/"+oldRoom["id"].(string), req, 200)
	before := view(t, a).Inputs[0]
	a.Action(batch(v, 0, 2, inputEvent(2, 0, 0, true)), 200)
	a.Action(down, 200)
	after := view(t, a).Inputs[0]
	if before.Sequence != 3 || after.Sequence != 3 || after.Buttons != engine.ButtonRight || before.LastPacketAt != after.LastPacketAt || before.IdleAt != after.IdleAt {
		t.Fatal("stale HTTP retry changed the accepted gesture", before, after)
	}
	record("delayed cancellation and old press cannot overwrite newer direction", view(t, a))
	// Another authenticated seat retains its own sequence stream.
	v = view(t, h.People[1])
	h.People[1].Action(batch(v, 1, 1, inputEvent(1, engine.ButtonLeft, 0, false)), 200)
	a.Action(batch(v, 0, 4, inputEvent(4, 0, 0, true)), 200)
	v = view(t, a)
	if v.Inputs[0].Buttons != 0 || v.Inputs[0].Pending != 0 || v.Runtime.PendingPressed[0] != 0 || v.Inputs[1].Buttons != engine.ButtonLeft || v.Inputs[1].Sequence != 1 {
		t.Fatal("cancellation leaked into another authenticated seat", v.Inputs)
	}
	record("cancellation is isolated across real authenticated seats", v)
}
