package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Failure inventory before implementation: captured input uses the ordinary
// movement group; B/A+ B delay loses a quick second edge; running jump becomes
// standing jump; a held A+B repeats escapes without a new press; the capture
// binding loses priority while the actor still holds the ball; denied airborne
// follow-up consumes escape counter; no input automatically jumps like AI;
// post-release input remains in the captured group; old input survives capture.
func TestOriginalCapturedHumanInput(t *testing.T) {
	f, err := os.Open("testdata/captured-human-input.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []heldAuxiliaryInputCase
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5264 {
		t.Fatal("incomplete captured human oracle", len(rows))
	}
	seen := map[Action]bool{}
	for _, r := range rows {
		s := r.Before
		c := r.context(t)
		c.PickupFlags, c.JumpEnhancements = r.PickupBefore, r.JumpEnhancements
		effects, err := advanceWorldHumanInputWithEffects(&s, r.Actor, c)
		flags := r.PickupBefore
		if effects.ChangePickupFlags {
			flags[r.Actor] = effects.PickupFlags
		}
		if err != nil || s != r.After || flags != r.PickupAfter {
			t.Fatalf("%s tick%d: %s flags%v/%v err=%v", r.Name, r.Tick, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "state"), flags, r.PickupAfter, err)
		}
		if s.World.Actors[r.Actor].Player.Request.Pending {
			seen[s.World.Actors[r.Actor].Player.Request.Action] = true
		}
	}
	t.Logf("matched %d original805C calls; requests=%v", len(rows), seen)
}

func TestWorldCapturedHumanInputThroughEscape(t *testing.T) {
	for _, holder := range []int8{1, 6} {
		t.Run(map[int8]string{1: "holding-ball", 6: "no-ball"}[holder], func(t *testing.T) {
			s, c := capturedWorldBeforeContact(t, holder, 4)
			c.Actions.Controls[1] = 1
			// Contact suppression reads the preceding rendered positions. Keep
			// these coherent with the controlled pre-contact actor/object setup.
			projection, err := StepOriginalProjection(resetOriginalWorldDrawList(s.Projection), ProjectionContext{CameraX: s.Camera.X, CameraY: s.Camera.Y, LayoutID: c.LayoutID, Poses: worldLoopPoses(s.Preparation.Actions), Positions: worldLoopPositions(s.Preparation.Actions)})
			if err != nil {
				t.Fatal(err)
			}
			s.Projection = projection
			last := Buttons(0)
			tick := 0
			step := func(held Buttons) {
				t.Helper()
				input := WorldLoopInput{}
				input.Frames[1] = ControllerFrame{Held: held, Pressed: held &^ last}
				r, e := StepWorldLoop(s, input, c)
				if e != nil {
					t.Fatalf("tick%d held%02x: %v", tick, held, e)
				}
				s = r.State
				last = held
				tick++
			}
			actor := func() Player { return s.Preparation.Actions.Actors[1].Player }
			step(0)
			if s.Preparation.Actions.SpecialHolder != 1 || actor().Action != ActionCapturedIdle || s.Preparation.Actions.CaptureCounter != 4 {
				t.Fatal("production preparation did not capture human", s.Preparation.Actions)
			}
			if s.Preparation.Contacts.Candidates[4] != 1 {
				t.Fatal("capture bypassed contact search")
			}
			if holder == 1 && (s.Preparation.Actions.Holder != 6 || s.Preparation.Actions.Ball.Action != BallDropped) {
				t.Fatal("capture did not release held ball through scheduled request", s.Preparation.Actions.Ball)
			}
			for n := 0; n < 32; n++ {
				step(0)
			}
			if actor().Action != ActionCapturedIdle || s.Preparation.Actions.CaptureCounter != 4 || s.Preparation.Actions.SpecialHolder != 1 {
				t.Fatal("idle human escaped or jumped without input", actor(), s.Preparation.Actions.CaptureCounter)
			}
			x := actor().Body.Position.X
			for n := 0; n < 5; n++ {
				step(ButtonRight)
			}
			if actor().Action != ActionCapturedWalk || actor().Body.Position.X <= x {
				t.Fatal("human captured walk not applied", actor())
			}
			step(0)
			step(ButtonRight)
			if actor().Action != ActionCapturedRun {
				t.Fatal("double tap did not enter captured run", actor())
			}
			step(ButtonRight | ButtonA | ButtonB)
			step(ButtonRight | ButtonA | ButtonB)
			if actor().Action != ActionCapturedRunningJump || !actor().Airborne {
				t.Fatal("run A+B did not request55", actor())
			}
			// Holding the chord is one press; do not generate new edges automatically.
			for n := 0; n < 40; n++ {
				step(ButtonA | ButtonB)
			}
			if s.Preparation.Actions.CaptureCounter != 3 || actor().Action != ActionCapturedIdle {
				t.Fatal("one held chord did not yield exactly one landing", actor(), s.Preparation.Actions.CaptureCounter)
			}
			step(0)
			for remaining := uint8(3); ; remaining-- {
				step(ButtonA | ButtonB)
				step(ButtonA | ButtonB)
				if actor().Action != ActionCapturedJump || !actor().Airborne {
					t.Fatal("fresh grounded chord did not jump", remaining, actor())
				}
				// A repeated airborne chord must not itself decrement the counter.
				step(0)
				step(ButtonA | ButtonB)
				step(ButtonA | ButtonB)
				if s.Preparation.Actions.CaptureCounter != remaining {
					t.Fatal("airborne press consumed escape count")
				}
				landed := false
				for n := 0; n < 40; n++ {
					step(0)
					if s.Preparation.Actions.CaptureCounter == remaining-1 {
						landed = true
						break
					}
				}
				if !landed {
					t.Fatal("human jump never landed", remaining, actor())
				}
				if remaining == 0 {
					break
				}
				for n := 0; actor().Action != ActionCapturedIdle && n < 20; n++ {
					step(0)
				}
				if actor().Action != ActionCapturedIdle {
					t.Fatal("captured landing did not return to wait", actor())
				}
				for n := 0; n < 12; n++ {
					step(0)
				}
				if s.Preparation.Actions.CaptureCounter != remaining-1 {
					t.Fatal("human auto-jumped while waiting")
				}
			}
			if s.Preparation.Actions.SpecialHolder != -1 || s.Preparation.Actions.CaptureCounter != 255 || s.Preparation.Actions.Auxiliary[0].Action != 7 {
				t.Fatal("fifth human landing did not release container", s.Preparation.Actions)
			}
			x = actor().Body.Position.X
			for n := 0; n < 16; n++ {
				step(ButtonRight)
			}
			if actor().Action != ActionWalk || actor().Body.Position.X <= x {
				t.Fatal("human control did not return to ordinary walking", actor())
			}
			t.Logf("Court4 human holder%d escaped after five explicit chords in %d World updates; final action=%02x", holder, tick, actor().Action)
		})
	}
}
