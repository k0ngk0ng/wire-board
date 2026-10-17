package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type observedBallReaction struct {
	observedAIControl
	Position                      Position
	BallAction, BallFacing, Carry uint8
	LandingX, LandingDepth        int32
	Options                       [4]AITacticalOption
}

func TestOriginalSharedBallReactions(t *testing.T) {
	b, err := os.ReadFile("testdata/ai-ball-reaction.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario      string
		Frame         int
		Actor         uint8
		Before, After observedBallReaction
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1039 {
		t.Fatal("unexpected ball reaction count", len(rows))
	}
	for _, r := range rows {
		heading := HeadingNone
		if r.Before.Heading != 255 {
			heading = Heading(r.Before.Heading / 32)
		}
		options := r.Before.Options
		view := AIControlView{Actor: r.Actor, Action: Action(r.Before.Action & 127), ActionEntered: r.Before.Action&128 != 0, Heading: heading, FacingLeft: r.Before.Facing&128 != 0, Position: r.Before.Position, HasBall: r.Before.Holder == r.Actor, Players: r.Before.Players, BallPosition: r.Before.Ball, BallHolder: int8(r.Before.Holder), Random: RandomState(binary.BigEndian.Uint32(r.Before.Random[:])), BallAction: BallAction(r.Before.BallAction & 127), BallFacingLeft: r.Before.BallFacing&128 != 0, BallLanding: Position{X: Fixed(r.Before.LandingX) * Pixel, Depth: Fixed(r.Before.LandingDepth) * Pixel}, PursuitOptions: &options}
		got, err := ReactOriginalBall(aiControlFromObservation(r.Before.observedAIControl), view)
		if err != nil {
			t.Fatalf("%s frame %d actor %d: %v", r.Scenario, r.Frame, r.Actor, err)
		}
		wantHeading := HeadingNone
		if r.After.Heading != 255 {
			wantHeading = Heading(r.After.Heading / 32)
		}
		if got.Control != aiControlFromObservation(r.After.observedAIControl) || got.Action != Action(r.After.Action&127) || got.ActionEntered != (r.After.Action&128 != 0) || got.Heading != wantHeading || got.FacingLeft != (r.After.Facing&128 != 0) || got.Random != RandomState(binary.BigEndian.Uint32(r.After.Random[:])) || got.ReactionReady != (r.After.Carry != 0) {
			t.Fatalf("%s frame %d actor %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, r.Actor, r.Before, got, r.After)
		}
		ball, holder := r.Before.BallAction, r.Before.Holder
		if got.Ball.Change {
			ball = uint8(got.Ball.Action)
		}
		if got.BindBall {
			holder = r.Actor
		}
		if ball != r.After.BallAction || holder != r.After.Holder || r.Before.Request != r.After.Request || r.Before.Position != r.After.Position {
			t.Fatal("incorrect reaction effects", r.Frame, got)
		}
	}
	t.Log("matched 1039 original shared ball reactions, including tactical choices, RNG and return status")
}

func TestGroundBallReactionPickupWindow(t *testing.T) {
	s := AIControlState{Mode: 2, Entered: true, Counter: 1}
	v := AIControlView{Actor: 1, Action: ActionIdle, ActionEntered: true, Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}, BallHolder: 6, BallAction: BallRest, BallPosition: Position{X: 109 * Pixel, Depth: 48 * Pixel}}
	got, err := ReactOriginalBall(s, v)
	if err != nil || !got.BindBall || !got.Ball.Change || got.Ball.Action != BallHeld || got.Action != ActionPickUp || !got.ReactionReady {
		t.Fatal(got, err)
	}
	v.BallPosition.X = 110 * Pixel
	got, err = ReactOriginalBall(s, v)
	if err != nil || got.BindBall || got.Action != ActionRun || got.ReactionReady {
		t.Fatal("pickup accepted outside original window", got, err)
	}
	v.BallPosition.Height = Pixel
	got, err = ReactOriginalBall(s, v)
	if err != nil || got.Action != v.Action || got.Control != s || got.BindBall {
		t.Fatal("resting-ball branch ignored nonzero height", got, err)
	}
}

func TestOriginalPursuitParameterTables(t *testing.T) {
	b, err := os.ReadFile("testdata/ai-pursuit-profiles.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Team, Character uint8
		Options         [4]AITacticalOption
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 32 {
		t.Fatal(len(rows))
	}
	for _, r := range rows {
		got, err := OriginalPursuitOptions(r.Team, r.Character)
		if err != nil || got != r.Options {
			t.Fatal(r, got, err)
		}
	}
	if _, err := OriginalPursuitOptions(8, 0); err == nil {
		t.Fatal("invalid team accepted")
	}
	if _, err := OriginalPursuitOptions(0, 4); err == nil {
		t.Fatal("invalid character accepted")
	}
}
