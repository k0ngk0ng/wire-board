package engine

import "testing"

func TestApplyOriginalAIActorDecisionDoesNotRunPhysics(t *testing.T) {
	position := Position{X: 12345, Depth: 6789, Height: 1357}
	s := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionRun, Entered: true, Body: Body{Position: position, Velocity: Velocity{X: 111, Depth: 222, Height: 333}, Gravity: 17}, Animation: AnimationState{Cursor: 2, Elapsed: 4}}, ShotCycle: 6, AimWindow: 29}, JumpPhase: 3, Dodge: DodgeMemory{Step: 2, InitialXHigh: -4}}
	d := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, Plan: 17}, Action: ActionCarrierJump, Heading: HeadingLeft, FacingLeft: true, BindHoop: -1, ShotPlan: AIShotCommand{ChangeJumpPhase: true, JumpPhase: -1, ChangeShotCycle: true, ShotCycle: 2, ClearDepthVelocity: true}, Engagement: AIEngagementCommand{ChangeNextAction: true, NextAction: ActionAimShot}}
	got := ApplyOriginalAIActorDecision(s, d)
	if got.Carrier.Player.Body.Position != position || got.Carrier.Player.Body.Velocity != (Velocity{X: 111, Height: 333}) || got.Carrier.Player.Body.Gravity != 17 || got.Carrier.Player.Animation.Cursor != 2 || got.Carrier.Player.Animation.Elapsed != 4 || got.Carrier.Player.Animation.NextAction != ActionAimShot || !got.Carrier.Player.Animation.HasNextAction || got.Carrier.Player.Action != ActionCarrierJump || got.Carrier.Player.Entered || got.Carrier.Player.Heading != HeadingLeft || !got.Carrier.Player.FacingLeft || got.Carrier.AimWindow != 29 || got.JumpPhase != -1 || got.Carrier.ShotCycle != 2 || got.Control != d.Control || got.Dodge != s.Dodge {
		t.Fatal(got)
	}
	d.Engagement.StopPlanar = true
	got = ApplyOriginalAIActorDecision(s, d)
	if got.Carrier.Player.Body.Velocity != (Velocity{Height: 333}) || got.Carrier.Player.Body.Position != position {
		t.Fatal("decision command ran physical motion", got)
	}
}
