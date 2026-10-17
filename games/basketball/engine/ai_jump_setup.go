package engine

// advanceOriginalJumpShotSetup restores bank 5 $A7A3. It emits the original
// movement commands; action physics applies them after all AI decisions.
func advanceOriginalJumpShotSetup(result *AIControlDecision, view AIControlView) error {
	zone, err := originalAIZone(view, view.Actor)
	if err != nil {
		return err
	}
	depth := uint8(zone & 15)
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		result.Action = [6]Action{ActionCarrierJump, ActionDribbleWalk, ActionCarrierJump, ActionDribbleWalk, ActionCarrierJump, ActionCarrierJump}[depth]
		result.ActionEntered = false
		result.Heading = [6]Heading{HeadingUp, HeadingUp, HeadingNone, HeadingDown, HeadingDown, HeadingDown}[depth]
		s.Destination = [6]CourtZone{1, 0, 0, 0, 1, 1}[depth]
		result.ShotPlan.ClearDepthVelocity = true
		result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
	}
	if result.Action == ActionDribbleWalk {
		if depth == 2 {
			result.Action, result.ActionEntered = ActionCarrierJump, false
			result.Heading = HeadingNone
			result.ShotPlan.ClearDepthVelocity = true
		}
		return nil
	}
	switch s.Destination {
	case 255:
		result.Action, result.ActionEntered = ActionAimAirShot, false
		s.Mode, s.Entered = AIPossessionWait, false
	case 1:
		if depth == 2 {
			result.Heading = HeadingRight
			if view.Actor >= 2 {
				result.Heading = HeadingLeft
			}
			result.Action, result.ActionEntered = ActionCarrierJump, false
			s.Destination = 255
			result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
		}
	default:
		if view.ShotContext.VerticalSpeed < 0 {
			s.Destination = 1
		}
	}
	return nil
}
