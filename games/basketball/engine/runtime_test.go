package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func runtimePlayingForTest(t *testing.T) RuntimeState {
	t.Helper()
	c := DefaultRuntimeConfig()
	c.Match.Seed = 0x12345678
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; s.Playing == nil && n < 300; n++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(n, e)
		}
		s = r.State
	}
	if s.Playing == nil {
		t.Fatal("opening did not enter the real world")
	}
	return s
}
func TestRuntimeVideoCadencePauseAndEdges(t *testing.T) {
	s := runtimePlayingForTest(t)
	if !s.Display.HasVisible {
		t.Fatal("runtime did not publish its staged display")
	}
	before := s.Playing.World.Updates
	r, e := StepVideo(s, [4]Buttons{ButtonB})
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	r, e = StepVideo(s, [4]Buttons{})
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	r, e = StepVideo(s, [4]Buttons{})
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	if !r.LogicAdvanced || s.Playing.World.Updates != before+1 || s.Playing.World.Input.Pressed[0]&ButtonB == 0 {
		t.Fatal("video cadence or latched short press failed", r.LogicAdvanced, s.Playing.World.Input.Pressed)
	}
	for n := 0; n < 3; n++ {
		r, e = StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if s.Playing.World.Input.Pressed[0] != 0 {
		t.Fatal("short press replayed")
	}
	frames := [4]ControllerFrame{{Pressed: ButtonStart}}
	before = s.Playing.World.Updates
	r, e = StepLogic(s, frames)
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	if s.Playing.Match.Phase != MatchPaused || s.Playing.World.Updates != before+1 {
		t.Fatal("pause entry must finish current world step")
	}
	clock := s.Playing.Match.Clock.VideoRemainder
	updates := s.Playing.World.Updates
	videos := s.VideoFrames
	for n := 0; n < 30; n++ {
		r, e = StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if s.VideoFrames != videos+30 || s.Playing.World.Updates != updates || s.Playing.Match.Clock.VideoRemainder != clock {
		t.Fatal("pause did not freeze playing clock")
	}
	r, e = StepLogic(s, frames)
	if e != nil {
		t.Fatal(e)
	}
	if r.State.Playing.Match.Phase != MatchPlaying || r.State.Playing.World.Updates != updates || r.State.VideoFrames != s.VideoFrames {
		t.Fatal("resume is projection-only; explicit logic must not tick video")
	}
}
func TestRuntimeRoundTripAndValidation(t *testing.T) {
	s := runtimePlayingForTest(t)
	data, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var restored RuntimeState
	if e = json.Unmarshal(data, &restored); e != nil {
		t.Fatal(e)
	}
	if e = ValidateRuntimeState(restored); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(s, restored) {
		t.Fatal("snapshot changed state")
	}
	a, e := StepVideo(s, [4]Buttons{ButtonRight})
	if e != nil {
		t.Fatal(e)
	}
	b, e := StepVideo(restored, [4]Buttons{ButtonRight})
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("restored runtime diverged", e)
	}
	if e = ValidateRuntimeState(RuntimeState{}); e == nil {
		t.Fatal("nil phases accepted")
	}
	malformed := restored
	malformed.Opening = &OpeningState{}
	if e = ValidateRuntimeState(malformed); e == nil {
		t.Fatal("multiple active phases accepted")
	}
	malformed = restored
	malformed.Config.Match.Court = 7
	if e = ValidateRuntimeState(malformed); e == nil {
		t.Fatal("config/runtime mismatch accepted")
	}
}
func TestRuntimePeriodBoundaryPreservesScores(t *testing.T) {
	s := runtimePlayingForTest(t)
	s.Playing.Match.Clock.Minutes, s.Playing.Match.Clock.Tens, s.Playing.Match.Clock.Ones = 0, 0, 0
	s.Playing.Match.DisplayedScores = [2][3]uint8{{0, 0, 3}, {0, 0, 0}}
	s.Playing.World.Preparation.Scores = [2]uint16{3, 0}
	s.Playing.World.Preparation.Actions.Holder = 6
	s.Playing.World.Preparation.Actions.Ball.Airborne = false
	r, e := StepLogic(s, [4]ControllerFrame{})
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	if s.Transition == nil || s.Transition.Scene.Transition.Match.Phase != MatchPeriodEnding {
		t.Fatal("missing period wait")
	}
	for n := 0; !s.AwaitingNextPeriod && n < 100; n++ {
		r, e = StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(n, e)
		}
		s = r.State
	}
	if !s.AwaitingNextPeriod {
		t.Fatal("period did not reach intermission")
	}
	if s.Transition.Scene.Transition.Scores != ([2]uint16{3, 0}) || s.Transition.Scene.Transition.Match.Period != 1 {
		t.Fatal("intermission lost score or period")
	}
	for s.WaitVideoFrames > 0 {
		r, e = StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	r, e = StepLogic(s, [4]ControllerFrame{{Pressed: ButtonStart}})
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	for n := 0; s.Playing == nil && n < 300; n++ {
		r, e = StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(n, e)
		}
		s = r.State
	}
	if s.Playing == nil || s.Playing.Match.Period != 1 || s.Playing.World.Preparation.Scores != ([2]uint16{3, 0}) {
		t.Fatal("next period did not preserve scores")
	}
}

func TestRuntimeColdDisplayWaitsForCourt(t *testing.T) {
	s, e := NewRuntime(DefaultRuntimeConfig())
	if e != nil {
		t.Fatal(e)
	}
	first, e := StepVideo(s, [4]Buttons{})
	if e != nil {
		t.Fatal(e)
	}
	if first.State.Display.HasSubmission || first.State.Display.HasVisible {
		t.Fatal("cold zeroed object slots were submitted before court initialization")
	}
	s = first.State
	for n := 0; !s.Display.HasVisible && n < 10; n++ {
		next, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = next.State
	}
	if !s.Display.HasVisible || len(s.Display.Visible.DrawOrder) != 0 || s.Display.Visible.Camera != (RenderCamera{X: 128, Y: 297}) {
		t.Fatal("first court image must be a valid empty initialized view", s.Display.Visible)
	}
}

func TestRuntimeShortDirectionHasOneHeldSample(t *testing.T) {
	s := runtimePlayingForTest(t)
	x := s.Playing.World.Preparation.Actions.Actors[0].Player.Body.Position.X
	for _, held := range [][4]Buttons{{ButtonRight}, {}, {}} {
		r, e := StepVideo(s, held)
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if s.Playing.World.Input.Held[0]&ButtonRight == 0 || s.Playing.World.Preparation.Actions.Actors[0].Player.Body.Position.X <= x {
		t.Fatal("latched direction became a pressed edge without a real held sample")
	}
	if s.Held[0] != 0 {
		t.Fatal("one-sample logical hold overwrote physical held state")
	}
	r, e := StepVideo(s, [4]Buttons{ButtonRight})
	if e != nil {
		t.Fatal(e)
	}
	if r.State.PendingPressed[0]&ButtonRight == 0 {
		t.Fatal("second short tap was lost after normalized sample")
	}
}

func TestRuntimeCancelInputOnlyClearsRequestedController(t *testing.T) {
	s := runtimePlayingForTest(t)
	r, e := StepVideo(s, [4]Buttons{ButtonB, ButtonRight})
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	want := s
	want.Held[0], want.PendingPressed[0] = 0, 0
	got, e := ClearRuntimeInput(s, 0)
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("input cancellation changed game state or another controller", e)
	}
	if s.PendingPressed[0]&ButtonB == 0 {
		t.Fatal("input cancel mutated its original snapshot")
	}
	for n := 0; n < 3; n++ {
		r, e = StepVideo(got, [4]Buttons{0, ButtonRight})
		if e != nil {
			t.Fatal(e)
		}
		got = r.State
	}
	if got.Playing.World.Input.Pressed[0]&ButtonB != 0 {
		t.Fatal("cancelled tap was replayed")
	}
	rejected, e := ClearRuntimeInput(s, 4)
	if e == nil || !reflect.DeepEqual(rejected, s) {
		t.Fatal("invalid controller cancellation was not atomic")
	}
}

func TestRuntimeCancelSelectedButtonsPreservesOtherInputs(t *testing.T) {
	s := runtimePlayingForTest(t)
	r, e := StepVideo(s, [4]Buttons{ButtonA | ButtonB | ButtonRight, ButtonUp})
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	want := s
	want.Held[0] &^= ButtonB
	want.PendingPressed[0] &^= ButtonB
	got, e := ClearRuntimeButtons(s, 0, ButtonB)
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("masked cancellation changed other inputs or game state", e)
	}
	if s.PendingPressed[0]&ButtonB == 0 {
		t.Fatal("masked cancellation mutated its input")
	}
	unchanged, e := ClearRuntimeButtons(s, 0, 0)
	if e != nil || !reflect.DeepEqual(unchanged, s) {
		t.Fatal("empty mask must be a no-op", e)
	}
	bad, e := ClearRuntimeButtons(s, 4, ButtonB)
	if e == nil || !reflect.DeepEqual(bad, s) {
		t.Fatal("invalid masked controller cancellation was not atomic")
	}
	all, e := ClearRuntimeButtons(s, 0, 255)
	if e != nil {
		t.Fatal(e)
	}
	legacy, e := ClearRuntimeInput(s, 0)
	if e != nil || !reflect.DeepEqual(all, legacy) {
		t.Fatal("whole-controller helper differs from mask 255")
	}
}
