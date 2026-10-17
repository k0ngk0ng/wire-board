package engine

import "errors"

// CourtID is the original court selector ($058B), independent from team number.
type AIDribbleContext struct {
	CourtID uint8
}

// originalDribbleObstacle restores $B15B: inspect opponents in seat order,
// using the actor's facing, not movement heading, for forward/rear distances.
func originalDribbleObstacle(result *AIControlDecision, view AIControlView) bool {
	base := (view.Actor ^ 2) & 2
	for enemy := base; enemy < base+2; enemy++ {
		p := view.Players[enemy]
		depth := uint8((view.Position.Depth >> 8) - (p.Depth >> 8))
		if depth >= 8 && depth < 248 {
			continue
		}
		dx := int32(int16((view.Position.X >> 8) - (p.X >> 8)))
		threshold := int32(16)
		if dx < 0 && !view.FacingLeft || dx >= 0 && view.FacingLeft {
			threshold = 48
		}
		if absPass(dx) < threshold {
			result.Control.PlanParameter = 0
			if threshold == 16 {
				result.Control.PlanParameter = 255
			}
			return true
		}
	}
	return false
}

// advanceOriginalDribbleAttack restores the shared bank-5 $A69F handler for
// modes 16 and 17. Decisions change logical actions only; no physics is run.
func advanceOriginalDribbleAttack(result *AIControlDecision, view AIControlView, ctx AIDribbleContext) error {
	if view.Actor >= 4 {
		return errors.New("invalid original dribble actor")
	}
	s := &result.Control
	mode := func(m AIMode) { s.Mode, s.Entered = m, false }
	if !s.Entered {
		if view.Difficulty>>2 >= 8 {
			return errors.New("original dribble difficulty table index out of range")
		}
		s.Entered = true
		s.Plan = 17
		var value byte
		result.Random, value = NextRandom(result.Random)
		s.Counter = int8([8]uint8{32, 28, 24, 20, 16, 12, 8, 4}[view.Difficulty>>2] + ((value >> 1) & 3))
		chooseAIAction(result, ActionDribbleRun)
		result.Random, value = NextRandom(result.Random)
		buttons := Buttons([8]uint8{9, 9, 9, 1, 5, 5, 5, 1}[value&7])
		if view.Actor >= 2 {
			buttons ^= ButtonRight | ButtonLeft
		}
		result.Heading = HeadingForButtons(buttons)
	}
	if !view.HasBall {
		mode(2)
		return nil
	}
	if result.Action != ActionDribbleRun {
		mode(1)
		return nil
	}
	if !view.TeamHasHuman {
		s.Counter--
		if s.Counter < 0 {
			mode(6)
			return nil
		}
	}
	zone, err := originalAIZone(view, view.Actor)
	if err != nil {
		return err
	}
	if zone >= 0x70 {
		result.Action, result.ActionEntered = ActionDribbleRun, false
		if zone&15 >= 5 && (ctx.CourtID == 3 || ctx.CourtID == 5) {
			result.Heading = Heading(5)
			if view.Actor >= 2 {
				result.Heading = Heading(3)
			}
		} else {
			var value byte
			result.Random, value = NextRandom(result.Random)
			buttons := Buttons([4]uint8{8, 4, 4, 0}[value&3]) | ButtonLeft
			if view.Actor >= 2 {
				buttons ^= ButtonRight | ButtonLeft
			}
			result.Heading = HeadingForButtons(buttons)
		}
		s.Counter = 16
		mode(27)
		s.Plan = 17
	} else if zone >= 0x60 {
		mode(6)
	} else {
		if zone&15 >= 5 && ctx.CourtID == 3 {
			result.Heading = Heading(3)
			if view.Actor >= 2 {
				result.Heading = Heading(5)
			}
		}
		if originalDribbleObstacle(result, view) {
			mode(26)
			s.Plan = 17
		}
	}
	return nil
}
