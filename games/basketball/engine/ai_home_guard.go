package engine

// advanceOriginalHomeGuard restores bank5 A229 (personal mode5). It returns to
// zone22 and delegates only the original threatening-ball cases to BACC.
func advanceOriginalHomeGuard(result *AIControlDecision, view AIControlView) error {
	s := &result.Control
	if !s.Entered {
		s.Entered, s.Counter, s.Plan = true, 0, 5
	}
	zone, err := originalAIZone(view, view.Actor)
	if err != nil {
		return err
	}
	if zone < 0x30 {
		if uint8(view.BallHolder) >= 4 {
			x := uint16(view.BallPosition.X >> 8)
			if uint8(x>>8) == view.Actor/2 && (view.Actor < 2 && uint8(x) < 0xa0 || view.Actor >= 2 && uint8(x) >= 0x50) {
				return reactOriginalBall(result, view)
			}
		}
		opponent := (view.Actor & 2) ^ 2
		if view.BallHolder == int8(opponent) || view.BallHolder == int8(opponent+1) {
			otherZone, err := originalAIZone(view, uint8(view.BallHolder))
			if err != nil {
				return err
			}
			if otherZone >= 0x50 {
				return reactOriginalBall(result, view)
			}
			s.Counter = 0
		}
	}
	s.Destination = 0x22
	result.Heading = steerOriginalZone(zone, s.Destination, view.Actor)
	if result.Heading != HeadingNone {
		chooseAIAction(result, ActionRun)
		return nil
	}
	chooseAIAction(result, ActionIdle)
	target, err := originalBallHolderPosition(view)
	if err != nil {
		return err
	}
	result.FacingLeft = uint16(view.Position.X>>8) >= uint16(target.X>>8)
	result.Heading = HeadingRight
	if result.FacingLeft {
		result.Heading = HeadingLeft
	}
	return nil
}
