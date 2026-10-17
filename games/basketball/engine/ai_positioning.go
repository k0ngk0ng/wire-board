package engine

import "errors"

// advanceOriginalForwardPosition is bank 5 $A323. It mirrors the original
// offensive-zone predicates, not the defensive mode's home-side thresholds.
func advanceOriginalForwardPosition(result *AIControlDecision, view AIControlView) error {
	zone, err := originalAIZone(view, view.Actor)
	if err != nil {
		return err
	}
	if zone >= 0x50 {
		base := (view.Actor & 2) ^ 2
		if view.BallHolder == int8(base) || view.BallHolder == int8(base+1) {
			enemyZone, err := originalAIZone(view, uint8(view.BallHolder))
			if err != nil {
				return err
			}
			if enemyZone < 0x30 {
				return reactOriginalBall(result, view)
			}
		}
		for enemy := base; enemy < base+2; enemy++ {
			enemyZone, err := originalAIZone(view, enemy)
			if err != nil {
				return err
			}
			if enemyZone >= 0x30 {
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
	}
	destination := CourtZone(0x51)
	if view.Actor&1 != 0 {
		destination = 0x53
	}
	result.Control.Destination = destination
	result.Heading = steerOriginalZone(zone, destination, view.Actor)
	if result.Heading != HeadingNone {
		chooseAIAction(result, ActionRun)
		return nil
	}
	chooseAIAction(result, ActionIdle)
	result.FacingLeft = view.Position.X>>8 >= view.BallPosition.X>>8
	result.Heading = HeadingRight
	if result.FacingLeft {
		result.Heading = HeadingLeft
	}
	return nil
}

// advanceOriginalApproachDelay restores $A851 (modes 19/39). One tactical
// random choice occurs on entry. The signed countdown decrements immediately;
// below zero it approaches, while nonnegative values preserve the retreat step.
func advanceOriginalApproachDelay(result *AIControlDecision, view AIControlView) error {
	s := &result.Control
	if !s.Entered {
		if view.PursuitOptions == nil {
			return errors.New("original character pursuit options are missing")
		}
		s.Entered = true
		var value byte
		result.Random, value = NextRandom(result.Random)
		option := view.PursuitOptions[(value>>1)&3]
		s.Destination, s.PlanParameter = option.Destination, option.Parameter
		if view.TeamHasHuman {
			s.Counter = 8
		} else {
			delays := [7]int8{12, 10, 8, 6, 4, 2, 0}
			difficulty := int(view.Difficulty >> 2)
			if difficulty >= len(delays) {
				return errors.New("original approach delay difficulty outside table")
			}
			s.Counter = delays[difficulty]
		}
	}
	if s.TrackedObject < 0 || s.TrackedObject >= 4 {
		return errors.New("original approach target is not a player")
	}
	target := view.Players[s.TrackedObject]
	s.Counter--
	if s.Counter < 0 {
		if trackOriginalOpponent(result, view.Position, target) {
			s.Mode, s.Entered = 7, false
		} else {
			chooseAIAction(result, ActionRun)
		}
		return nil
	}
	dz := uint8((view.Position.Depth >> 8) - (target.Depth >> 8))
	if dz >= 16 && dz < 240 {
		s.Mode, s.Entered = s.Plan, false
		return nil
	}
	result.Heading = HeadingLeft
	if view.FacingLeft {
		result.Heading = HeadingRight
	}
	chooseAIAction(result, ActionWalk)
	return nil
}
