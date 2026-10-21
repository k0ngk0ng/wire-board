package basketball_test

import (
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"reflect"
	"testing"
	"time"
)

// Failure inventory is specified in PLAYER_MODES.md before implementation.
func TestBasketballHTTPPlayerModes(t *testing.T) {
	for _, tc := range []struct {
		mode     string
		actors   []int
		controls [4]int8
	}{
		{"solo", []int{0}, [4]int8{0, -1, -1, -1}},
		{"cooperative", []int{0, 1}, [4]int8{0, 1, -1, -1}},
		{"versus", []int{0, 2}, [4]int8{0, -1, 2, -1}},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			h := gametest.NewHTTP(t, len(tc.actors), game.Module(), "basketball")
			h.AssertUnpublished()
			h.Start(map[string]any{"mode": tc.mode})
			basketballReadyHTTP(t, h)
			c := h.People[0]
			v := await(t, c, func(v game.View) bool { return v.Runtime.Playing != nil })
			if v.Runtime.Config.Match.ControllerAssignments != tc.controls || !reflect.DeepEqual(v.SeatActors, tc.actors) {
				t.Fatal("wrong roster", v.SeatActors, v.Runtime.Config.Match.ControllerAssignments)
			}
			if view(t, h.Spectator).LocalActor != -1 || view(t, h.Spectator).CanInput {
				t.Fatal("spectator assigned actor")
			}
			// Freeze through the original START input; restore must preserve this state.
			c.Action(buttons(v, 0, engine.ButtonStart), 200)
			v = await(t, c, func(v game.View) bool { return v.Phase == "paused" })
			c.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": v.Inputs[0].Sequence + 1}, 200)
			for seat, actor := range tc.actors {
				p := h.People[seat]
				v = view(t, p)
				if v.LocalActor != actor {
					t.Fatal("viewer actor", seat, v.LocalActor)
				}
				seq := v.Inputs[actor].Sequence + 1
				batch := map[string]any{"type": "inputs", "epoch": v.Epoch, "seat": seat, "basedOn": v.ServerAt, "sequence": seq, "events": []map[string]any{inputEvent(seq, engine.ButtonRight, 0, false)}}
				if actor != seat {
					bad := map[string]any{}
					for k, x := range batch {
						bad[k] = x
					}
					bad["seat"] = actor
					reject(t, p, bad)
				}
				p.Action(batch, 200)
				v = view(t, p)
				if v.Inputs[actor].Buttons != engine.ButtonRight || v.Inputs[actor].Sequence != seq {
					t.Fatal("input reached wrong actor", v.Inputs)
				}
				p.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": seq + 1}, 200)
			}
			// Autoplay the last real seat, keeping actor0 human when there are two.
			last := len(tc.actors) - 1
			actor := tc.actors[last]
			p := h.People[last]
			p.Command("autoplay", map[string]any{"enabled": true}, 200)
			v = await(t, c, func(v game.View) bool { return v.Inputs[actor].Automated })
			before := v.Runtime.VideoFrames
			p.Command("autoplay", map[string]any{"enabled": false}, 200)
			v = view(t, p)
			if v.Inputs[actor].Automated || v.Runtime.Config.Match.ControllerAssignments != tc.controls || v.Runtime.VideoFrames < before {
				t.Fatal("reclaim changed roster or reset runtime")
			}
			epoch := v.Epoch
			frames := v.Runtime.VideoFrames
			h.Restart()
			v = view(t, p)
			if v.Epoch != epoch || v.Runtime.VideoFrames < frames || v.LocalActor != actor || !reflect.DeepEqual(v.SeatActors, tc.actors) {
				t.Fatal("restore lost mode/identity")
			}
			h.Record("mode, authenticated actor mapping, handoff and SQLite restore verified")
			if len(tc.actors) == 2 {
				if v.Phase != "paused" {
					t.Fatal("pause did not survive restore")
				}
				c.Command("autoplay", map[string]any{"enabled": true}, 200)
				v = await(t, c, func(v game.View) bool { return v.Phase == "playing" })
				if !v.Inputs[0].Automated || v.Inputs[tc.actors[1]].Automated {
					t.Fatal("first-seat automation affected second human")
				}
				h.Record("original START proceeds when only the first seat is automated")
			}
		})
	}
}
func TestBasketballHTTPModeCapacity(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "basketball")
	c := h.People[0]
	c.Request("POST", "/api/admin/games/basketball", map[string]bool{"enabled": true}, 200)
	for _, bad := range []map[string]any{
		{"capacity": 1, "options": map[string]any{"mode": "versus"}},
		{"capacity": 2, "options": map[string]any{"mode": "solo"}},
		{"capacity": 2, "options": map[string]any{"mode": "unknown"}},
		{"capacity": 3}, {"capacity": 4},
	} {
		bad["kind"] = "basketball"
		bad["name"] = "无效人数"
		c.Request("POST", "/api/rooms", bad, 400)
	}
	r := c.Request("POST", "/api/rooms", map[string]any{"kind": "basketball", "name": "等待队友", "capacity": 2, "options": map[string]any{"mode": "cooperative"}}, 201)
	c.Command("ready", nil, 200)
	c.Command("start", nil, 400)
	if c.Room()["status"] != "waiting" {
		t.Fatal("underfilled room started")
	}
	h.People[1].Request("POST", "/api/rooms/"+r["id"].(string), map[string]any{"type": "join", "version": c.Room()["version"], "nonce": "mode-join-000001"}, 200)
	h.People[1].Command("ready", nil, 200)
	c.Command("start", nil, 200)
}

func TestBasketballHTTPVersusIdleAndManualControl(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "basketball")
	h.Start(map[string]any{"mode": "versus"})
	basketballReadyHTTP(t, h)
	c := h.People[1]
	v := view(t, c)
	epoch := v.Epoch
	for deadline := time.Now().Add(18 * time.Second); time.Now().Before(deadline); {
		time.Sleep(100 * time.Millisecond)
		v = view(t, c)
	}
	seat := c.Room()["seats"].([]any)[1].(map[string]any)
	if v.Inputs[2].Automated || seat["autoPlay"] == true || seat["timeoutAutoPlay"] == true {
		t.Fatal("idle second seat unexpectedly transferred to AI")
	}
	c.Command("autoplay", map[string]any{"enabled": true}, 200)
	v = await(t, c, func(v game.View) bool { return v.Inputs[2].Automated })
	frames := v.Runtime.VideoFrames
	c.Command("autoplay", map[string]any{"enabled": false}, 200)
	v = view(t, c)
	if v.Epoch != epoch || v.Runtime.VideoFrames < frames || v.Inputs[2].Automated || v.Runtime.Config.Match.ControllerAssignments[2] != 2 || !v.Inputs[1].Automated || !v.Inputs[3].Automated {
		t.Fatal("timeout reclaim changed character mapping")
	}
	h.Record("idle retains manual seat1; explicit autoplay uses actor2; reclaim preserves fixed computers")
}
