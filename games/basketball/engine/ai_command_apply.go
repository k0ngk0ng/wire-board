package engine

// ApplyOriginalAIActorDecision applies only the original actor commands. It
// performs no input resolution, animation, physics, or object ownership work.
// Both the legacy single-actor integration and the unified input phase use it.
func ApplyOriginalAIActorDecision(original ComputerActorState, decision AIControlDecision) ComputerActorState {
	result := original
	p := &result.Carrier.Player
	result.Control = decision.Control
	if decision.ShotPlan.ChangeJumpPhase {
		result.JumpPhase = decision.ShotPlan.JumpPhase
	}
	if decision.ShotPlan.ChangeShotCycle {
		result.Carrier.ShotCycle = decision.ShotPlan.ShotCycle
	}
	if decision.ShotPlan.ChangeNextAction {
		p.Animation.NextAction, p.Animation.HasNextAction = decision.ShotPlan.NextAction, true
	}
	if decision.ShotPlan.ClearDepthVelocity {
		p.Body.Velocity.Depth = 0
	}
	if decision.Engagement.StopPlanar {
		p.Body.Velocity.X, p.Body.Velocity.Depth = 0, 0
	}
	if decision.Engagement.ChangeNextAction {
		p.Animation.NextAction, p.Animation.HasNextAction = decision.Engagement.NextAction, true
	}
	p.Action, p.Entered, p.Heading, p.FacingLeft = decision.Action, decision.ActionEntered, decision.Heading, decision.FacingLeft
	return result
}
