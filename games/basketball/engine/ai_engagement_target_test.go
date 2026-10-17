package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalEngagementHoopTarget(t *testing.T) {
	var rows []struct {
		Actor, Action, Mode, Counter, JumpPhase, Target, Heading, Facing uint8
		X, TargetX                                                       Fixed
		Velocity                                                         [4]uint8
	}
	raw, err := os.ReadFile("testdata/engagement-hoop-isolated.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 48 {
		t.Fatal("missing original hoop engagement cases", len(rows))
	}
	for _, r := range rows {
		view := AIControlView{Actor: r.Actor, Action: ActionRun, ActionEntered: true, Position: Position{X: r.X}, BallHolder: 6}
		view.Hoops[0].Position.X = r.TargetX
		control := AIControlState{Mode: 7, Counter: 9, TrackedObject: 7}
		got, err := StepOriginalAIControl(control, view)
		if err != nil {
			t.Fatal(r, err)
		}
		if uint8(got.Action) != r.Action || got.ActionEntered || uint8(got.Control.Mode) != r.Mode || got.Control.Entered || uint8(got.Control.Counter) != r.Counter || got.Control.TrackedObject != int8(r.Target) || uint8(got.Heading)*32 != r.Heading || got.FacingLeft != (r.Facing >= 128) || !got.Engagement.StopPlanar || r.Velocity != ([4]uint8{}) || !got.ShotPlan.ChangeJumpPhase || uint8(got.ShotPlan.JumpPhase) != r.JumpPhase || got.Random != view.Random {
			t.Fatal(r, got)
		}
	}
}
