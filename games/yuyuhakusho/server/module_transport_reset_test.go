package yuyuhakusho

import (
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func TestModuleTransportResetReleasesAtNextTick(t *testing.T) {
	for _, kind := range []string{"restore", "reclaim"} {
		for _, queued := range []bool{false, true} {
			name := kind
			if queued {
				name += "-with-queued-input"
			}
			t.Run(name, func(t *testing.T) {
				mod, ctx, match := moduleStartTest(t, 2, Options{Characters: []uint8{3, 9}}, []bool{false, false})
				epoch := moduleViewTest(t, mod, match, 0).Epoch
				for seat := 0; seat < 2; seat++ {
					ctx.Actor = seat
					match = moduleActionTest(t, mod, ctx, match, map[string]any{"type": "loaded", "epoch": epoch})
				}
				held := engine.ButtonRight | engine.ButtonX
				peerHeld := engine.ButtonLeft
				ctx.Actor = 0
				match = moduleActionTest(t, mod, ctx, match, map[string]any{"type": "input", "epoch": epoch, "sequence": 1, "buttons": held})
				ctx.Actor = 1
				match = moduleActionTest(t, mod, ctx, match, map[string]any{"type": "input", "epoch": epoch, "sequence": 1, "buttons": peerHeld})
				match = moduleTickTest(t, mod, ctx, match, 1017)
				before := moduleCheckpointTest(t, mod, match, 0)
				if before.World.Controllers[0].Current != held || before.World.Controllers[1].Current != peerHeld {
					t.Fatal("fixture did not apply held inputs")
				}
				sequence := 1
				ctx.Actor = 0
				ctx.Now = time.UnixMilli(1017)
				if queued {
					sequence++
					match = moduleActionTest(t, mod, ctx, match, map[string]any{"type": "input", "epoch": epoch, "sequence": sequence, "buttons": engine.ButtonDown | engine.ButtonZ})
				}
				var err error
				if kind == "restore" {
					ctx.Actor = -1
					match, err = mod.Restore(ctx, match)
				} else {
					match, err = mod.Reclaim(ctx, match)
				}
				if err != nil {
					t.Fatal(err)
				}
				after := moduleCheckpointTest(t, mod, match, 0)
				if before.Frame != after.Frame || !reflect.DeepEqual(before.World, after.World) || !reflect.DeepEqual(before.Presentation, after.Presentation) {
					t.Fatal("transport reset mutated canonical state outside a tick")
				}
				match = moduleTickTest(t, mod, ctx, match, 1034)
				after = moduleCheckpointTest(t, mod, match, 0)
				controller := after.World.Controllers[0]
				if controller.Current != 0 || controller.Released&held != held {
					t.Fatal("transport reset did not release through the next recorded tick", controller)
				}
				peerExpected := peerHeld
				if kind == "restore" {
					peerExpected = 0
				}
				if after.World.Controllers[1].Current != peerExpected {
					t.Fatal("transport reset changed another seat's control scope")
				}
				for _, player := range moduleViewTest(t, mod, match, 0).Players {
					if player.Computer {
						t.Fatal("transport reset automatically enabled AI")
					}
				}
				ctx.Actor = 0
				ctx.Now = time.UnixMilli(1034)
				match = moduleActionTest(t, mod, ctx, match, map[string]any{"type": "input", "epoch": epoch, "sequence": sequence + 1, "buttons": engine.ButtonY})
				match = moduleTickTest(t, mod, ctx, match, 1051)
				if got := moduleCheckpointTest(t, mod, match, 0).World.Controllers[0].Current; got != engine.ButtonY {
					t.Fatal("new input was lost or stale queue replayed", got)
				}
			})
		}
	}
}

func TestModulePhaseTracksOriginalManagerStage(t *testing.T) {
	mod, ctx, match := moduleStartTest(t, 2, Options{Characters: []uint8{3, 9}}, []bool{true, true})
	for tick := 1; tick <= 42; tick++ {
		match = moduleTickTest(t, mod, ctx, match, int64(1000+tick*100))
	}
	checkpoint := moduleCheckpointTest(t, mod, match, 0)
	manager := checkpoint.World.Objects[0x8040]
	if checkpoint.Frame < 240 || manager.Header.Stage != 10 {
		t.Fatal("fixture must reach original opening after frame 240", checkpoint.Frame, manager.Header.Stage)
	}
	if got := moduleViewTest(t, mod, match, 0).Phase; got != "opening" {
		t.Fatal("phase ignored original opening manager", got, checkpoint.Frame, manager.Header.Stage)
	}
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []uint16{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26} {
		manager.Header.Stage = stage
		state.World.Objects[0x8040] = manager
		expected := "playing"
		if stage <= 12 {
			expected = "opening"
		}
		if got := state.phase(); got != expected {
			t.Fatal("manager phase differs", stage, got, expected)
		}
	}
	state.World.Clock.PauseFlags |= 128
	if got := state.phase(); got != "paused" {
		t.Fatal("pause did not take precedence", got)
	}
	state.World.MatchComplete = true
	if got := state.phase(); got != "result" {
		t.Fatal("completed result did not take precedence", got)
	}
}
