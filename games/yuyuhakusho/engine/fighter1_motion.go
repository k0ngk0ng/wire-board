package engine

import "errors"

// Fighter1MotionState exposes the existing FX extras needed by the four-FX
// initializer. Power is the existing+7c word cursor, not another counter.
type Fighter1MotionState struct {
	ChuSpecialMotionState
	EffectExtras [16]ReactionEffectExtra
}

// StepFighter1Motion recovers1a/1b/1c/1d and shared0f/11 through15afc.
// Child and FX lifetimes, animation timing and the shared tail are external.
func StepFighter1Motion(state Fighter1MotionState, parameters JumpParameters) (Fighter1MotionState, error) {
	original := state
	body := &state.Body
	switch body.ActionKind {
	case 0x0f:
		view, err := StepChuSpecialMotion(state.ChuSpecialMotionState, parameters)
		if err != nil {
			return original, err
		}
		state.ChuSpecialMotionState = view
	case 0x11:
		view, err := StepFighter10Motion(Fighter10MotionState{ObjectID: state.ObjectID, Body: state.Body, Controls: state.Controls,
			Aux48: state.AlternateVelocity, ContactFlags: state.ContactFlags, Pool: state.Pool}, parameters)
		if err != nil {
			return original, err
		}
		state.Body, state.Controls, state.Pool = view.Body, view.Controls, view.Pool
		state.AlternateVelocity, state.ContactFlags = view.Aux48, view.ContactFlags
	case 0x1a, 0x1b:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex != 2 || body.Clock.Ticks != 0 {
			break
		}
		child := &state.Children[0]
		if child.Size != 0 {
			break
		}
		if body.ActionKind == 0x1a {
			initializeFighter1Child(child, state.ChuSpecialMotionState, 0x12dd4, 0x40)
			child.Attributes = child.Attributes&0x9fff | 0x6000
			child.Pose = 9
			dx := int16(body.VelocityX >> 16)
			if body.Facing != 0 {
				dx = -dx
			}
			child.X, child.Y = addThrowCoordinateWord(body.X, dx), body.Y
		} else {
			initializeFighter1Child(child, state.ChuSpecialMotionState, 0x12ee8, 0x60)
			child.VelocityX = body.VelocityX
			child.X, child.Y = body.X, addThrowCoordinateWord(body.Y, -0x34)
			dx := int16(0x20)
			if body.Facing != 0 {
				dx, child.VelocityX = -dx, -child.VelocityX
			}
			child.X = addThrowCoordinateWord(child.X, dx)
		}
	case 0x1c:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.Ticks != 1 {
			break
		}
		switch body.Clock.FrameIndex {
		case 0:
			view := Fighter0MotionState{ChuSpecialMotionState: state.ChuSpecialMotionState, EffectExtras: state.EffectExtras}
			if err := spawnFighter0FourEffects(&view); err != nil {
				return original, err
			}
			state.Pool, state.EffectExtras = view.Pool, view.EffectExtras
		case 1, 4, 8, 10, 11:
		default:
			return state, nil
		}
		cursor := uint16(state.Power) + 0x100
		if cursor == 0x300 {
			cursor = 0
		}
		state.Power = int16(cursor)
		if cursor > 0x200 || cursor&0xff != 0 {
			return original, errors.New("source1 Power cursor exceeds three reserved children")
		}
		child := &state.Children[cursor/0x100]
		if child.Size != 0 {
			break
		}
		initializeFighter1Child(child, state.ChuSpecialMotionState, 0x1300c, 0x40+(cursor>>4))
		child.Y = addThrowCoordinateWord(body.Y, -0x10)
		// The original stores parent X as this child's origin input at+10.
		// Its old X/Pose/Power are intentionally not initialized here.
		child.VelocityX, child.VerticalVelocity = body.X, -0x28000
	case 0x1d:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex < 18 {
			break
		}
		var displacement int32
		switch {
		case body.Clock.FrameIndex < 27:
			if body.Clock.Ticks&3 == 0 {
				if err := spawnChuSpecialTrail(&state.ChuSpecialMotionState); err != nil {
					return original, err
				}
			}
			displacement = 0x40000
		case body.Clock.FrameIndex == 27:
			displacement = 0x30000
		case body.Clock.FrameIndex == 28:
			displacement = 0x20000
		case body.Clock.FrameIndex < 32:
			displacement = 0x10000
		}
		if body.Facing != 0 {
			displacement = -displacement
		}
		body.X += displacement
		if body.Clock.FrameIndex == 18 {
			if body.Clock.Ticks >= 48 {
				body.Clock.Ticks, body.Clock.FrameIndex = 0, 27
			} else if state.ContactFlags&1 != 0 {
				body.Clock.Ticks = 0
				body.Clock.FrameIndex++
			}
		}
	default:
		return original, errors.New("unsupported source1 motion action")
	}
	return state, nil
}

func initializeFighter1Child(child *ChuSpecialChild, state ChuSpecialMotionState, behavior uint32, attributeOffset uint16) {
	child.Behavior, child.Size, child.Stage, child.Auxiliary76 = behavior, 0x100, 0, 255
	child.Team, child.TargetBit = state.Body.Team, state.TargetBit+1
	child.Attributes = state.Body.Attributes + attributeOffset
	child.Ground = replaceChuSpecialHighWord(child.Ground, uint16(state.Body.Ground>>16))
	child.Facing, child.Depth, child.Layer = state.Body.Facing, state.Body.Depth, state.Body.Layer-4
	child.TargetMask = 0xffffffff
}
