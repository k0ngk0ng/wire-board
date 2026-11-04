package engine

import "errors"

// StepFighter8Motion recovers source8 slots3e/3f/40 and shared0f/11 through
// 15afc. AlternateVelocity owns the whole+48 long; only its high word counts.
func StepFighter8Motion(state ChuSpecialMotionState, parameters JumpParameters) (ChuSpecialMotionState, error) {
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
	case 0x3e:
		if body.Clock.Completed == 1 {
			body.Clock.Completed, body.ExtraJump = 0, 0
			body.GroundActionState = ResetActionState(body.GroundActionState)
			body.Gravity, body.VelocityX, body.VerticalVelocity = parameters.Gravity, 0, 0
			body.ActionKind, body.Pose, body.EnabledStages = 2, 7, 0x14
			break
		}
		if body.Clock.FrameIndex < 7 {
			body.Y -= 0x10000
			break
		}
		if body.Clock.FrameIndex > 7 {
			body.Y += 0x10000
			if int16(body.Ground>>16) <= int16(body.Y>>16) {
				body.Y = body.Ground
			}
			break
		}
		if int16(body.Ground>>16)-32 <= int16(body.Y>>16) && body.Clock.Ticks&3 == 0 {
			if err := spawnChuSpecialTrail(&state); err != nil {
				return original, err
			}
		}
		if state.Controls.Held&ButtonUp != 0 {
			if body.VerticalVelocity >= -0x40000 {
				body.VerticalVelocity -= 0x4000
			}
		} else if state.Controls.Held&ButtonDown != 0 && body.VerticalVelocity <= 0x40000 {
			body.VerticalVelocity += 0x4000
		}
		body.Y += body.VerticalVelocity
		if int16(body.Ground>>16) <= int16(body.Y>>16) {
			body.Y = body.Ground
		}
		moveFighter0Facing(body)
		if state.ContactFlags&12 != 0 {
			body.Clock.Ticks = 0
			body.Clock.FrameIndex++
		}
	case 0x3f:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex == 0 && body.Clock.Ticks == 0 {
			child := &state.Children[0]
			if child.Size != 0 {
				break
			}
			child.Behavior, child.Size, child.Stage, child.Auxiliary76 = 0x147ae, 0x100, 0, 255
			child.Team, child.TargetBit = body.Team+0x10, state.TargetBit+1
			child.Attributes = body.Attributes + 0x60
			child.Ground = replaceChuSpecialHighWord(child.Ground, uint16(body.Ground>>16))
			child.X, child.Y, child.Facing = body.X, body.Y, body.Facing
			child.Depth, child.Layer = body.Depth, body.Layer-4
			child.TargetMask, child.Pose = 0xffffffff, 0xa6
		} else if body.Clock.FrameIndex == 1 && body.Clock.Ticks == 0 {
			moveFighter0Facing(body)
			body.Facing ^= 1
		}
	case 0x40:
		switch body.Pose {
		case 0xad:
			counter := uint16(uint32(state.AlternateVelocity)>>16) + 1
			state.AlternateVelocity = replaceChuSpecialHighWord(state.AlternateVelocity, counter)
			if int16(counter) > 128 || state.Controls.Pressed&ButtonC != 0 {
				body.Pose = 0xae
				body.GroundActionState = ResetActionState(body.GroundActionState)
				break
			}
			if state.Controls.Held&ButtonRight != 0 {
				if body.VelocityX <= 0x30000 {
					body.VelocityX += 0x3000
				}
			} else if state.Controls.Held&ButtonLeft != 0 && body.VelocityX >= -0x30000 {
				body.VelocityX -= 0x3000
			}
			body.X += body.VelocityX
		case 0xae:
			if body.Clock.Completed == 1 {
				body.Clock.Completed = 0
				enterChuSpecialGround(body, false)
			} else {
				body.VelocityX >>= 1
				body.X += body.VelocityX
			}
		default:
			if body.Clock.Completed == 1 {
				body.Pose = 0xad
				body.GroundActionState = ResetActionState(body.GroundActionState)
			}
		}
	default:
		return original, errors.New("unsupported source8 motion action")
	}
	return state, nil
}
