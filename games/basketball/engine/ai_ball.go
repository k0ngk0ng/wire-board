package engine

import "errors"

type AITacticalOption struct {
	Destination CourtZone `json:"destination"`
	Parameter   uint8     `json:"parameter"`
}

type aiApproach struct {
	Reached        bool
	XDistance      int32
	DepthSeparated bool
}

func steerOriginalPoint(result *AIControlDecision, player, target Position, alignX, alignDepth, stopX uint8) aiApproach {
	dx := int32((player.X >> 8) - (target.X >> 8))
	dz := int32((player.Depth >> 8) - (target.Depth >> 8))
	xButtons := ButtonLeft
	if dx < 0 {
		xButtons = ButtonRight
	}
	var zButtons Buttons
	if absPass(dz) >= int32(alignDepth) {
		zButtons = ButtonDown
		if dz < 0 {
			zButtons = ButtonUp
		}
	}
	reached := zButtons == 0 && absPass(dx) < int32(stopX)
	if zButtons != 0 && absPass(dx) < int32(alignX) {
		xButtons = 0
	}
	result.Heading = HeadingForButtons(xButtons | zButtons)
	return aiApproach{Reached: reached, XDistance: absPass(dx), DepthSeparated: zButtons != 0}
}

func originalBallHolderPosition(view AIControlView) (Position, error) {
	if view.BallHolder >= 0 && view.BallHolder < 4 {
		return view.Players[view.BallHolder], nil
	}
	if view.BallHolder == 6 {
		return view.BallPosition, nil
	}
	return Position{}, errors.New("ball holder object is not reconstructed")
}

func requestOriginalBallPickup(result *AIControlDecision) {
	result.Ball = BallCommand{Change: true, Action: BallHeld}
	result.BindBall = true
	chooseAIAction(result, ActionPickUp)
	result.ReactionReady = true
}

// ReactOriginalBall is the shared original AI ball dispatch, independent of
// the calling tactical mode. Binding a pickup is an action command; ball and
// player motion still run in their respective world phases.
func ReactOriginalBall(original AIControlState, view AIControlView) (AIControlDecision, error) {
	initial := AIControlDecision{Control: original, Action: view.Action, ActionEntered: view.ActionEntered, Heading: view.Heading, FacingLeft: view.FacingLeft, Random: view.Random, BindHoop: -1}
	result := initial
	if view.Actor >= 4 {
		return initial, errors.New("invalid ball-reaction actor")
	}
	if err := reactOriginalBall(&result, view); err != nil {
		return initial, err
	}
	return result, nil
}

func reactOriginalBall(result *AIControlDecision, view AIControlView) error {
	s := &result.Control
	result.ReactionReady = false
	switch view.BallAction {
	case BallRest:
		if view.BallPosition.Height>>8 != 0 {
			return nil
		}
		if s.Counter != 1 && s.Counter != 2 {
			s.Counter = 1
			var value byte
			result.Random, value = NextRandom(result.Random)
			if value >= 192 {
				s.Counter = 2
			}
		}
		if s.Counter == 2 {
			if approachOriginalLooseObject(result, view.Position, view.BallPosition) {
				requestOriginalBallPickup(result)
			}
			return nil
		}
		s.Destination = 10
		approach := steerOriginalPoint(result, view.Position, view.BallPosition, 9, 9, 10)
		if approach.Reached {
			requestOriginalBallPickup(result)
		} else if approach.DepthSeparated && approach.XDistance < 16 {
			chooseAIAction(result, ActionWalk)
		} else {
			chooseAIAction(result, ActionRun)
		}
	case BallDribble, BallHeld:
		if s.Counter != -1 {
			s.Counter = -1
			chooseAIAction(result, ActionRun)
			var value byte
			if uint8(view.BallPosition.Height>>8) >= 32 {
				s.Destination = 0x30
				result.Random, value = NextRandom(result.Random)
				choices := [4]uint8{10, 11, 1, 6}
				s.PlanParameter = choices[(value>>1)&3]
			} else {
				if view.PursuitOptions == nil {
					return errors.New("original character pursuit options are missing")
				}
				result.Random, value = NextRandom(result.Random)
				choice := view.PursuitOptions[(value>>1)&3]
				s.Destination, s.PlanParameter = choice.Destination, choice.Parameter
			}
		}
		target, err := originalBallHolderPosition(view)
		if err != nil {
			return err
		}
		approach := steerOriginalPoint(result, view.Position, target, 16, 8, uint8(s.Destination))
		result.ReactionReady = approach.Reached
		if approach.Reached {
			s.TrackedObject = view.BallHolder
			s.Mode, s.Entered = 7, false
		}
	case BallPass:
		s.Destination = 8
		if steerOriginalPoint(result, view.Position, view.BallLanding, 8, 8, 8).Reached {
			chooseAIAction(result, ActionIdle)
		} else {
			chooseAIAction(result, ActionRun)
		}
	case BallRolling, BallStuck, BallAction(10):
		index := view.Actor & 2
		if view.BallFacingLeft {
			index++
		}
		zones := [4]CourtZone{0x72, 0x02, 0x02, 0x72}
		s.Destination = zones[index]
		zone, err := originalAIZone(view, view.Actor)
		if err != nil {
			return err
		}
		result.Heading = steerOriginalZone(zone, s.Destination, view.Actor)
		if result.Heading == HeadingNone {
			chooseAIAction(result, ActionIdle)
		} else {
			chooseAIAction(result, ActionRun)
		}
	default:
		chooseAIAction(result, ActionRun)
		if uint8(s.Counter) < 4 {
			s.Counter++
			return nil
		}
		s.Counter = 0
		s.Destination = 16
		target, err := originalBallHolderPosition(view)
		if err != nil {
			return err
		}
		if !steerOriginalPoint(result, view.Position, target, 16, 8, 16).Reached {
			return nil
		}
		if view.BallPosition.Height>>16 != 0 {
			result.ReactionReady = true
			return nil
		}
		if uint8(view.BallPosition.Height>>8) < 48 {
			return nil
		}
		result.Action, result.ActionEntered = ActionJump, false
		s.Mode, s.Entered = AIRecover, false
		result.Heading = HeadingNone
		result.ReactionReady = true
	}
	return nil
}
