package engine

// Before implementation: original transition restrictions, action entry timing,
// same-update jump revisit, last-pose retention, left-facing persistence, fixed
// movement/landing order, and accidental acceptance of unimplemented actions.

import (
	"encoding/json"
	"os"
	"testing"
)

type observedPlayer struct {
	Action, Request, Facing, Pose, Clock, Cursor, RepeatCount, NextAction      int32
	Air, Attachment, Boost, Heading, JumpGrade, Arena, Holder, Team, Character int32
	X, Depth, Height, VX, VY, VZ, Gravity                                      int32
	Passage                                                                    int32
}

func playerFromObservation(r observedPlayer) Player {
	heading := HeadingNone
	if r.Heading != 255 {
		heading = Heading(r.Heading / 32)
	}
	next := ActionIdle
	if r.NextAction < 128 {
		next = Action(r.NextAction)
	}
	return Player{
		Body:   Body{Position: Position{X: Fixed(r.X << 8 >> 8), Depth: Fixed(r.Depth), Height: Fixed(r.Height << 8 >> 8)}, Velocity: Velocity{X: Fixed(int16(r.VX)), Depth: Fixed(int16(r.VY)), Height: Fixed(int16(r.VZ))}, Gravity: Fixed(int16(r.Gravity))},
		Action: Action(r.Action & 127), Entered: r.Action&128 != 0,
		Request:    ActionRequest{Action: Action(r.Request & 127), Pending: r.Request&128 == 0},
		FacingLeft: r.Facing == 128, Pose: Pose{Frame: uint8(r.Pose & 127), Mirrored: r.Pose&128 != 0},
		Animation: AnimationState{Cursor: uint8(r.Cursor), Elapsed: uint8(r.Clock), Loops: uint8(r.RepeatCount), NextAction: next, HasNextAction: r.NextAction < 128},
		Airborne:  r.Air&128 != 0, Heading: heading,
		NewYorkPassage: uint8(r.Passage),
	}
}

func TestFreePlayerActionPhaseAgainstOriginal(t *testing.T) {
	var records []struct {
		Scenario      string
		Frame         int
		Before, After observedPlayer
	}
	data, err := os.ReadFile("testdata/free-player.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 633 {
		t.Fatalf("expected 633 completed original phases, got %d", len(records))
	}
	profile := KunioProfile()
	for _, r := range records {
		if r.Before.Attachment != 0 || r.Before.Boost != 0 || r.Before.Holder == 0 || r.Before.Team != 0 || r.Before.Character != 0 || r.Before.JumpGrade != 3 || r.Before.Arena != 1 || r.After.Attachment != 64 {
			t.Fatalf("fixture leaves declared free-player context: %+v", r)
		}
		input, want := playerFromObservation(r.Before), playerFromObservation(r.After)
		got, err := StepFreePlayer(input, profile, NewYorkGroundCourt())
		if err != nil {
			t.Fatalf("%s frame=%d: %v", r.Scenario, r.Frame, err)
		}
		if got != want {
			t.Fatalf("%s frame=%d\ninput=%+v\ngot=%+v\nwant=%+v", r.Scenario, r.Frame, input, got, want)
		}
	}
	t.Log("matched 633 original world action phases for unpossessed Kunio")
}

func TestFreePlayerDoesNotPretendUnknownActionsWork(t *testing.T) {
	p := Player{Action: Action(0x3e)}
	if _, err := StepFreePlayer(p, KunioProfile(), NewYorkGroundCourt()); err == nil {
		t.Fatal("unsupported action silently accepted")
	}
}
