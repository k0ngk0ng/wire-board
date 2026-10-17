package engine

// Platform handoff failure scenarios, specified before the adapter: resetting
// scores/physics/RNG/AI; advancing time; replaying a seat's old pending input;
// cancelling another seat; losing control mode across opening/intermission or
// JSON restore; silently remapping shared controllers; changing a final result.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRuntimeAutomationHandoffPreservesMatch(t *testing.T) {
	s := runtimePlayingForTest(t)
	s.Held = [4]Buttons{ButtonB, ButtonRight}
	s.PendingPressed = [4]Buttons{ButtonA, ButtonUp}
	s.Playing.World.Input.Memory[0] = InputMemory{LastTap: ButtonRight, TapWindow: 3, PendingButtons: ButtonB}
	s.Playing.World.Input.Memory[1] = InputMemory{LastTap: ButtonLeft, TapWindow: 4}
	before, _ := json.Marshal(s)
	auto := [4]bool{true, true, true, true}
	got, err := SetRuntimeAutomation(s, auto)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Playing.World.Preparation, s.Playing.World.Preparation) || got.Playing.Match != s.Playing.Match || got.Playing.World.Camera != s.Playing.World.Camera || got.LogicSteps != s.LogicSteps || got.VideoFrames != s.VideoFrames {
		t.Fatal("handoff changed gameplay, random/AI state or clock")
	}
	if got.Held[0] != 0 || got.PendingPressed[0] != 0 || got.Playing.World.Input.Memory[0] != (InputMemory{}) || got.Held[1] != s.Held[1] || got.Playing.World.Input.Memory[1] != s.Playing.World.Input.Memory[1] {
		t.Fatal("handoff replayed old input or cleared another seat")
	}
	if got.Config.Match.ControllerAssignments[0] != -1 || got.Playing.Context.Actions.Controls[0] != 128 {
		t.Fatal("original AI was not selected")
	}
	if after, _ := json.Marshal(s); string(after) != string(before) {
		t.Fatal("handoff mutated its input snapshot")
	}
	unchanged, err := SetRuntimeAutomation(got, auto)
	if err != nil || !reflect.DeepEqual(unchanged, got) {
		t.Fatal("unchanged control metadata must be a no-op", err)
	}
	raw, _ := json.Marshal(got)
	var restored RuntimeState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if err = ValidateRuntimeState(restored); err != nil {
		t.Fatal(err)
	}
	auto[0] = false
	human, err := SetRuntimeAutomation(restored, auto)
	if err != nil || human.Config.Match.ControllerAssignments[0] != 0 || human.Playing.Context.Actions.Controls[0] != 0 {
		t.Fatal("manual control did not return to the seat", err)
	}
	if !reflect.DeepEqual(human.Playing.World.Preparation, got.Playing.World.Preparation) || human.Held[0] != 0 || human.PendingPressed[0] != 0 {
		t.Fatal("manual reclaim reset gameplay or held old buttons")
	}
}

func TestRuntimeAutomationOpeningAndIntermission(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Seed = 0x12345678
	s, err := NewRuntime(c)
	if err != nil {
		t.Fatal(err)
	}
	flags := [4]bool{false, false, true, true}
	s, err = SetRuntimeAutomation(s, flags)
	if err != nil {
		t.Fatal(err)
	}
	for frame := 0; frame < 300 && s.Playing == nil; frame++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if s.Playing == nil || s.Playing.Context.Actions.Controls != ([4]uint8{0, 1, 128, 128}) {
		t.Fatal("opening dropped platform controller assignments")
	}
	// Explicit period-boundary state; natural-clock coverage is a separate gate.
	s.Playing.Match.Clock.Minutes, s.Playing.Match.Clock.Tens, s.Playing.Match.Clock.Ones = 0, 0, 0
	s.Playing.World.Preparation.Actions.Ball.Airborne = false
	s.Playing.World.Preparation.Actions.Holder = 6
	for frame := 0; frame < 100 && !s.AwaitingNextPeriod; frame++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if !s.AwaitingNextPeriod {
		t.Fatal("period did not reach the existing intermission")
	}
	flags[1] = true
	s, err = SetRuntimeAutomation(s, flags)
	if err != nil || s.Transition.Scene.Transition.ControllerAssignments[1] != -1 {
		t.Fatal("intermission did not retain new control mode", err)
	}
	for frame := 0; frame < 400 && s.Playing == nil; frame++ {
		var held [4]Buttons
		if s.AwaitingNextPeriod && s.WaitVideoFrames == 0 {
			held[0] = ButtonStart
		}
		r, e := StepVideo(s, held)
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if s.Playing == nil || s.Playing.Match.Period != 1 || s.Playing.Context.Actions.Controls[1] != 128 {
		t.Fatal("next half lost automation")
	}
}

func TestRuntimeAutomationRejectsForeignControllerLayout(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.ControllerAssignments[1] = 0
	s, err := NewRuntime(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := SetRuntimeAutomation(s, [4]bool{})
	if err == nil || !reflect.DeepEqual(got, s) {
		t.Fatal("platform handoff silently reassigned a local shared controller")
	}
}
