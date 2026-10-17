package engine

import "fmt"

// detachWorldJumpSupport restores bank6 $8B67 before the jump initializer
// clears its attachment. Kind12 follows the rider only when grounded and free.
func detachWorldJumpSupport(s *WorldActionState, actor uint8, c WorldActionContext) error {
	link := s.PlayerLinks[actor]
	if link&128 == 0 {
		return nil
	}
	parent := link & 15
	if parent >= 13 {
		return fmt.Errorf("jump support %d outside original object table", parent)
	}
	support := worldFloorState(*s, c).Objects[parent]
	if support.Kind != 12 || support.Attachment&128 != 0 {
		return nil
	}
	if parent < 4 || parent > 5 {
		return fmt.Errorf("Kind12 jump support %d outside auxiliary slots", parent)
	}
	prop := &s.Auxiliary[parent-4]
	prop.Entered = false
	if support.Flags&128 != 0 {
		prop.Action = 12
		return nil
	}
	prop.Action = 4
	prop.Body.Velocity = s.Actors[actor].Player.Body.Velocity
	prop.Body.Gravity = s.Actors[actor].Player.Body.Gravity
	return nil
}
