package engine

import "errors"

// Fighter3MotionState is a transparent view of the existing canonical fields.
// Combo input uses VelocityX.bit24; auto-charge writes Controls.Pressed itself.
type Fighter3MotionState = Fighter0MotionState

// StepFighter3Motion recovers core slots13/21/23/24 and shared0f/11/15/17.
// Slot22/1863a, the shared tail and child/FX callbacks are separate consumers.
func StepFighter3Motion(state Fighter3MotionState, parameters JumpParameters) (Fighter3MotionState, error) {
	original := state
	body := &state.Body
	switch body.ActionKind {
	case 0x0f:
		view, err := StepChuSpecialMotion(state.ChuSpecialMotionState, parameters)
		if err != nil {
			return original, err
		}
		state.ChuSpecialMotionState = view
	case 0x11, 0x15, 0x17:
		return StepFighter0Motion(state, parameters)
	case 0x13:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, body.Clock.Flags&4 != 0)
			break
		}
		if body.Clock.FrameIndex == 0 {
			break
		}
		if body.Clock.Ticks == 0 {
			if err := spawnChuSpecialDust(&state.ChuSpecialMotionState); err != nil {
				return original, err
			}
		}
		moveChuSpecialHorizontal(body)
		if body.Clock.FrameIndex >= 2 && body.Clock.Ticks == 0 {
			body.VelocityX >>= 1
		}
	case 0x21:
		const inputLock uint32 = 1 << 24
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			body.VelocityX = int32(uint32(body.VelocityX) &^ inputLock)
			break
		}
		if state.Controls.Pressed&0x70 != 0 {
			body.VelocityX = int32(uint32(body.VelocityX) | inputLock)
		}
		if body.Clock.FrameIndex >= 26 {
			return original, errors.New("source3 combo frame exceeds recovered twenty-six-entry table")
		}
		target := uint8(0)
		switch body.Clock.FrameIndex {
		case 3, 5, 7:
			target = 9
		case 11, 13, 15:
			target = 17
		case 19, 21, 23:
			target = 25
		}
		if target != 0 {
			if uint32(body.VelocityX)&inputLock != 0 {
				body.VelocityX = int32(uint32(body.VelocityX) &^ inputLock)
			} else if body.Clock.Ticks == 0 {
				body.Clock.Ticks, body.Clock.FrameIndex = 0, target
			}
		}
	case 0x23:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			energy := int16(state.Energy >> 16)
			if energy >= 4 {
				state.Energy = replaceChuSpecialHighWord(state.Energy, uint16(energy-4))
				body.ActionKind, body.Pose = 0x24, 0xa4
				body.VelocityX = replaceChuSpecialHighWord(body.VelocityX, 32)
				body.VerticalVelocity = replaceChuSpecialHighWord(body.VerticalVelocity, 0xffd0)
				body.Gravity = int32(uint32(body.Gravity) & 0x00ffffff)
				state.Power = 255
				body.Request, body.EnabledStages = 0, 0
				body.GroundActionState = ResetActionState(body.GroundActionState)
			}
			break
		}
		if body.Clock.FrameIndex == 31 && body.Clock.Ticks == 0 {
			state.Energy = replaceChuSpecialHighWord(state.Energy, uint16(int16(state.Energy>>16)+16))
		}
		if body.Clock.FrameIndex != 0 {
			break
		}
		if body.Clock.Ticks&3 == 0 {
			if err := spawnChuSpecialTrail(&state.ChuSpecialMotionState); err != nil {
				return original, err
			}
		}
		moveFighter0Facing(body)
		if body.Clock.Ticks >= 64 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
		} else if state.ContactFlags&1 != 0 {
			body.Clock.Ticks = 0
			body.Clock.FrameIndex++
		}
	case 0x24:
		if body.Clock.FrameIndex == 4 && body.Clock.Ticks == 16 {
			state.Controls.Pressed |= ButtonA
		}
		// The original jumps into1a430 with ActionKind24 still present. Preserve
		// that kind and the changed pressed channel through the shared consumer.
		stepFighter0Charge(&state)
	default:
		return original, errors.New("unsupported source3 core motion action")
	}
	return state, nil
}
