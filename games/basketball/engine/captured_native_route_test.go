package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Cold-route acceptance boundaries: no injected world positions/actions/contact
// candidates, unchanged seed and game clock, pickup then original thrown-box
// contact, human idle must not escape, each fresh chord has exactly one landing,
// and JSON restoration must preserve inputs, contact binding and countdown.
func TestRuntimeCourtFourHumanCaptureColdControllerRoute(t *testing.T) {
	config := DefaultRuntimeConfig()
	config.Match.Court = 4
	config.Match.Mode = 128
	config.Match.Seed = 0x12345678
	config.Match.ControllerAssignments = [4]int8{0, 1, 2, 3}
	state, err := NewRuntime(config)
	if err != nil {
		t.Fatal(err)
	}
	step := func(held [4]Buttons) {
		t.Helper()
		next, e := StepVideo(state, held)
		if e != nil {
			t.Fatalf("video %d: %v", state.VideoFrames, e)
		}
		state = next.State
	}
	for state.Playing == nil && state.VideoFrames < 300 {
		step([4]Buttons{})
	}
	if state.Playing == nil || state.VideoFrames != 114 {
		t.Fatal("cold opening boundary changed", state.VideoFrames)
	}
	restored := state
	restore := func() {
		t.Helper()
		raw, e := json.Marshal(state)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &restored); e != nil {
			t.Fatal(e)
		}
		if e = ValidateRuntimeState(restored); e != nil {
			t.Fatal(e)
		}
	}
	restore()
	firstCapture := uint64(0)
	landings := []uint64{}
	advance := func(frames int, p0, p1 Buttons) {
		t.Helper()
		held := [4]Buttons{p0, p1}
		for n := 0; n < frames; n++ {
			before := state.Playing.World.Preparation.Actions
			step(held)
			r, e := StepVideo(restored, held)
			if e != nil || !reflect.DeepEqual(r.State, state) {
				t.Fatalf("video %d JSON replay mismatch: %v", state.VideoFrames, e)
			}
			restored = r.State
			w := state.Playing.World.Preparation.Actions
			if before.SpecialHolder == -1 && w.SpecialHolder == 1 {
				if before.Auxiliary[0].Action != 3 || before.Auxiliary[0].Kind != 10 || state.Playing.World.Preparation.Contacts.Candidates[4] != 1 {
					t.Fatal("capture bypassed thrown Kind10 contact search")
				}
				firstCapture = state.VideoFrames
			}
			if before.SpecialHolder == 1 && before.CaptureCounter != w.CaptureCounter {
				if w.CaptureCounter != before.CaptureCounter-1 {
					t.Fatal("escape countdown skipped a landing")
				}
				landings = append(landings, state.VideoFrames)
			}
		}
	}
	advance(21, ButtonUp, ButtonUp)
	advance(54, ButtonLeft, ButtonUp)
	advance(15, ButtonLeft, 0)
	advance(6, ButtonA, 0)
	advance(15, 0, 0)
	w := state.Playing.World.Preparation.Actions
	if w.Auxiliary[0].Kind != 10 || w.Auxiliary[0].Owner != 0 || w.Actors[0].Player.Action != Action(0x20) {
		t.Fatal("controller did not pick up original box")
	}
	advance(6, ButtonRight|ButtonA, 0)
	advance(60, 0, 0)
	w = state.Playing.World.Preparation.Actions
	if firstCapture == 0 || w.SpecialHolder != 1 || w.CaptureCounter != 4 || w.Actors[1].Player.Action != ActionCapturedIdle || len(landings) != 0 {
		t.Fatal("cold controller route did not capture and retain idle human", firstCapture, w.SpecialHolder, w.CaptureCounter)
	}
	restore()
	for jump := 0; jump < 5; jump++ {
		advance(6, 0, ButtonA|ButtonB)
		if state.Playing.World.Preparation.Actions.Actors[1].Player.Action != ActionCapturedJump {
			t.Fatal("native controller chord did not start captured jump", jump)
		}
		restore()
		advance(120, 0, 0)
		if len(landings) != jump+1 {
			t.Fatal("one fresh chord must yield one escape landing", jump, landings)
		}
		restore()
	}
	w = state.Playing.World.Preparation.Actions
	if w.SpecialHolder != -1 || w.CaptureCounter != 255 || w.Actors[1].Player.Action != ActionIdle {
		t.Fatal("fifth landing did not restore ordinary idle")
	}
	x := w.Actors[1].Player.Body.Position.X
	advance(18, 0, ButtonRight)
	w = state.Playing.World.Preparation.Actions
	if w.Actors[1].Player.Action != ActionWalk || w.Actors[1].Player.Body.Position.X <= x {
		t.Fatal("released human cannot walk")
	}
	t.Logf("cold Court4 seed 0x12345678: original opening 114; thrown Kind10 capture video %d; five human escape landings %v; normal walking video %d; JSON replay matched every video", firstCapture, landings, state.VideoFrames)
}
