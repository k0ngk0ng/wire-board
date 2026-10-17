package engine

import "testing"

// Original bank 6 $8467, REA evidence ev_73c134350e992660959611d967c52b28cd023b86d3ad3a489d197de31de7ef17.
func TestOwnedBallInputBoundaries(t *testing.T) {
	for action := Action(0); action < 128; action++ {
		for _, entered := range []bool{false, true} {
			for _, ballAction := range []BallAction{BallDribble, BallHeld, BallShot} {
				before := Ball{Action: ballAction, Entered: entered, Request: BallRequest{Action: BallHeld, Pending: true}, Animation: AnimationState{Cursor: 3, Elapsed: 2}, Body: Body{Position: Position{X: 101}}}
				want := before
				if action >= ActionDribbleIdle && action <= ActionDribbleBrake {
					if ballAction != BallDribble {
						want.Action, want.Entered = BallDribble, false
					}
				} else {
					want.Request = BallRequest{Action: BallRest, Pending: true}
				}
				owner := Player{Action: action, Entered: entered, Request: ActionRequest{Action: ActionDribbleIdle, Pending: true}}
				if got := StepOwnedBallInput(before, &owner); got != want {
					t.Fatalf("owner %x ball %d: got %+v want %+v", action, ballAction, got, want)
				}
				if got := StepOwnedBallInput(before, nil); got != before {
					t.Fatal("unowned ball changed", got)
				}
			}
		}
	}
}

func TestBallCommandMergesIndependentFields(t *testing.T) {
	request := BallRequest{Action: BallHeld, Pending: true}
	first := BallCommand{ChangeRequest: true, Request: request}
	second := BallCommand{Change: true, Action: BallHeld}
	merged := mergeBallCommands(first, second)
	if !merged.ChangeRequest || merged.Request != request || !merged.Change || merged.Action != BallHeld {
		t.Fatal(merged)
	}
	newer := BallCommand{ChangeRequest: true, Request: BallRequest{Action: BallRest, Pending: true}}
	merged = mergeBallCommands(merged, newer)
	original := Ball{Action: BallDribble, Entered: true, Body: Body{Position: Position{X: 123}}}
	got := ApplyBallCommand(original, merged)
	if got.Action != BallHeld || got.Entered || got.Request != newer.Request || got.Body != original.Body {
		t.Fatal(got)
	}
	if got := ApplyBallCommand(original, BallCommand{}); got != original {
		t.Fatal(got)
	}
}

func TestPossessionPassageKeepsLateRequestUntilNextInput(t *testing.T) {
	position := Position{X: 375 * Pixel, Depth: 80 * Pixel, Height: -192 * Pixel}
	s := PossessionState{HasBall: true, Carrier: Carrier{Player: Player{Action: ActionDribbleIdle, Entered: true, Heading: HeadingNone, NewYorkPassage: 1, Body: Body{Position: position}}}, Ball: Ball{Action: BallHeld, Entered: true}}
	got, err := StepControlledPossession(s, ControllerFrame{}, Position{}, KunioProfile(), NewYorkGroundCourt())
	if err != nil {
		t.Fatal(err)
	}
	if got.Carrier.Player.NewYorkPassage != 0 || got.Carrier.Player.Action != ActionCarrierJump || got.Ball.Request != (BallRequest{Action: BallHeld, Pending: true}) {
		t.Fatal("exit request lost", got)
	}
	next, err := StepControlledPossession(got, ControllerFrame{}, Position{}, KunioProfile(), NewYorkGroundCourt())
	if err != nil {
		t.Fatal(err)
	}
	if next.Ball.Request != (BallRequest{Action: BallRest, Pending: true}) {
		t.Fatal("next input did not clear passage request", next.Ball)
	}
}

func TestComputerCatchMergesInputAndActionCommands(t *testing.T) {
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionCatchBall, Heading: HeadingNone, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}}}}
	decision := AIControlDecision{Action: ActionCatchBall, Heading: HeadingNone, BindHoop: -1, Ball: BallCommand{ChangeRequest: true, Request: BallRequest{Action: BallHeld, Pending: true}}}
	got, err := advanceComputerActorDecision(actor, ComputerActorContext{HasBall: true, BallAction: BallDribble}, decision, KunioProfile(), NewYorkGroundCourt())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Ball.Change || got.Ball.Action != BallHeld || !got.Ball.ChangeRequest || got.Ball.Request != (BallRequest{Action: BallRest, Pending: true}) {
		t.Fatal("input clear or later action lost", got.Ball)
	}
}
