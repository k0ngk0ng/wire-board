package engine

import "errors"

// StepFighter2Motion recovers source2 slots1e/1f/20 and shared0f/11 through
// the15afc boundary. Children are initialized here and stepped by the caller.
// Gravity's high word is the scatter cursor; no additional storage is kept.
func StepFighter2Motion(state ChuSpecialMotionState, parameters JumpParameters) (ChuSpecialMotionState, error) {
	original := state
	body := &state.Body
	switch body.ActionKind {
	case 0x0f:
		return StepChuSpecialMotion(state, parameters)
	case 0x11:
		view, err := StepFighter10Motion(Fighter10MotionState{ObjectID: state.ObjectID, Body: state.Body, Controls: state.Controls,
			Aux48: state.AlternateVelocity, ContactFlags: state.ContactFlags, Pool: state.Pool}, parameters)
		if err != nil {
			return original, err
		}
		state.Body, state.Controls, state.Pool = view.Body, view.Controls, view.Pool
		state.AlternateVelocity, state.ContactFlags = view.Aux48, view.ContactFlags
	case 0x1e:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex != 4 || body.Clock.Ticks != 0 {
			break
		}
		child := &state.Children[0]
		if child.Size != 0 {
			break
		}
		initializeFighter0Child(child, Fighter0MotionState{ChuSpecialMotionState: state}, 0x12dd4, 0x60, 252)
		dx := int16(body.VelocityX >> 16)
		if body.Facing != 0 {
			dx = -dx
		}
		child.X, child.Y = addThrowCoordinateWord(body.X, dx), body.Y
		child.Facing, child.Pose, child.TargetMask = body.Facing, 12, 0xffffffff
		// The original leaves both old child velocities and Power untouched.
	case 0x1f:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex < 4 {
			break
		}
		var displacement int32
		switch body.Clock.FrameIndex {
		case 4, 5:
			if body.Clock.Ticks&3 == 0 {
				if err := spawnChuSpecialTrail(&state); err != nil {
					return original, err
				}
			}
			displacement = 0x40000
		case 6:
			displacement = 0x20000
		case 7:
			displacement = 0x10000
		}
		if body.Facing != 0 {
			displacement = -displacement
		}
		body.X += displacement
		if body.Clock.FrameIndex == 4 && state.ContactFlags&1 != 0 {
			body.Clock.Ticks = 0
			body.Clock.FrameIndex++
		}
	case 0x20:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex != 5 || body.Clock.Ticks > 2 {
			break
		}
		body.VerticalVelocity = [3]int32{0, -0x30000, 0x30000}[body.Clock.Ticks]
		cursor := uint16(uint32(body.Gravity)>>16) + 0x100
		if cursor == 0x300 {
			cursor = 0
		}
		body.Gravity = replaceChuSpecialHighWord(body.Gravity, cursor)
		if cursor > 0x200 || cursor&0xff != 0 {
			return original, errors.New("source2 scatter cursor exceeds three reserved children")
		}
		child := &state.Children[cursor/0x100]
		if child.Size != 0 {
			break
		}
		initializeChuSpecialChild(child, state, 0x12c96, 0x40+(cursor>>4), 252)
		child.X, child.Y = body.X, addThrowCoordinateWord(body.Y, -0x34)
		child.VelocityX, child.VerticalVelocity, child.Facing = body.VelocityX, body.VerticalVelocity, body.Facing
		dx := int16(0x38)
		if body.Facing != 0 {
			dx, child.VelocityX = -dx, -child.VelocityX
		}
		child.X = addThrowCoordinateWord(child.X, dx)
		child.TargetMask, child.Pose = 0xffffffff, 13
	default:
		return original, errors.New("unsupported source2 motion action")
	}
	return state, nil
}
