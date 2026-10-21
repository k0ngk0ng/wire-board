package basketball_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// Full host boundary failures: an otherwise complete engine match stalls on
// real system deadlines, handoff/restart changes progress, the host misses the
// original intermission/result, or internal AI accidentally rates human seats.
// The test uses normal wall clocks, real authentication and SQLite; no world
// fixture, predetermined result, seed replacement, or shortened match clock.
func TestBasketballHTTPCompletePreparedMatch(t *testing.T) {
	if os.Getenv("BASKETBALL_FULL_HTTP") != "1" {
		t.Skip("set BASKETBALL_FULL_HTTP=1 for normal-clock full HTTP match")
	}
	h := gametest.NewHTTP(t, 2, observedModule(t), "basketball")
	a, b := h.People[0], h.People[1]
	began := time.Now()
	h.Start(map[string]any{"mode": "versus", "court": "court-3", "characters": []int{1, 0, 1, 0}})
	basketballReadyHTTP(t, h)
	v := await(t, a, func(v game.View) bool { return v.Runtime.Playing != nil })
	cold := v.Runtime.Config
	epoch := v.Epoch
	stages := []any{}
	record := func(label string, v game.View) {
		stages = append(stages, map[string]any{"label": label, "elapsedSeconds": time.Since(began).Seconds(), "videoFrames": v.Runtime.VideoFrames, "logicSteps": v.Runtime.LogicSteps, "phase": v.Phase, "scores": v.Scores, "controls": v.Runtime.Config.Match.ControllerAssignments})
	}
	t.Cleanup(func() {
		dir := filepath.Join("..", "..", "..", ".local", "basketball", "full-http")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "scope": "normal-clock complete authoritative HTTP match with two authenticated humans, original AI takeover, reclaim and SQLite restart; not a complete multiplayer browser test", "configuration": cold, "epoch": epoch, "elapsedSeconds": time.Since(began).Seconds(), "stages": stages}, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, "result.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	// Both authenticated seats supply a genuine accepted gesture before handing
	// their characters to original AI; implicit computer teammates stay unchanged.
	for seat, c := range h.People {
		v = view(t, c)
		actor := v.SeatActors[seat]
		c.Action(buttons(v, actor, engine.ButtonRight), 200)
		time.Sleep(120 * time.Millisecond)
		v = view(t, c)
		c.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": v.Inputs[actor].Sequence + 1}, 200)
		c.Command("autoplay", map[string]any{"enabled": true}, 200)
	}
	v = await(t, a, func(v game.View) bool { return v.Inputs[0].Automated && v.Inputs[2].Automated })
	record("both humans transfer to original AI", v)
	// Reclaim the opposing team through the platform, not an engine state edit.
	b.Command("autoplay", map[string]any{"enabled": false}, 200)
	v = view(t, b)
	if v.LocalActor != 2 || v.Inputs[2].Automated || !v.Inputs[0].Automated {
		t.Fatal("incorrect opposing-seat reclaim")
	}
	b.Action(buttons(v, 2, engine.ButtonLeft), 200)
	v = view(t, b)
	b.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": v.Inputs[2].Sequence + 1}, 200)
	b.Command("autoplay", map[string]any{"enabled": true}, 200)
	// Pause using the original first controller, then restore the actual database.
	a.Command("autoplay", map[string]any{"enabled": false}, 200)
	v = view(t, a)
	a.Action(buttons(v, 0, engine.ButtonStart), 200)
	v = await(t, a, func(v game.View) bool { return v.Phase == "paused" })
	a.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": v.Inputs[0].Sequence + 1}, 200)
	before := view(t, a)
	h.Restart()
	v = view(t, a)
	if v.Epoch != epoch || v.Phase != "paused" || v.Runtime.VideoFrames < before.Runtime.VideoFrames || v.Scores != before.Scores {
		t.Fatal("restart lost match")
	}
	record("real SQLite restart preserves paused game", v)
	a.Command("autoplay", map[string]any{"enabled": true}, 200)
	v = await(t, a, func(v game.View) bool { return v.Phase == "playing" && v.Inputs[0].Automated && v.Inputs[2].Automated })
	record("first AI controller resumes original match", v)
	second := false
	for deadline := began.Add(8 * time.Minute); time.Now().Before(deadline) && !t.Failed(); {
		v = view(t, a)
		if v.Epoch != epoch {
			t.Fatal("match replaced during real clock")
		}
		if v.Runtime.Playing != nil && v.Runtime.Playing.Match.Period == 1 && !second {
			second = true
			record("natural second period", v)
		}
		if v.Runtime.Outcome != nil && v.Runtime.Outcome.Confirmed {
			if !second || v.Runtime.VideoFrames < 21600 || time.Since(began) < 5*time.Minute {
				t.Fatal("normal regulation clock was bypassed")
			}
			room := a.Room()
			g := room["game"].(map[string]any)
			if room["status"] != "finished" || g["finished"] != true || len(g["winners"].([]any)) != 1 {
				t.Fatal("host did not commit result", room["status"], g["winners"])
			}
			record("host commits original confirmed result", v)
			h.Record("complete prepared match with real clocks and SQLite recovery")
			t.Logf("complete HTTP match: videos=%d logic=%d scores=%v elapsed=%s", v.Runtime.VideoFrames, v.Runtime.LogicSteps, v.Scores, time.Since(began))
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	record("last observed state", v)
	t.Fatal("normal-clock HTTP match did not finish within eight minutes")
}
