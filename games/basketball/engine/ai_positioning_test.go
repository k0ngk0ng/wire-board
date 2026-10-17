package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalAIForwardPositionAndApproachDelay(t *testing.T) {
	b, err := os.ReadFile("testdata/ai-positioning.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After struct {
			observedAIPossession
			Difficulty uint8
		}
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 235 {
		t.Fatal(len(rows))
	}
	counts := map[uint8]int{}
	for _, r := range rows {
		b, a := r.Before, r.After
		heading := HeadingNone
		if b.Heading != 255 {
			heading = Heading(b.Heading / 32)
		}
		options := b.Options
		v := AIControlView{Actor: r.Actor, Action: Action(b.Action & 127), ActionEntered: b.Action&128 != 0, Heading: heading, FacingLeft: b.Facing&128 != 0, Position: Position{X: Fixed(b.X), Depth: Fixed(b.Depth), Height: Fixed(b.Height)}, HasBall: b.Holder == r.Actor, Players: b.Players, BallPosition: b.Ball, BallHolder: int8(b.Holder), Random: RandomState(binary.BigEndian.Uint32(b.Random[:])), BallAction: BallAction(b.BallAction & 127), BallFacingLeft: b.BallFacing&128 != 0, BallLanding: Position{X: Fixed(b.LandingX) * Pixel, Depth: Fixed(b.LandingDepth) * Pixel}, PursuitOptions: &options, Team: b.Team, TeamHasHuman: b.TeamHasHuman, Airborne: b.Airborne, PlayerActions: b.PlayerActions, Difficulty: b.Difficulty}
		got, err := StepOriginalAIControl(aiControlFromObservation(b.observedAIControl), v)
		if err != nil {
			t.Fatalf("frame %d actor %d mode %d: %v", r.Frame, r.Actor, b.Mode&127, err)
		}
		wantHeading := HeadingNone
		if a.Heading != 255 {
			wantHeading = Heading(a.Heading / 32)
		}
		if got.Control != aiControlFromObservation(a.observedAIControl) || got.Action != Action(a.Action&127) || got.ActionEntered != (a.Action&128 != 0) || got.Heading != wantHeading || got.FacingLeft != (a.Facing&128 != 0) || got.Random != RandomState(binary.BigEndian.Uint32(a.Random[:])) {
			t.Fatalf("frame %d actor %d\nbefore %+v\ngot %+v\nwant %+v", r.Frame, r.Actor, b, got, a)
		}
		holder, ball := b.Holder, b.BallAction
		if got.BindBall {
			holder = r.Actor
		}
		if got.Ball.Change {
			ball = uint8(got.Ball.Action)
		}
		if holder != a.Holder || ball != a.BallAction || got.BindHoop != -1 || b.Request != a.Request || b.X != a.X || b.Depth != a.Depth || b.Height != a.Height {
			t.Fatal("world mutation", r.Frame, got)
		}
		counts[b.Mode&127]++
	}
	t.Logf("matched %d original positioning/delay calls: %v", len(rows), counts)
}

func TestOriginalApproachDelayBoundaries(t *testing.T) {
	options := [4]AITacticalOption{{Destination: 32}, {Destination: 32}, {Destination: 32}, {Destination: 32}}
	base := AIControlView{Actor: 2, Action: ActionRun, Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, PursuitOptions: &options}
	base.Players[0] = Position{X: 210 * Pixel, Depth: 40 * Pixel}
	s := AIControlState{Mode: 19, TrackedObject: 0, Plan: 8}
	for difficulty, want := range []int8{11, 9, 7, 5, 3, 1, -1} {
		v := base
		v.Difficulty = uint8(difficulty * 4)
		got, err := StepOriginalAIControl(s, v)
		if err != nil || got.Control.Counter != want {
			t.Fatal(difficulty, got, err)
		}
		if difficulty == 6 && got.Control.Mode != 7 {
			t.Fatal("zero delay did not approach immediately", got)
		}
	}
	base.TeamHasHuman = true
	got, err := StepOriginalAIControl(s, base)
	if err != nil || got.Control.Counter != 7 || got.Heading != HeadingLeft || got.Action != ActionWalk {
		t.Fatal(got, err)
	}
	s.Entered = true
	s.Counter = 2
	for _, tc := range []struct {
		delta Fixed
		mode  AIMode
	}{{-16, 19}, {15, 19}, {16, 8}, {-17, 8}} {
		v := base
		v.Position.Depth = v.Players[0].Depth + tc.delta*Pixel
		got, err := StepOriginalAIControl(s, v)
		if err != nil || got.Control.Mode != tc.mode {
			t.Fatal(tc, got, err)
		}
	}
}
