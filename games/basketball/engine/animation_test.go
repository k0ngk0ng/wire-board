package engine

// Before implementation: off-by-one pose durations, mirror XOR, finite/infinite
// repeats, completion changing the action too early, and invalid sequence state.
// Expectations are sampled at the original animation routine's actual returns.

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

func TestAnimationAgainstOriginalExecution(t *testing.T) {
	type rawState struct{ Action, Next, Clock, Cursor, Repeat, Pose uint8 }
	var fixture struct {
		Specs map[string]struct {
			Frames     []struct{ Pose, Ticks uint8 }
			End, Limit uint8
		}
		Records []struct {
			Scenario              string
			Frame, Actor, Address int
			Facing                uint8
			Before, After         rawState
		}
	}
	data, err := os.ReadFile("testdata/animation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Records) != 1406 {
		t.Fatalf("unexpected reference count %d", len(fixture.Records))
	}
	for _, r := range fixture.Records {
		raw := fixture.Specs[strconv.Itoa(r.Address)]
		spec := AnimationSpec{RepeatLimit: raw.Limit}
		switch raw.End {
		case 255:
			spec.End = RepeatForever
		case 254:
			spec.End = RepeatRemaining
		case 253:
			spec.End = RepeatUntilCount
		default:
			t.Fatalf("unknown original ending %d", raw.End)
		}
		for _, f := range raw.Frames {
			spec.Frames = append(spec.Frames, AnimationFrame{Pose: Pose{Frame: f.Pose & 127, Mirrored: f.Pose&128 != 0}, Ticks: f.Ticks})
		}
		in := AnimationState{Cursor: r.Before.Cursor, Elapsed: r.Before.Clock, Loops: r.Before.Repeat, HasNextAction: r.Before.Next < 128, NextAction: Action(r.Before.Next & 127)}
		got, err := AdvanceAnimation(spec, in, r.Facing == 128)
		if err != nil {
			t.Fatal(err)
		}
		wantPose := Pose{Frame: r.After.Pose & 127, Mirrored: r.After.Pose&128 != 0}
		transition := r.Before.Next < 128 && r.After.Next == 255 && r.After.Repeat == 255
		if got.State.Cursor != r.After.Cursor || got.State.Elapsed != r.After.Clock || got.State.Loops != r.After.Repeat || got.Pose != wantPose || got.State.HasNextAction != (r.After.Next < 128) || (got.State.HasNextAction && got.State.NextAction != Action(r.After.Next)) || got.ChangeAction != transition || (transition && got.Action != Action(r.After.Action)) {
			t.Fatalf("%s frame=%d actor=%d sequence=%04x\nbefore=%+v\ngot=%+v\noriginal=%+v", r.Scenario, r.Frame, r.Actor, r.Address, r.Before, got, r.After)
		}
	}
	t.Log("matched 1406 original animation calls across 14 observed sequences")
}

func TestAnimationRejectsInvalidState(t *testing.T) {
	valid := AnimationSpec{Frames: []AnimationFrame{{Ticks: 1}}, End: RepeatForever}
	for _, c := range []struct {
		spec  AnimationSpec
		state AnimationState
	}{
		{AnimationSpec{}, AnimationState{}},
		{valid, AnimationState{Cursor: 1}},
		{AnimationSpec{Frames: []AnimationFrame{{Ticks: 0}}}, AnimationState{}},
		{AnimationSpec{Frames: []AnimationFrame{{Ticks: 1}}, End: 99}, AnimationState{}},
	} {
		if _, err := AdvanceAnimation(c.spec, c.state, false); err == nil {
			t.Fatalf("invalid animation accepted: %+v", c)
		}
	}
}
