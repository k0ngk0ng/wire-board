package engine

import "errors"

// Fighter10MotionState owns only the fields used by source10's five additional
// continuations. Aux48 is one long: slot49 changes its highest byte only.
// Other action-state views must merge this value rather than keep a second copy.
type Fighter10MotionState struct {
	ObjectID     uint16
	Body         FighterBodyState
	Controls     FighterControls
	Aux48        int32
	ContactFlags uint8
	Pool         EffectPool
}

// StepFighter10Motion recovers slots11/2d/47/48/49 to the shared action-tail
// boundary. Animation timing, record submission and FX updates remain external.
func StepFighter10Motion(state Fighter10MotionState, parameters JumpParameters) (Fighter10MotionState, error) {
	original := state
	body := &state.Body
	switch body.ActionKind {
	case 0x11:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			finishFighter10Motion(body, parameters)
			break
		}
		// Original eight-entry table: frames0/6/7 do not move;1..5 retreat.
		// Source10's normal pose has seven records, although entry7 is defined.
		if body.Clock.FrameIndex > 7 {
			return original, errors.New("retreat frame exceeds the recovered eight-entry table")
		}
		if body.Clock.FrameIndex >= 1 && body.Clock.FrameIndex <= 5 {
			velocity := body.VelocityX
			if body.Facing&1 != 0 {
				velocity = -velocity
			}
			body.X -= velocity
		}
	case 0x2d:
		if body.Clock.Completed&1 != 0 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.Ticks != 0 && body.Clock.FrameIndex >= 2 && body.Clock.FrameIndex != 14 && state.Controls.Pressed&(ButtonA|ButtonB) != 0 {
			body.Clock.Ticks = 0
			body.Clock.FrameIndex++
		}
	case 0x47:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			finishFighter10Motion(body, parameters)
			break
		}
		if body.Clock.Ticks&3 == 0 {
			if err := spawnFighter10Trail(&state); err != nil {
				return original, err
			}
		}
		moveChuSpecialHorizontal(body)
		if body.Clock.FrameIndex != 0 {
			if body.Clock.Ticks == 0 {
				body.VelocityX >>= 1
			}
		} else if state.ContactFlags&1 != 0 {
			body.Clock.Ticks = 255
			body.Clock.FrameIndex++
		}
	case 0x48:
		if body.Clock.FrameIndex == 0 {
			break
		}
		if int16(body.Ground>>16)-64 <= int16(body.Y>>16) && body.Clock.Ticks&3 == 0 {
			if err := spawnFighter10Trail(&state); err != nil {
				return original, err
			}
		}
		moveChuSpecialHorizontal(body)
		body.Y += body.VerticalVelocity
		if int16(body.Ground>>16) <= int16(body.Y>>16) {
			body.Y = body.Ground
			enterChuSpecialGround(body, false)
			body.ActionKind, body.Pose = 0x14, 0xae
			body.VelocityX >>= 1
			body.EnabledStages = 0
			body.GroundActionState = ResetActionState(body.GroundActionState)
		}
	case 0x49:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex < 4 {
			break
		}
		var velocity int32
		switch {
		case body.Clock.FrameIndex < 28:
			counter := uint8(uint32(state.Aux48)>>24) + 1
			state.Aux48 = int32(uint32(counter)<<24 | uint32(state.Aux48)&0x00ffffff)
			if counter&3 == 0 {
				if err := spawnFighter10Trail(&state); err != nil {
					return original, err
				}
			}
			velocity = 0x40000
		case body.Clock.FrameIndex == 28:
			velocity = 0x30000
		case body.Clock.FrameIndex == 29:
			velocity = 0x20000
		case body.Clock.FrameIndex == 30:
			velocity = 0x10000
		}
		if body.Facing != 0 {
			velocity = -velocity
		}
		body.X += velocity
	default:
		return original, errors.New("unsupported source10 motion action")
	}
	return state, nil
}

func finishFighter10Motion(body *FighterBodyState, parameters JumpParameters) {
	if int16(body.Ground>>16) == int16(body.Y>>16) {
		enterChuSpecialGround(body, false)
		return
	}
	body.ExtraJump = 0
	body.GroundActionState = ResetActionState(body.GroundActionState)
	body.Gravity, body.VelocityX, body.VerticalVelocity = parameters.Gravity, 0, 0
	body.ActionKind, body.Pose, body.EnabledStages = 2, 7, 0x14
}

func spawnFighter10Trail(state *Fighter10MotionState) error {
	view := ChuSpecialMotionState{Body: state.Body, Pool: state.Pool}
	if err := spawnChuSpecialTrail(&view); err != nil {
		return err
	}
	state.Pool = view.Pool
	return nil
}
