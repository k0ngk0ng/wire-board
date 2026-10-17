package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalEngagementTrace(t *testing.T) {
	type state struct {
		observedAIControl
		Court, Difficulty                                  uint8
		PlayerActions                                      [4]Action
		TeammateLink, NextAction, JumpPhase, VerticalSpeed uint8
		Airborne                                           bool
		VelocityX, VelocityDepth                           uint16
		TeamHasHuman                                       bool
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After state
	}
	b, err := os.ReadFile("testdata/ai-engagement.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 206 {
		t.Fatal("missing original engagement decisions", len(rows))
	}
	for _, r := range rows {
		b, a := r.Before, r.After
		h := HeadingNone
		if b.Heading != 255 {
			h = Heading(b.Heading / 32)
		}
		v := AIControlView{Actor: r.Actor, Action: Action(b.Action & 127), ActionEntered: b.Action&128 != 0, Heading: h, FacingLeft: b.Facing&128 != 0, HasBall: b.Holder == r.Actor, TeamHasHuman: b.TeamHasHuman, Players: b.Players, Position: Position{X: Fixed(b.X), Depth: Fixed(b.Depth), Height: Fixed(b.Height)}, Random: RandomState(binary.BigEndian.Uint32(b.Random[:])), Difficulty: b.Difficulty, PlayerActions: b.PlayerActions, Airborne: b.Airborne}
		v.TeammateLink = b.TeammateLink
		v.ShotContext.VerticalSpeed = int8(b.VerticalSpeed)
		got, err := StepOriginalAIControl(aiControlFromObservation(b.observedAIControl), v)
		if err != nil {
			t.Fatalf("frame %d actor %d: %v", r.Frame, r.Actor, err)
		}
		wantH := HeadingNone
		if a.Heading != 255 {
			wantH = Heading(a.Heading / 32)
		}
		if got.Control != aiControlFromObservation(a.observedAIControl) || got.Action != Action(a.Action&127) || got.ActionEntered != (a.Action&128 != 0) || got.Heading != wantH || got.FacingLeft != (a.Facing&128 != 0) || got.Random != RandomState(binary.BigEndian.Uint32(a.Random[:])) {
			t.Fatalf("frame %d actor %d before %+v\ngot %+v\nafter %+v", r.Frame, r.Actor, b, got, a)
		}
		jump, next, vx, vd := b.JumpPhase, b.NextAction, b.VelocityX, b.VelocityDepth
		if got.ShotPlan.ChangeJumpPhase {
			jump = uint8(got.ShotPlan.JumpPhase)
		}
		if got.Engagement.ChangeNextAction {
			next = uint8(got.Engagement.NextAction)
		}
		if got.Engagement.StopPlanar {
			vx, vd = 0, 0
		}
		if jump != a.JumpPhase || next != a.NextAction || vx != a.VelocityX || vd != a.VelocityDepth {
			t.Fatalf("frame %d actor %d command %+v / %+v: want %+v", r.Frame, r.Actor, got.Engagement, got.ShotPlan, a)
		}

		if got.BindBall || got.Ball.Change || got.BindHoop != -1 || b.Request != a.Request || b.Holder != a.Holder || b.X != a.X || b.Depth != a.Depth || b.Height != a.Height {
			t.Fatal("unexpected world mutation", r.Frame)
		}
	}
	t.Logf("matched every %d original mode 7 call", len(rows))
}

func TestOriginalEngagementDispatchBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name         string
		parameter    uint8
		action       Action
		entered, air bool
		height       int
		speed        int8
		want         Action
		mode         AIMode
		next         bool
		nextAction   Action
	}{
		{"standing punch", 0, ActionRun, true, false, 0, 0, 0x60, 0, false, 0},
		{"direct kick", 2, ActionRun, true, false, 0, 0, 0x62, 20, false, 0},
		{"jump request", 1, ActionRun, true, false, 0, 0, ActionJump, 7, false, 0},
		{"jump high speed", 1, ActionJump, true, true, 20, 3, ActionJump, 7, false, 0},
		{"jump attack", 1, ActionJump, true, true, 20, 2, 0x61, 20, false, 0},
		{"falling byte unsigned", 1, ActionJump, true, true, 20, -1, ActionJump, 7, false, 0},
		{"landing pending", 3, ActionLand, false, false, 0, 0, ActionLand, 7, false, 0},
		{"landing followup", 3, ActionLand, true, false, 0, 0, ActionLand, 20, true, 0x63},
		{"spin", 8, ActionRun, true, false, 0, 0, 0x68, 0, false, 0},
		{"runjump low", 10, ActionRunningJump, true, true, 15, 0, ActionRunningJump, 7, false, 0},
		{"runjump threshold", 10, ActionRunningJump, true, true, 16, 0, 0x61, 20, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := AIControlView{Actor: 0, Action: tc.action, ActionEntered: tc.entered, Airborne: tc.air, Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: Fixed(tc.height) * Pixel}}
			v.Players[2] = Position{X: 220 * Pixel}
			got := AIControlDecision{Control: AIControlState{Mode: 7, Entered: true, TrackedObject: 2, PlanParameter: tc.parameter}, Action: tc.action, ActionEntered: tc.entered}
			err := advanceOriginalEngagement(&got, v, AIEngagementContext{VerticalSpeed: tc.speed})
			if err != nil || got.Action != tc.want || got.Control.Mode != tc.mode || got.Engagement.ChangeNextAction != tc.next || tc.next && got.Engagement.NextAction != tc.nextAction {
				t.Fatal(got, err)
			}
		})
	}
}
func TestOriginalEngagementHandoffs(t *testing.T) {
	for _, tc := range []struct {
		ball bool
		link uint8
		want AIMode
	}{{true, 0, 1}, {false, 128, 28}} {
		v := AIControlView{Actor: 0, HasBall: tc.ball}
		got := AIControlDecision{Control: AIControlState{Mode: 7}}
		if err := advanceOriginalEngagement(&got, v, AIEngagementContext{TeammateLink: tc.link}); err != nil || got.Control.Mode != tc.want || got.Control.Entered || got.Control.Counter != 8 || !got.ShotPlan.ChangeJumpPhase || got.ShotPlan.JumpPhase != 0 {
			t.Fatal(got, err)
		}
	}
	v := AIControlView{Actor: 0}
	got := AIControlDecision{Control: AIControlState{Mode: 7, Entered: true, PlanParameter: 12}}
	if err := advanceOriginalEngagement(&got, v, AIEngagementContext{}); err == nil {
		t.Fatal("unrepresented dispatch accepted")
	}
}
