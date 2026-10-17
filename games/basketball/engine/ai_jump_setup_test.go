package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalAIJumpShotSetup(t *testing.T) {
	b, err := os.ReadFile("testdata/ai-jump-setup.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After struct {
			observedAIPossession
			JumpPhase, VerticalSpeed uint8
			DepthVelocity            uint16
		}
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 77 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		b, a := r.Before, r.After
		heading := HeadingNone
		if b.Heading != 255 {
			heading = Heading(b.Heading / 32)
		}
		v := AIControlView{Actor: r.Actor, Action: Action(b.Action & 127), ActionEntered: b.Action&128 != 0, Heading: heading, FacingLeft: b.Facing&128 != 0, Position: Position{X: Fixed(b.X), Depth: Fixed(b.Depth), Height: Fixed(b.Height)}, HasBall: b.Holder == r.Actor, Random: RandomState(binary.BigEndian.Uint32(b.Random[:])), ShotContext: AIShotContext{JumpPhase: int8(b.JumpPhase), VerticalSpeed: int8(b.VerticalSpeed)}}
		got, err := StepOriginalAIControl(aiControlFromObservation(b.observedAIControl), v)
		if err != nil {
			t.Fatalf("frame %d actor %d: %v", r.Frame, r.Actor, err)
		}
		wantHeading := HeadingNone
		if a.Heading != 255 {
			wantHeading = Heading(a.Heading / 32)
		}
		phase, depthSpeed := b.JumpPhase, b.DepthVelocity
		if got.ShotPlan.ChangeJumpPhase {
			phase = uint8(got.ShotPlan.JumpPhase)
		}
		if got.ShotPlan.ClearDepthVelocity {
			depthSpeed = 0
		}
		if got.Control != aiControlFromObservation(a.observedAIControl) || got.Action != Action(a.Action&127) || got.ActionEntered != (a.Action&128 != 0) || got.Heading != wantHeading || got.FacingLeft != (a.Facing&128 != 0) || got.Random != RandomState(binary.BigEndian.Uint32(a.Random[:])) || phase != a.JumpPhase || depthSpeed != a.DepthVelocity {
			t.Fatalf("frame %d actor %d\nbefore %+v\ngot %+v\nwant %+v", r.Frame, r.Actor, b, got, a)
		}
		if got.ShotPlan.ChangeShotCycle || got.BindBall || got.Ball.Change || got.BindHoop != -1 || b.Request != a.Request || b.X != a.X || b.Depth != a.Depth || b.Height != a.Height {
			t.Fatal("world mutation", r.Frame, got)
		}
	}
	t.Log("matched all 77 original jump-shot setup calls")
}
