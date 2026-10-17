package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Failure cases: the Scene 2 menu's cleared camera is submitted as a court;
// Start's new unentered Scene 1/phase 0 submits the same zero camera; retained
// display disappears on restore; gating also suppresses initialized court and
// referee frames, preventing the second period from becoming visible.
func TestRuntimeIntermissionRetainsCourtUntilInitialized(t *testing.T) {
	s := runtimePlayingForTest(t)
	s.Playing.Match.Clock.Minutes, s.Playing.Match.Clock.Tens, s.Playing.Match.Clock.Ones = 0, 0, 0
	s.Playing.World.Preparation.Scores = [2]uint16{3, 0}
	s.Playing.Match.DisplayedScores = [2][3]uint8{{0, 0, 3}, {0, 0, 0}}
	s.Playing.World.Preparation.Actions.Holder = 6
	s.Playing.World.Preparation.Actions.Ball.Airborne = false
	step, err := StepLogic(s, [4]ControllerFrame{})
	if err != nil {
		t.Fatal(err)
	}
	s = step.State
	assertCourt := func(s RuntimeState) {
		t.Helper()
		for _, d := range []BasketballRenderSnapshot{s.Display.Submitted, s.Display.Visible} {
			if d.Camera.Y < 48 || d.Camera.Y > 297 || d.Camera.Y&255 >= 240 {
				t.Fatalf("menu/uninitialized camera leaked into court: video=%d phase=%d camera=%+v", s.VideoFrames, d.Phase, d.Camera)
			}
		}
	}
	for n := 0; n < 120 && !s.AwaitingNextPeriod; n++ {
		step, err = StepVideo(s, [4]Buttons{})
		if err != nil {
			t.Fatal(err)
		}
		s = step.State
		assertCourt(s)
	}
	if !s.AwaitingNextPeriod {
		t.Fatal("missing intermission")
	}
	submitted := s.Display.Submitted
	for n := 0; n < 60; n++ {
		raw, _ := json.Marshal(s)
		var restored RuntimeState
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		step, err = StepVideo(restored, [4]Buttons{})
		if err != nil {
			t.Fatal(err)
		}
		s = step.State
		assertCourt(s)
		if !reflect.DeepEqual(s.Display.Submitted, submitted) {
			t.Fatal("menu changed the retained court submission")
		}
	}
	// A real video input edge confirms the platform's existing period selections.
	step, err = StepVideo(s, [4]Buttons{ButtonStart})
	if err != nil {
		t.Fatal(err)
	}
	s = step.State
	if s.Opening == nil || s.Opening.Transition.Match.Entered {
		t.Fatal("expected uninitialized next court")
	}
	assertCourt(s)
	if !reflect.DeepEqual(s.Display.Submitted, submitted) {
		t.Fatal("Start submitted uninitialized court")
	}
	sawInitialized := false
	for n := 0; n < 300 && s.Playing == nil; n++ {
		step, err = StepVideo(s, [4]Buttons{})
		if err != nil {
			t.Fatal(err)
		}
		s = step.State
		assertCourt(s)
		if s.Display.Submitted.Phase == 0 && s.Display.Submitted.Camera == (RenderCamera{X: 128, Y: 297}) {
			sawInitialized = true
		}
	}
	if s.Playing == nil || s.Playing.Match.Period != 1 || !sawInitialized {
		t.Fatal("initialized court/referee handoff suppressed")
	}
}
