package engine

import "fmt"

// AIAuxiliaryControlCommand records only writes made by the AI mode. Binding
// changes the selected auxiliary object's owner; action/attachment/physics are
// left for their ordinary world stages. Message is the actor's 06C6 slot.
type AIAuxiliaryControlCommand struct {
	Bind       bool
	Object     uint8
	SetMessage bool
	Message    uint8
}

// originalAvailableAuxiliary is AD78: first free, inactive, ground-level
// auxiliary object, preserving the original 4 then 5 search order. Owner FF
// is unowned, while any byte >=4 is not a player owner.
func originalAvailableAuxiliary(aux [2]AuxiliaryState) (uint8, bool) {
	for i, o := range aux {
		if uint8(o.Owner) >= 4 && o.Action == 0 && uint16(o.Body.Position.Height>>8) == 0 {
			return uint8(i + 4), true
		}
	}
	return 0, false
}

// advanceOriginalAuxiliaryCooperation restores A557, A187 and A599. These
// modes request the source pickup/carry/support actions; the world owns motion
// and attachment. No replacement target search or teammate strategy is added.
func advanceOriginalAuxiliaryCooperation(result *AIControlDecision, view AIControlView) error {
	if view.Actor >= 4 || view.Auxiliary == nil {
		return fmt.Errorf("auxiliary cooperation requires actor and auxiliary world state")
	}
	s := &result.Control
	switch s.Mode {
	case 3:
		s.Entered = true
		owned := false
		for _, o := range *view.Auxiliary {
			owned = owned || o.Owner == int8(view.Actor)
		}
		if !owned {
			s.Mode, s.Entered, s.Plan = 0, false, 8
			return nil
		}
		if result.Action == Action(0x20) {
			s.Mode = AIMode(uint8(s.Plan) & 127)
			s.Entered = uint8(s.Plan)&128 != 0
		}
	case 13:
		s.Entered = true
		s.Destination = 0x10
		object, ok := originalAvailableAuxiliary(*view.Auxiliary)
		if !ok {
			result.ClearCommandStatus = true
			result.AuxiliaryCommand.SetMessage, result.AuxiliaryCommand.Message = true, 2
			result.Action, result.ActionEntered = ActionIdle, false
			return nil
		}
		target := view.Auxiliary[object-4].Body.Position
		dx := int32(int16(uint16(view.Position.X>>8) - uint16(target.X>>8)))
		reached := false
		if absPass(dx) < 9 {
			depth := uint8(uint8(view.Position.Depth>>8) - uint8(target.Depth>>8))
			reached = depth >= 248 || depth < 8
			if !reached {
				result.Heading = HeadingDown
				if uint8(view.Position.Depth>>8) < uint8(target.Depth>>8) {
					result.Heading = HeadingUp
				}
				chooseAIAction(result, ActionWalk)
			}
		} else {
			result.Heading = HeadingDownLeft
			if dx < 0 {
				result.Heading = HeadingDownRight
			}
			chooseAIAction(result, ActionRun)
		}
		if reached {
			result.AuxiliaryCommand.Bind, result.AuxiliaryCommand.Object = true, object
			result.Action, result.ActionEntered = Action(0x4e), false
			s.Mode, s.Entered, s.Plan = 3, false, 14
		}
	case 14:
		if !s.Entered {
			s.Entered = true
			result.Action, result.ActionEntered = Action(0x22), false
			s.Destination = 0x30
		}
		finish := func() { s.Mode, s.Entered, s.Plan = 0, false, 22 }
		aux := *view.Auxiliary
		if uint8(aux[0].Owner)&uint8(aux[1].Owner) == 255 {
			finish()
			return nil
		}
		object := uint8(5)
		if aux[0].Owner == int8(view.Actor) {
			object = 4
		}
		if view.TeammateLink&128 != 0 && view.TeammateLink&15 == object {
			result.Heading = HeadingRight
			if view.Actor >= 2 {
				result.Heading = HeadingLeft
			}
			result.Action, result.ActionEntered = Action(0x2b), false
			finish()
			return nil
		}
		target := view.Actor ^ 2
		if uint8(view.BallHolder) < 4 {
			target = (view.Actor & 2) ^ 2
			if int8(target) != view.BallHolder && view.PlayerActions[target] == Action(0x4a) {
				target++
				if int8(target) != view.BallHolder && view.PlayerActions[target] == Action(0x4a) {
					target = view.Actor ^ 2
				}
			}
		}
		s.TrackedObject = int8(target)
		position := view.Players[target]
		approachOriginalRiderHolder(result, view, position)
		dx := int32(int16(uint16(view.Position.X>>8) - uint16(position.X>>8)))
		dz := int32(int8(uint8(view.Position.Depth>>8) - uint8(position.Depth>>8)))
		// A617 checks the high byte of absolute X distance, not B05C's carry.
		if absPass(dx) < 256 && absPass(dz) < 8 && s.Destination < 0x50 {
			result.FacingLeft = uint16(view.Position.X>>8) >= uint16(position.X>>8)
			result.Heading = HeadingRight
			if result.FacingLeft {
				result.Heading = HeadingLeft
			}
			result.Action, result.ActionEntered = Action(0x27), false
			finish()
		}
	default:
		return fmt.Errorf("auxiliary cooperation mode %d outside restored handlers", s.Mode)
	}
	return nil
}
