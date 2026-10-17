package engine

import "errors"

// ADA4's approach is deliberately asymmetric. It first closes X with a
// downward diagonal, then adjusts only depth within the narrow X window.
func approachOriginalSupport(result *AIControlDecision, view AIControlView, target Position) bool {
	dx := int32(int16(uint16(view.Position.X>>8) - uint16(target.X>>8)))
	if absPass(dx) < 9 {
		depth := uint8(uint8(view.Position.Depth>>8) - uint8(target.Depth>>8))
		if depth >= 252 || depth < 8 {
			return true
		}
		result.Heading = HeadingDown
		if uint8(view.Position.Depth>>8) < uint8(target.Depth>>8) {
			result.Heading = HeadingUp
		}
		chooseAIAction(result, ActionWalk)
		return false
	}
	result.Heading = HeadingDownLeft
	if dx < 0 {
		result.Heading = HeadingDownRight
	}
	chooseAIAction(result, ActionRun)
	return false
}

// B05C uses signed 16-bit X and signed-byte depth differences and leaves a
// horizontal heading even when it reports that the close window is reached.
func approachOriginalRiderHolder(result *AIControlDecision, view AIControlView, target Position) bool {
	dx := int32(int16(uint16(view.Position.X>>8) - uint16(target.X>>8)))
	dz := int32(int8(uint8(view.Position.Depth>>8) - uint8(target.Depth>>8)))
	horizontal := ButtonLeft
	if dx < 0 {
		horizontal = ButtonRight
	}
	var depth Buttons
	if absPass(dz) >= 8 {
		depth = ButtonDown
		if dz < 0 {
			depth = ButtonUp
		}
	}
	distance := absPass(dx)
	if depth != 0 && distance < 16 {
		horizontal = 0
	}
	result.Heading = HeadingForButtons(horizontal | depth)
	return depth == 0 && distance < int32(result.Control.Destination)
}

func advanceOriginalRiderApproach(result *AIControlDecision, view AIControlView) (err error) {
	initial := *result
	defer func() {
		if err != nil {
			*result = initial
		}
	}()
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		s.Destination = 4
		s.TrackedObject = int8(view.Actor ^ 1)
	}
	jump := func() {
		result.Engagement.StopPlanar = true
		result.Heading = HeadingNone
		result.Action, result.ActionEntered = ActionJump, false
	}
	switch result.Action {
	case ActionRiding:
		return nil
	case ActionJump:
		grade, e := originalShotJumpGrade(view.ShotContext.JumpVelocity)
		if e != nil {
			return e
		}
		if grade < 4 && view.ShotContext.JumpPhase >= 0 && uint8(view.ShotContext.VerticalSpeed) == 0 {
			result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
			jump()
			return nil
		}
		position, e := originalBallHolderPosition(view)
		if e != nil {
			return e
		}
		result.FacingLeft = uint16(view.Position.X>>8) >= uint16(position.X>>8)
		result.Heading = HeadingRight
		if result.FacingLeft {
			result.Heading = HeadingLeft
		}
		return nil
	case Action(0x3f):
		s.Destination = 16
		position, e := originalBallHolderPosition(view)
		if e != nil {
			return e
		}
		if approachOriginalRiderHolder(result, view, position) {
			result.Action, result.ActionEntered = Action(0x65), false
			s.Mode, s.Entered = AIRecover, false
		}
		return nil
	}
	if s.TrackedObject < 0 || s.TrackedObject >= 4 {
		return errors.New("original rider approach requires its tracked player")
	}
	if approachOriginalSupport(result, view, view.Players[s.TrackedObject]) {
		jump()
		return nil
	}
	if !view.Airborne {
		zone, e := originalAIZone(view, view.Actor)
		if e != nil {
			return e
		}
		if zone&15 >= 5 {
			result.Heading = HeadingDown
			chooseAIAction(result, ActionWalk)
		}
	}
	return nil
}
