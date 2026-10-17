package engine

func trackOriginalOpponent(result *AIControlDecision, player, target Position) bool {
	dx := int32((player.X >> 8) - (target.X >> 8))
	dz := int32((player.Depth >> 8) - (target.Depth >> 8))
	xButtons := ButtonLeft
	if dx < 0 {
		xButtons = ButtonRight
	}
	var zButtons Buttons
	if absPass(dz) >= 8 {
		zButtons = ButtonDown
		if dz < 0 {
			zButtons = ButtonUp
		}
	}
	// The original reuses its encoded destination byte as this horizontal
	// stopping distance (normally 33 or 35 pixels in defensive mode).
	close := zButtons == 0 && absPass(dx) < int32(result.Control.Destination)
	if zButtons != 0 && absPass(dx) < 16 {
		xButtons = 0
	}
	result.Heading = HeadingForButtons(xButtons | zButtons)
	return close
}

func returnToOriginalDefenceZone(result *AIControlDecision, view AIControlView, zone CourtZone) {
	destination := CourtZone(0x21)
	if view.Actor&1 != 0 {
		destination = 0x23
	}
	result.Control.Destination = destination
	result.Heading = steerOriginalZone(zone, destination, view.Actor)
	if result.Heading != HeadingNone {
		chooseAIAction(result, ActionRun)
		return
	}
	chooseAIAction(result, ActionIdle)
	result.FacingLeft = view.Position.X>>8 >= view.BallPosition.X>>8
	result.Heading = HeadingRight
	if result.FacingLeft {
		result.Heading = HeadingLeft
	}
}

// The original defensive mode returns to a team-specific zone, faces the ball,
// then follows the first threatening opponent. Ball reactions use the same
// restored shared dispatch as the other original tactical modes.
func advanceOriginalDefence(result *AIControlDecision, view AIControlView) error {
	zone, err := originalAIZone(view, view.Actor)
	if err != nil {
		return err
	}
	if zone >= 0x30 {
		returnToOriginalDefenceZone(result, view, zone)
		return nil
	}
	base := (view.Actor & 2) ^ 2
	if view.BallHolder == int8(base) || view.BallHolder == int8(base+1) {
		opponentZone, err := originalAIZone(view, uint8(view.BallHolder))
		if err != nil {
			return err
		}
		if opponentZone >= 0x50 {
			return reactOriginalBall(result, view)
		}
	}
	for enemy := base; enemy < base+2; enemy++ {
		opponentZone, err := originalAIZone(view, enemy)
		if err != nil {
			return err
		}
		if opponentZone < 0x50 {
			continue
		}
		result.Control.TrackedObject = int8(enemy)
		if trackOriginalOpponent(result, view.Position, view.Players[enemy]) {
			result.Control.Mode, result.Control.Entered = 19, false
		} else {
			chooseAIAction(result, ActionRun)
		}
		return nil
	}
	returnToOriginalDefenceZone(result, view, zone)
	return nil
}
