package engine

import "errors"

// Fighter6MotionState adds only the existing+84 word owner. AlternateVelocity
// remains the single+48 long, including slot33's high-word counter view.
type Fighter6MotionState struct {
	ChuSpecialMotionState
	StunTicks uint16
}

// StepFighter6Motion is the source6 pre-tail entry. The support file owns the
// five delegated slots; this consumer owns only33/34/35/36/38.
func StepFighter6Motion(state Fighter6MotionState, parameters JumpParameters) (Fighter6MotionState, error) {
	original := state
	body := &state.Body
	switch body.ActionKind {
	case 0x0e, 0x0f, 0x11, 0x13, 0x37:
		return StepFighter6SupportMotion(state, parameters)
	case 0x33:
		body.Y += body.VerticalVelocity
		counter := uint16(uint32(state.AlternateVelocity)>>16) + 1
		state.AlternateVelocity = replaceChuSpecialHighWord(state.AlternateVelocity, counter)
		if counter == 16 {
			body.Clock.Completed = 0
			enterFighter6MainAir(body, parameters)
		}
	case 0x34:
		body.Y += body.VerticalVelocity
		if int16(body.Ground>>16) <= int16(body.Y>>16) {
			body.Y = body.Ground
			landed, pool, _, err := BeginLanding(*body, state.Pool)
			if err != nil {
				return original, err
			}
			state.Body, state.Pool = landed, pool
			state.Body.ExtraJump = 0
		}
	case 0x35, 0x36:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			if int16(body.Ground>>16) == int16(body.Y>>16) {
				enterChuSpecialGround(body, false)
			} else {
				enterFighter6MainAir(body, parameters)
			}
			break
		}
		first, taper := uint8(4), uint8(6)
		if body.ActionKind == 0x36 {
			first, taper = 8, 13
		}
		if body.Clock.FrameIndex < first {
			break
		}
		var displacement int32
		if body.Clock.FrameIndex < taper {
			if int16(body.Ground>>16)-32 <= int16(body.Y>>16) && body.Clock.Ticks&3 == 0 {
				if err := spawnChuSpecialTrail(&state.ChuSpecialMotionState); err != nil {
					return original, err
				}
			}
			displacement = 0x40000
		} else if body.Clock.FrameIndex == taper {
			displacement = 0x20000
		} else if body.Clock.FrameIndex == taper+1 {
			displacement = 0x10000
		}
		if body.Facing != 0 {
			displacement = -displacement
		}
		body.X += displacement
		if body.Clock.FrameIndex == first && state.ContactFlags&1 != 0 {
			body.Clock.Ticks = 0
			body.Clock.FrameIndex++
		}
	case 0x38:
		switch body.Pose {
		case 0xaa:
			state.StunTicks++
			if int16(state.StunTicks) > 256 || state.Controls.Pressed&0x70 != 0 {
				body.Pose = 0xa9
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
		case 0xa9:
			if body.Clock.Completed == 1 {
				body.Clock.Completed = 0
				if int16(state.StunTicks) > 128 {
					enterFighter6Exit(body)
				} else {
					enterChuSpecialGround(body, false)
				}
			} else {
				body.VelocityX >>= 1
				body.X += body.VelocityX
			}
		default:
			if body.Clock.Completed == 1 {
				body.Pose = 0xaa
				body.GroundActionState = ResetActionState(body.GroundActionState)
			}
		}
	default:
		return original, errors.New("unsupported source6 motion action")
	}
	return state, nil
}

func enterFighter6MainAir(body *FighterBodyState, parameters JumpParameters) {
	body.ExtraJump = 0
	body.GroundActionState = ResetActionState(body.GroundActionState)
	body.Gravity, body.VelocityX, body.VerticalVelocity = parameters.Gravity, 0, 0
	body.ActionKind, body.Pose, body.EnabledStages = 2, 7, 0x14
}
