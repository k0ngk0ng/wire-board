package engine

import "errors"

// Fighter4ChildExtra is a narrow initializer view. Direction and the input
// and contact bytes map existing WorldObject owners; DragonBank maps its +fe
// word. No child coordinate, velocity or other canonical field is duplicated.
type Fighter4ChildExtra struct {
	Direction                   int32
	Held, Pressed, ContactFlags uint8
	DragonBank                  uint16
}

// Fighter4MotionState carries only the existing movement-source, shared bank
// cursor and narrow child/FX owners missing from the common motion view.
type Fighter4MotionState struct {
	ChuSpecialMotionState
	MovementSource   uint32
	DragonBankCursor uint16
	ChildExtras      [3]Fighter4ChildExtra
	EffectExtras     [16]ReactionEffectExtra
}

// StepFighter4Motion recovers the ten source4 pre-tail motion entries. The
// caller resolves parameters from the current MovementSource for every step.
// This phase creates children and effects but never schedules their callbacks.
func StepFighter4Motion(state Fighter4MotionState, parameters JumpParameters) (Fighter4MotionState, error) {
	original := state
	body := &state.Body
	switch body.ActionKind {
	case 0x0e:
		view, err := StepFighter0Motion(Fighter0MotionState{ChuSpecialMotionState: state.ChuSpecialMotionState}, parameters)
		if err != nil {
			return original, err
		}
		state.ChuSpecialMotionState = view.ChuSpecialMotionState
	case 0x12:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			finishFighter10Motion(body, parameters)
			break
		}
		if body.Clock.FrameIndex >= 8 {
			return original, errors.New("source4 retreat frame exceeds recovered eight-entry table")
		}
		switch body.Clock.FrameIndex {
		case 1, 2, 4, 5:
			velocity := body.VelocityX
			if body.Facing&1 != 0 {
				velocity = -velocity
			}
			body.X -= velocity
		}
	case 0x25:
		if body.Clock.Completed != 1 {
			body.X += body.VelocityX
			break
		}
		body.Clock.Completed = 0
		offset := int16(64)
		if body.Facing&1 != 0 {
			offset = -offset
		}
		body.X = addThrowCoordinateWord(body.X, offset)
		state.Body = FinishFighter4Rush(ChuAttackState{Body: *body}).Body
	case 0x26, 0x27:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		count, slow := uint8(7), uint8(4)
		if body.ActionKind == 0x27 {
			count, slow = 11, 8
		}
		if body.Clock.FrameIndex >= count {
			return original, errors.New("source4 dash frame exceeds recovered finite table")
		}
		if body.Clock.FrameIndex > slow {
			break
		}
		displacement := int32(0x40000)
		if body.Clock.FrameIndex == slow {
			displacement = 0x20000
		}
		residue := body.Clock.Ticks & 3
		displacement |= int32(residue)
		if residue == 0 {
			if err := spawnChuSpecialTrail(&state.ChuSpecialMotionState); err != nil {
				return original, err
			}
			// Original1fa96 overwrites D0 with this value even on a full
			// pool;1b296 then consumes that same register as displacement.
			displacement = 0x149ac
		}
		if body.Facing != 0 {
			displacement = -displacement
		}
		body.X += displacement
	case 0x28:
		if body.Clock.Completed == 1 {
			body.ExtraJump++
			enterFighter4MotionAir(body, parameters)
		}
	case 0x29:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			state.Body = BeginFighter4DragonFollowup(ChuAttackState{Body: *body}).Body
			break
		}
		if body.Clock.FrameIndex == 0 {
			if body.Clock.Ticks&15 == 0 {
				if err := spawnFighter4DragonChargeEffects(&state); err != nil {
					return original, err
				}
			}
			break
		}
		if body.Clock.FrameIndex != 2 || body.Clock.Ticks != 0 || state.Children[0].Size != 0 {
			break
		}
		child := &state.Children[0]
		initializeFighter4MotionChild(child, state.ChuSpecialMotionState, 0x13bb6, 248)
		child.Ground, child.Y, child.X = body.Ground, addThrowCoordinateWord(body.Y, -42), body.X
		child.Facing, child.TargetMask = body.Facing, 0xffffffff
		direction, held, offset := int32(0x1000000), uint8(8), int16(32)
		if body.Facing != 0 {
			direction, held, offset = 0x3000000, 4, -32
		}
		child.X = addThrowCoordinateWord(child.X, offset)
		state.ChildExtras[0].Direction, state.ChildExtras[0].Held = direction, held
		state.DragonBankCursor = (state.DragonBankCursor + 0x200) & 0x7ff
		state.ChildExtras[0].DragonBank = state.DragonBankCursor
	case 0x2a:
		state.ChildExtras[0].Held, state.ChildExtras[0].Pressed = uint8(state.Controls.Held), uint8(state.Controls.Pressed)
		child := &state.Children[0]
		dx := fighter4MotionAbsWord(int16(body.X>>16) - int16(child.X>>16))
		dy := fighter4MotionAbsWord(int16(body.Y>>16) - 32 - int16(child.Y>>16))
		if dx <= 24 && dy <= 32 {
			view := ReactionMotionState{ActorID: state.ObjectID, Body: *body, Pool: state.Pool, EffectExtras: state.EffectExtras}
			if err := spawnReactionMarker(&view); err != nil {
				return original, err
			}
			state.Pool, state.EffectExtras = view.Pool, view.EffectExtras
			body.Clock.Completed = 0
			state.Energy = replaceChuSpecialHighWord(state.Energy, 223)
			state.ChildExtras[0].ContactFlags |= 16
			body.Pose, body.ActionKind, state.MovementSource = 0xab, 0x2b, 0x15946
			body.GroundActionState = ResetActionState(body.GroundActionState)
		} else if child.Size == 0 || state.Controls.Pressed&ButtonC != 0 {
			body.Clock.Completed = 0
			body.Pose, body.ActionKind = 0xaa, 3
			body.GroundActionState = ResetActionState(body.GroundActionState)
		}
	case 0x2b:
		if body.Clock.Completed == 1 {
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex == 1 && body.Clock.Ticks == 0 && state.Children[0].Size == 0 {
			child := &state.Children[0]
			initializeFighter4MotionChild(child, state.ChuSpecialMotionState, 0x13fc8, 0)
			child.TargetMask = 0xffffffff
		}
	case 0x2c:
		if body.Clock.Completed == 1 {
			body.ExtraJump++
			body.GroundActionState = ResetActionState(body.GroundActionState)
			body.Gravity = parameters.Gravity
			body.VerticalVelocity = (parameters.VerticalVelocity >> 1) + (parameters.VerticalVelocity >> 2)
			body.VelocityX = parameters.HorizontalVelocity
			if body.Facing != 1 {
				body.VelocityX = -body.VelocityX
			}
			body.ActionKind, body.Pose, body.EnabledStages = 2, 0xaf, 0
			break
		}
		if state.Controls.Held&ButtonRight != 0 {
			body.VelocityX += 0x8000
			if body.VelocityX >= 0x40000 {
				body.VelocityX = 0x40000
			}
		} else if state.Controls.Held&ButtonLeft != 0 {
			body.VelocityX -= 0x8000
			if body.VelocityX <= -0x40000 {
				body.VelocityX = -0x40000
			}
		}
		body.X += body.VelocityX
		body.VerticalVelocity += body.Gravity
		body.Y += body.VerticalVelocity
		if body.VerticalVelocity >= 0 && int16(body.Ground>>16) <= int16(body.Y>>16) {
			body.Y = body.Ground
			body.ActionKind, body.Pose, body.EnabledStages, body.ExtraJump = 3, 0xa9, 0, 0
			body.GroundActionState = ResetActionState(body.GroundActionState)
			break
		}
		if body.Pose == 0xa8 {
			height := fighter4MotionAbsWord(int16(body.Ground>>16) - int16(body.Y>>16))
			if height >= 32 && state.Controls.Pressed&(ButtonB|ButtonA) != 0 {
				body.Pose, body.VerticalVelocity, body.EnabledStages = 0xae, 0x28000, 0
				body.GroundActionState = ResetActionState(body.GroundActionState)
			}
		} else if state.ContactFlags&12 != 0 {
			body.Clock.Ticks = 0
			body.Clock.FrameIndex++
			body.VerticalVelocity = 0xa0000
		}
	default:
		return original, errors.New("unsupported source4 motion action")
	}
	return state, nil
}

func enterFighter4MotionAir(body *FighterBodyState, parameters JumpParameters) {
	body.ExtraJump = 0
	body.GroundActionState = ResetActionState(body.GroundActionState)
	body.Gravity, body.VelocityX, body.VerticalVelocity = parameters.Gravity, 0, 0
	body.ActionKind, body.Pose, body.EnabledStages = 2, 7, 0x14
}

func fighter4MotionAbsWord(value int16) int16 {
	if value < 0 {
		return -value
	}
	return value
}

func initializeFighter4MotionChild(child *ChuSpecialChild, state ChuSpecialMotionState, behavior uint32, layerOffset uint8) {
	child.Behavior, child.Size, child.Stage, child.Auxiliary76 = behavior, 0x100, 0, 255
	child.Team, child.TargetBit = state.Body.Team, state.TargetBit+1
	child.Attributes = (state.Body.Attributes+0x4d)&0x9fff | 0x4000
	child.Depth, child.Layer = state.Body.Depth, state.Body.Layer+layerOffset
}

func spawnFighter4DragonChargeEffects(state *Fighter4MotionState) error {
	for side, phase := range [...]uint8{0, 4, 8} {
		pool, index, err := AllocateEffect(state.Pool)
		if err != nil {
			return err
		}
		state.Pool = pool
		if index < 0 {
			continue
		}
		slot := &state.Pool.Slots[index]
		slot.Type = 0x20
		offset := int16(16 - side*16)
		if state.Body.Facing != 0 {
			offset = -offset
		}
		slot.X, slot.Y = addThrowCoordinateWord(state.Body.X, offset), state.Body.Ground
		slot.Depth, slot.Layer = state.Body.Depth, state.Body.Layer-4
		slot.Attributes = state.Body.Attributes&0x8000 | 0x4000
		if state.Body.Facing != 0 {
			slot.Attributes |= 0x800
		}
		state.EffectExtras[index].Phase = phase
	}
	return nil
}
