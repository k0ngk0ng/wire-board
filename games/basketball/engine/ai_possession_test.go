package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type observedAIPossession struct {
	observedAIControl
	Team, BallAction, BallFacing uint8
	TeamHasHuman                 bool
	Airborne                     bool
	PlayerActions                [4]Action
	LandingX, LandingDepth       int32
	Options                      [4]AITacticalOption
}

func TestOriginalAIPossessionHandoffAndPursuit(t *testing.T) {
	b, err := os.ReadFile("testdata/ai-possession-control.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Actor         uint8
		Frame         int
		Before, After observedAIPossession
	}
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1142 {
		t.Fatal("missing original decisions", len(rows))
	}
	counts := map[uint8]int{}
	for _, r := range rows {
		before, after := r.Before, r.After
		heading := HeadingNone
		if before.Heading != 255 {
			heading = Heading(before.Heading / 32)
		}
		options := before.Options
		v := AIControlView{Actor: r.Actor, Action: Action(before.Action & 127), ActionEntered: before.Action&128 != 0, Heading: heading, FacingLeft: before.Facing&128 != 0, Position: Position{X: Fixed(before.X), Depth: Fixed(before.Depth), Height: Fixed(before.Height)}, HasBall: before.Holder == r.Actor, Players: before.Players, BallPosition: before.Ball, BallHolder: int8(before.Holder), Random: RandomState(binary.BigEndian.Uint32(before.Random[:])), BallAction: BallAction(before.BallAction & 127), BallFacingLeft: before.BallFacing&128 != 0, BallLanding: Position{X: Fixed(before.LandingX) * Pixel, Depth: Fixed(before.LandingDepth) * Pixel}, PursuitOptions: &options, Team: before.Team, TeamHasHuman: before.TeamHasHuman, Airborne: before.Airborne, PlayerActions: before.PlayerActions}
		got, err := StepOriginalAIControl(aiControlFromObservation(before.observedAIControl), v)
		if err != nil {
			t.Fatalf("frame %d actor %d mode %d: %v", r.Frame, r.Actor, before.Mode&127, err)
		}
		wantHeading := HeadingNone
		if after.Heading != 255 {
			wantHeading = Heading(after.Heading / 32)
		}
		if got.Control != aiControlFromObservation(after.observedAIControl) || got.Action != Action(after.Action&127) || got.ActionEntered != (after.Action&128 != 0) || got.Heading != wantHeading || got.FacingLeft != (after.Facing&128 != 0) || got.Random != RandomState(binary.BigEndian.Uint32(after.Random[:])) {
			t.Fatalf("frame %d actor %d\nbefore %+v\ngot %+v\nwant %+v", r.Frame, r.Actor, before, got, after)
		}
		holder, ball := before.Holder, before.BallAction
		if got.BindBall {
			holder = r.Actor
		}
		if got.Ball.Change {
			ball = uint8(got.Ball.Action)
		}
		if holder != after.Holder || ball != after.BallAction || got.BindHoop != -1 || before.Request != after.Request || before.X != after.X || before.Depth != after.Depth || before.Height != after.Height {
			t.Fatalf("unexpected world mutation at %d: %+v", r.Frame, got)
		}
		counts[before.Mode&127]++
	}
	t.Logf("matched %d original AI calls: %v", len(rows), counts)
}

func TestOriginalPossessionWaitBranches(t *testing.T) {
	s := AIControlState{Mode: 1, Plan: 6}
	v := AIControlView{Actor: 2, HasBall: true, Action: ActionDribbleIdle, ActionEntered: true, Airborne: true}
	got, err := StepOriginalAIControl(s, v)
	if err != nil || got.Action != ActionCarrierJump || got.ActionEntered || got.Control.Mode != 1 || !got.Control.Entered {
		t.Fatal(got, err)
	}
	v.Action = ActionDribbleRun
	got, err = StepOriginalAIControl(s, v)
	if err != nil || got.Action != ActionDribbleRun || got.Control.Mode != 1 {
		t.Fatal(got, err)
	}
	v.Airborne = false
	got, err = StepOriginalAIControl(s, v)
	if err != nil || got.Control.Mode != 6 || got.Control.Entered {
		t.Fatal(got, err)
	}
	v.HasBall = false
	got, err = StepOriginalAIControl(s, v)
	if err != nil || got.Control.Mode != AIRecover || got.Control.Entered {
		t.Fatal(got, err)
	}
	s.Plan = 33
	v.HasBall = true
	v.Action = ActionCatchAir
	v.Team = 7
	v.TeamHasHuman = false
	got, err = StepOriginalAIControl(s, v)
	if err != nil || got.Control.Mode != 33 || got.Control.Entered {
		t.Fatal(got, err)
	}
	v.TeamHasHuman = true
	got, err = StepOriginalAIControl(s, v)
	if err != nil || got.Control.Mode != 1 || !got.Control.Entered {
		t.Fatal(got, err)
	}
}

func TestComputerActorPassesPursuitTeamContext(t *testing.T) {
	// This fixed rule integration distinguishes human-team ball pursuit from
	// the all-computer opponent-search path; it is not an original full replay.
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionIdle, Entered: true, Heading: HeadingNone, Body: Body{Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}}}}, Control: AIControlState{Mode: 2}}
	ctx := ComputerActorContext{Seat: 1, TeamHasHuman: true, BallHolder: 6, BallAction: BallPass, BallLanding: Position{X: 200 * Pixel, Depth: 40 * Pixel}}
	ctx.PlayerActions[2] = ActionLand // The alternate path would hit the original index quirk.
	got, err := AdvanceComputerActor(actor, ctx, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.Carrier.Player.Action != ActionRun || got.State.Carrier.Player.Heading != HeadingRight || got.State.Control.Mode != 2 {
		t.Fatal(got, err)
	}
	ctx.TeamHasHuman = false
	got, err = AdvanceComputerActor(actor, ctx, KunioProfile(), NewYorkGroundCourt())
	if err == nil || got.State != actor {
		t.Fatal("unrepresented original index did not fail atomically", got, err)
	}
}

func TestComputerActorPassesPossessionTeamAndAirState(t *testing.T) {
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionCatchAir, Entered: true, Airborne: true, Heading: HeadingRight, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 20 * Pixel}}}}, Control: AIControlState{Mode: 1, Plan: 33}}
	ctx := ComputerActorContext{Seat: 2, HasBall: true, BallHolder: 2, BallAction: BallHeld, Team: 7}
	got, err := AdvanceComputerActor(actor, ctx, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.Control.Mode != 33 {
		t.Fatal(got, err)
	}
	ctx.TeamHasHuman = true
	got, err = AdvanceComputerActor(actor, ctx, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.Control.Mode != 1 {
		t.Fatal(got, err)
	}
	actor.Carrier.Player.Action = ActionDribbleIdle
	actor.Carrier.Player.Entered = true
	got, err = AdvanceComputerActor(actor, ctx, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.Control.Mode != 1 || got.State.Carrier.Player.Action != ActionCarrierJump {
		t.Fatal("airborne ground pose failed to enter original jump", got, err)
	}
}
