package engine

import "testing"

func TestComputerActorAppliesOriginalDecisionCommandsBeforeMotion(t *testing.T) {
	position := Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 20 * Pixel}
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionCatchAir, Entered: true, Airborne: true, Heading: HeadingRight, Body: Body{Position: position, Velocity: Velocity{X: 2 * Pixel, Depth: 3 * Pixel}}}}, JumpPhase: 4}
	decision := AIControlDecision{Action: ActionCatchAir, ActionEntered: true, Heading: HeadingRight, BindHoop: -1, ShotPlan: AIShotCommand{ChangeJumpPhase: true, JumpPhase: -1, ChangeShotCycle: true, ShotCycle: 3, ClearDepthVelocity: true}}
	got, err := advanceComputerActorDecision(actor, ComputerActorContext{HasBall: true, BallAction: BallHeld}, decision, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.JumpPhase != -1 || got.State.Carrier.ShotCycle != 3 || got.State.Carrier.Player.Body.Position.Depth != position.Depth || got.State.Carrier.Player.Body.Position.X != position.X+2*Pixel {
		t.Fatal(got, err)
	}
	decision.Engagement = AIEngagementCommand{StopPlanar: true}
	got, err = advanceComputerActorDecision(actor, ComputerActorContext{HasBall: true, BallAction: BallHeld}, decision, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.Carrier.Player.Body.Position != position {
		t.Fatal(got, err)
	}
}

func TestComputerActorSuppliesShotPlannerWithOwnJumpState(t *testing.T) {
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionCarrierJump, Entered: true, Airborne: true, Heading: HeadingUp, Body: Body{Position: Position{X: 100 * Pixel, Depth: 60 * Pixel, Height: 30 * Pixel}}}}, Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 14}, JumpPhase: 0}
	ctx := ComputerActorContext{Seat: 2, HasBall: true, BallHolder: 2, BallAction: BallHeld, CourtID: 1}
	got, err := AdvanceComputerActor(actor, ctx, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.JumpPhase != -1 || got.State.Carrier.Player.Body.Velocity.Height <= 0 || !got.State.Carrier.Player.Airborne {
		t.Fatal(got, err)
	}
}
