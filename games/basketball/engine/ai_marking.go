package engine

import "errors"

// advanceOriginalMarking restores bank 5 $A4D4, shared by modes 4 and 12.
// The target is selected once, in original seat order, never by nearest distance.
func advanceOriginalMarking(result *AIControlDecision, view AIControlView) error {
	if view.Actor >= 4 {
		return errors.New("invalid original marking actor")
	}
	s := &result.Control
	if !s.Entered {
		s.Entered = true
		result.Action, result.ActionEntered = ActionRun, false
		s.Counter = 32
		var value byte
		result.Random, value = NextRandom(result.Random)
		s.Plan = s.Mode
		offset := uint8(0)
		if s.Mode == 4 {
			offset = value & 1
		}
		s.TrackedObject = int8(((view.Actor & 2) ^ 2) | offset)
		s.Destination = 48
	}
	if s.TrackedObject < 0 || s.TrackedObject >= 4 {
		return errors.New("original marking target is not a player")
	}
	target := uint8(s.TrackedObject)
	position := view.Players[target]
	action := view.PlayerActions[target]
	if action == 0x49 || action == 0x4a {
		// $AF9C only negates the low distance byte in its negative branch. Keep
		// the original -256 -> 0 wrap, rather than silently replacing it by abs.
		dx := int32(int16((view.Position.X >> 8) - (position.X >> 8)))
		close := false
		if dx >= -256 && dx < 256 {
			result.Heading = HeadingRight
			if dx < 0 {
				result.Heading = HeadingLeft
			}
			zone, err := originalAIZone(view, target)
			if err != nil {
				return err
			}
			if zone&0xf0 == 0 || zone&0xf0 >= 0x70 {
				result.Heading = HeadingRight
				if uint16(position.X>>8)>>8 != 0 {
					result.Heading = HeadingLeft
				}
			}
			distance := uint8(dx)
			if dx < 0 {
				distance = uint8(-dx)
			}
			close = distance < 64
		}
		if close {
			chooseAIAction(result, ActionWalk)
			result.FacingLeft = dx >= 0
		} else {
			// $B1E9 faces the target after the retreat helper returns clear carry.
			result.FacingLeft = uint16(view.Position.X>>8) >= uint16(position.X>>8)
			result.Heading = HeadingRight
			if result.FacingLeft {
				result.Heading = HeadingLeft
			}
			chooseAIAction(result, ActionIdle)
		}
		return nil
	}
	approach := steerOriginalPoint(result, view.Position, position, 16, 8, uint8(s.Destination))
	if approach.Reached {
		s.Mode, s.Entered = 19, false
		if view.TeamHasHuman {
			s.Mode = 39
		}
	} else if approach.XDistance >= 256 || approach.XDistance > int32(s.Destination) {
		chooseAIAction(result, ActionRun)
	} else {
		chooseAIAction(result, ActionWalk)
		result.FacingLeft = view.Position.X>>8 >= position.X>>8
	}
	return nil
}
