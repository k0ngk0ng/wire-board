package engine

import (
	"errors"
	"fmt"
)

type AIEvasionContext struct {
	Court uint8
}

// Bank 5 $BA5C: first four choices are used for a negative prior plan
// parameter, the remaining eight for an ordinary approach. Human teammates
// always use team zero's original table.
var originalDribbleEvasionChoices = [8][12]uint8{
	{0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1},
	{1, 0, 2, 0, 4, 0, 5, 2, 4, 0, 5, 1},
	{2, 2, 0, 0, 2, 2, 11, 11, 2, 3, 3, 0},
	{1, 1, 1, 3, 4, 5, 5, 5, 0, 0, 4, 1},
	{1, 1, 1, 1, 0, 2, 2, 4, 4, 5, 0, 0},
	{1, 1, 1, 1, 0, 0, 5, 5, 3, 3, 2, 2},
	{0, 0, 3, 3, 3, 3, 4, 4, 1, 1, 5, 5},
	{1, 0, 4, 5, 1, 0, 4, 5, 1, 0, 4, 5},
}

func originalDribbleEscapeHeading(zone CourtZone, actor uint8) Heading {
	up := zone&7 == 0 || zone&7 == 1 || zone&7 == 3
	if actor < 2 {
		if up {
			return HeadingUpRight
		}
		return HeadingDownRight
	}
	if up {
		return HeadingUpLeft
	}
	return HeadingDownLeft
}

// advanceOriginalDribbleEvasion restores $A8DE and the recovered $B7CD
// choices. It requests original player actions; action physics and hit
// detection remain separate, and no evasive movement is fabricated here.
func advanceOriginalDribbleEvasion(result *AIControlDecision, view AIControlView, ctx AIEvasionContext) (err error) {
	initial := *result
	defer func() {
		if err != nil {
			*result = initial
		}
	}()
	zone, err := originalAIZone(view, view.Actor)
	if err != nil {
		return err
	}
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		if ctx.Court == 1 && zone&15 >= 5 {
			s.PlanParameter = 1
		} else {
			s.Counter = 0
			team := view.Team
			if view.TeamHasHuman {
				team = 0
			}
			if team >= 8 {
				return errors.New("invalid original AI evasion team")
			}
			var value byte
			result.Random, value = NextRandom(result.Random)
			index := value & 3
			if s.PlanParameter&128 == 0 {
				index = (value & 7) + 4
			}
			s.PlanParameter = originalDribbleEvasionChoices[team][index]
		}
	}
	switch s.PlanParameter {
	case 0:
		// $2E has its own original player action. This decision does not claim
		// that its animation/contact handler is already restored.
		result.Action, result.ActionEntered = Action(0x2e), false
		result.Heading = HeadingUp
		if zone&15 == 2 || zone&15 >= 4 {
			result.Heading = HeadingDown
		}
		s.Mode, s.Entered = AIPossessionWait, false
		s.Plan = 17
	case 1:
		result.Action, result.ActionEntered = ActionDribbleRun, false
		result.Heading = originalDribbleEscapeHeading(zone, view.Actor)
		s.Mode, s.Entered = AIPause, false
		s.Plan = 17
	case 2:
		// B840 measures the original 16-bit integer X distance to the
		// teammate. A near teammate changes only the next choice; plan5
		// executes on a later AI call, never as a same-frame fallthrough.
		distance := int32(int16(uint16(view.Position.X>>8) - uint16(view.Players[view.Actor^1].X>>8)))
		if absPass(distance) < 96 {
			s.PlanParameter = 5
		} else {
			result.Action, result.ActionEntered = ActionPass, false
			s.Mode, s.Entered = AIPossessionWait, false
		}
	case 3:
		s.Mode, s.Entered = 6, false
	case 4:
		result.Action, result.ActionEntered = ActionCarrierRunningJump, false
		result.Heading = HeadingRight
		if view.Actor >= 2 {
			result.Heading = HeadingLeft
		}
		result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
		s.Mode, s.Entered = AIPossessionWait, false
		s.Plan = 17
	case 5:
		result.Action, result.ActionEntered = ActionCarrierRunningJump, false
		result.Heading = originalDribbleEscapeHeading(zone, view.Actor)
		result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
		s.Mode, s.Entered = AIPossessionWait, false
		s.Plan = 17
	case 6:
		// B8D8 consumes a random byte only at counter zero. A8DE's
		// ordinary random-choice table does not select this direct plan.
		if s.Counter == 0 {
			chooseAIAction(result, Action(0x2e))
			var value uint8
			result.Random, value = NextRandom(result.Random)
			depth := zone & 15
			if depth >= 8 {
				return errors.New("direct evasion6 zone threshold alias not reconstructed")
			}
			result.Heading = HeadingDown
			if value >= [8]uint8{64, 64, 128, 192, 192, 128, 128, 128}[depth] {
				result.Heading = HeadingUp
			}
			s.Counter++
			return nil
		}
		if zone&0xf0 >= 0x70 {
			s.Mode, s.Entered = AIPause, false
			return nil
		}
		if result.Action == Action(0x2e) {
			return nil
		}
		chooseAIAction(result, ActionDribbleRun)
		result.Heading = HeadingRight
		if view.Actor >= 2 {
			result.Heading = HeadingLeft
		}
		if uint8(s.Counter) >= 8 {
			s.Plan = 17
		} else {
			s.Counter++
		}
	case 7, 8, 9:
		// B92C compares the original high vertical-speed byte unsigned.
		if view.ShotContext.JumpPhase < 0 && uint8(view.ShotContext.VerticalSpeed) >= 2 {
			return nil
		}
		result.Heading = HeadingDown
		if s.PlanParameter == 7 {
			result.Heading = HeadingUpRight
			if view.Actor >= 2 {
				result.Heading = HeadingUpLeft
			}
		} else if s.PlanParameter == 8 {
			result.Heading = HeadingRight
			if view.Actor >= 2 {
				result.Heading = HeadingLeft
			}
		}
		if view.ShotContext.JumpPhase >= 0 {
			result.Action, result.ActionEntered = ActionCarrierRunningJump, false
			result.ShotPlan.ChangeJumpPhase, result.ShotPlan.JumpPhase = true, -1
		} else {
			result.Action, result.ActionEntered = ActionCarrierJump, false
			s.Mode, s.Entered, s.Plan = AIPossessionWait, false, 17
		}
	case 10:
		if result.Action != ActionCarrierJump {
			result.Action, result.ActionEntered = ActionCarrierJump, false
			result.Heading = HeadingNone
			s.Mode, s.Entered, s.Plan = AIPossessionWait, false, 17
		}
	case 12:
		chooseAIAction(result, ActionDribbleRun)
		s.Counter++
		if uint8(s.Counter)&7 == 0 {
			// XOR80 maps FF to raw7F, which has no Heading representation.
			// Do not silently reinterpret that byte as a diagonal direction.
			if result.Heading >= HeadingNone {
				return errors.New("direct evasion12 raw heading flip not reconstructed")
			}
			result.Heading = Heading((uint8(result.Heading) + 4) & 7)
		}
		if uint8(s.Counter) >= 32 {
			s.Mode, s.Entered = 17, false
		}
	case 13:
		if result.Action != ActionDribbleIdle {
			result.Action, result.ActionEntered = ActionDribbleIdle, false
			s.Counter = 0
		} else {
			s.Counter++
			if uint8(s.Counter) >= 8 {
				result.Action, result.ActionEntered = ActionDribbleRun, false
				s.Mode, s.Entered, s.Plan = AIPause, false, 17
			}
		}
	case 11:
		// $B97B is a short vertical walking interval. It uses the original
		// six-byte zone-depth heading table without mirroring by team.
		if result.Action != ActionDribbleWalk {
			s.Counter = 8
			result.Heading = [6]Heading{HeadingUp, HeadingDown, HeadingDown, HeadingUp, HeadingUp, HeadingDown}[zone&15]
			result.Action, result.ActionEntered = ActionDribbleWalk, false
		} else {
			s.Counter--
			if s.Counter < 0 {
				s.Mode, s.Entered, s.Plan = AIPossessionWait, false, 17
			}
		}
	default:
		return fmt.Errorf("original AI dribble-evasion branch %d not reconstructed", s.PlanParameter)
	}
	return nil
}
