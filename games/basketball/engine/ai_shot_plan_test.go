package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalAIShotPlanning(t *testing.T) {
	data, err := os.ReadFile("testdata/ai-shot-plan.json")
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		observedAIControl
		Team, Court, JumpPhase, VerticalSpeed, ShotCycle, ShotSkill, Difficulty uint8
		TeamHasHuman, Airborne                                                  bool
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After state
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 283 {
		t.Fatal("missing original mode-six calls", len(rows))
	}
	for _, row := range rows {
		b, a := row.Before, row.After
		heading := HeadingNone
		if b.Heading != 255 {
			heading = Heading(b.Heading / 32)
		}
		view := AIControlView{Actor: row.Actor, Action: Action(b.Action & 127), ActionEntered: b.Action&128 != 0,
			Heading: heading, FacingLeft: b.Facing&128 != 0, HasBall: b.Holder == row.Actor,
			Position: Position{X: Fixed(b.X), Depth: Fixed(b.Depth), Height: Fixed(b.Height)},
			Team:     b.Team, TeamHasHuman: b.TeamHasHuman, Airborne: b.Airborne, Difficulty: b.Difficulty,
			Random: RandomState(binary.BigEndian.Uint32(b.Random[:]))}
		view.CourtID = b.Court
		view.ShotContext = AIShotContext{JumpPhase: int8(b.JumpPhase), VerticalSpeed: int8(b.VerticalSpeed), ShotSkill: b.ShotSkill}
		got, err := StepOriginalAIControl(aiControlFromObservation(b.observedAIControl), view)
		if err != nil {
			t.Fatalf("frame %d actor %d: %v", row.Frame, row.Actor, err)
		}
		wantHeading := HeadingNone
		if a.Heading != 255 {
			wantHeading = Heading(a.Heading / 32)
		}
		phase, cycle := b.JumpPhase, b.ShotCycle
		if got.ShotPlan.ChangeJumpPhase {
			phase = uint8(got.ShotPlan.JumpPhase)
		}
		if got.ShotPlan.ChangeShotCycle {
			cycle = uint8(got.ShotPlan.ShotCycle)
		}
		if got.Control != aiControlFromObservation(a.observedAIControl) || got.Action != Action(a.Action&127) ||
			got.ActionEntered != (a.Action&128 != 0) || got.Heading != wantHeading || got.FacingLeft != (a.Facing&128 != 0) ||
			got.Random != RandomState(binary.BigEndian.Uint32(a.Random[:])) || phase != a.JumpPhase || cycle != a.ShotCycle {
			t.Fatalf("frame %d actor %d\nbefore %+v\ngot %+v phase %d cycle %d\nwant %+v", row.Frame, row.Actor, b, got, phase, cycle, a)
		}
		if b.Holder != a.Holder || b.Request != a.Request || b.X != a.X || b.Depth != a.Depth || b.Height != a.Height ||
			got.BindBall || got.BindHoop != -1 || got.Ball.Change {
			t.Fatal("unexpected world mutation", row.Frame, got)
		}
	}
	t.Logf("matched all %d original mode-six calls, including shot/jump commands and RNG", len(rows))
}

func TestOriginalAIShotPlanningBoundaries(t *testing.T) {
	view := AIControlView{Actor: 2, HasBall: true, Position: Position{X: 100 * Pixel, Depth: 60 * Pixel}, Team: 1}
	initial := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 14}, Action: ActionCarrierJump, ActionEntered: true}
	for _, speed := range []int8{-1, 0, 1} {
		v := view
		v.Airborne = true
		got := initial
		if err := advanceOriginalShotPlan(&got, v, AIShotContext{Court: 1, VerticalSpeed: speed}); err != nil {
			t.Fatal(err)
		}
		if speed == 0 {
			if got.ActionEntered || !got.ShotPlan.ChangeJumpPhase || got.ShotPlan.JumpPhase != -1 {
				t.Fatal("jump boost not requested", got)
			}
		} else if got != initial {
			t.Fatal("unsigned vertical velocity comparison changed", speed, got)
		}
	}
	for _, teamHuman := range []bool{true, false} {
		got := AIControlDecision{Control: AIControlState{Mode: 6}, Random: 0x2468ace0}
		v := view
		v.TeamHasHuman = teamHuman
		v.Team = 7
		v.Action = ActionAimAirShot
		got.Action = v.Action
		if err := advanceOriginalShotPlan(&got, v, AIShotContext{VerticalSpeed: 2}); err != nil {
			t.Fatal(err)
		}
		if teamHuman {
			if got.Control.PlanParameter != 0 || got.Action != ActionAimShot || !got.ShotPlan.ChangeShotCycle {
				t.Fatal(got)
			}
		} else if got.Control.PlanParameter != 1 || got.Random != 0x2468ace0 {
			t.Fatal("special team entry consumed weighted RNG", got)
		}
	}
}
