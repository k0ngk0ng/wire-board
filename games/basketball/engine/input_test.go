package engine

// Before implementation: button identity, A/B chord delay, taps surviving key
// release, double-tap running, opposite-direction braking, window expiry, and
// untouched pending requests during running. Oracle is original input code.

import (
	"encoding/json"
	"os"
	"testing"
)

type observedInput struct {
	Request, Heading, TapTimer, LastTap, AttackTapTimer, LastAttackTap, ButtonDelay, PendingButtons, Boost uint8
}

func inputMemory(r observedInput) InputMemory {
	return InputMemory{TapWindow: r.TapTimer, LastTap: Buttons(r.LastTap), AttackTapWindow: r.AttackTapTimer, LastAttackTap: Buttons(r.LastAttackTap), ButtonDelay: r.ButtonDelay, PendingButtons: Buttons(r.PendingButtons)}
}
func observedRequest(value uint8) ActionRequest {
	return ActionRequest{Action: Action(value & 127), Pending: value&128 == 0}
}

func TestInputAgainstOriginalExecution(t *testing.T) {
	var rows []struct {
		Scenario                                 string
		Frame                                    int
		Held, Pressed, Action, Pose, Air, Holder uint8
		Before, After                            observedInput
	}
	data, err := os.ReadFile("testdata/input.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 638 {
		t.Fatalf("unexpected input observation count %d", len(rows))
	}
	for _, r := range rows {
		if r.Holder == 0 || r.Before.Boost != 0 || r.After.Boost != 0 {
			t.Fatal("fixture leaves free-player context")
		}
		ctx := FreeInputContext{Action: Action(r.Action & 127), PoseFacingLeft: r.Pose&128 != 0, Airborne: r.Air&128 != 0, Request: observedRequest(r.Before.Request)}
		got, err := ResolveFreeInput(ctx, inputMemory(r.Before), ControllerFrame{Held: Buttons(r.Held), Pressed: Buttons(r.Pressed)})
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		heading := HeadingNone
		if r.After.Heading != 255 {
			heading = Heading(r.After.Heading / 32)
		}
		if got.Memory != inputMemory(r.After) || got.Heading != heading || got.Request != observedRequest(r.After.Request) {
			t.Fatalf("%s frame %d: got %+v; original %+v", r.Scenario, r.Frame, got, r.After)
		}
	}
	t.Log("matched 638 original human-input calls")
}

func TestInputCombinesButtonsAndRetainsQuickTaps(t *testing.T) {
	ctx := FreeInputContext{Action: ActionIdle}
	one, err := ResolveFreeInput(ctx, InputMemory{}, ControllerFrame{Held: ButtonA, Pressed: ButtonA})
	if err != nil {
		t.Fatal(err)
	}
	if one.Request.Action != ActionIdle || one.Memory.ButtonDelay != 1 {
		t.Fatal(one)
	}
	two, err := ResolveFreeInput(ctx, one.Memory, ControllerFrame{Held: ButtonB, Pressed: ButtonB})
	if err != nil {
		t.Fatal(err)
	}
	if two.Request.Action != ActionJump || two.Memory.PendingButtons != 0 {
		t.Fatal(two)
	}
	tap, err := ResolveFreeInput(ctx, one.Memory, ControllerFrame{})
	if err != nil {
		t.Fatal(err)
	}
	if tap.Request.Action != ActionAttackA {
		t.Fatal("released quick tap disappeared", tap)
	}
}

func TestInputDoubleTapAndRunBrake(t *testing.T) {
	ctx := FreeInputContext{Action: ActionIdle}
	a, _ := ResolveFreeInput(ctx, InputMemory{}, ControllerFrame{Held: ButtonRight, Pressed: ButtonRight})
	ctx.Action = ActionWalk
	b, _ := ResolveFreeInput(ctx, a.Memory, ControllerFrame{})
	c, err := ResolveFreeInput(ctx, b.Memory, ControllerFrame{Held: ButtonRight, Pressed: ButtonRight})
	if err != nil {
		t.Fatal(err)
	}
	if c.Request.Action != ActionRun {
		t.Fatal(c)
	}
	ctx.Action = ActionRun
	ctx.Request = ActionRequest{Action: ActionRun}
	d, err := ResolveFreeInput(ctx, c.Memory, ControllerFrame{Held: ButtonLeft, Pressed: ButtonLeft})
	if err != nil {
		t.Fatal(err)
	}
	if d.Request.Action != ActionBrake {
		t.Fatal(d)
	}
}
