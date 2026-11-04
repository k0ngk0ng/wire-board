package engine

import "errors"

// Fighter0MotionState adds only the global sound request and the existing FX
// extras to the shared motion fields. Gravity and AlternateVelocity keep their
// original single ownership even when a byte/word is a mode, offset or cursor.
type Fighter0MotionState struct {
	ChuSpecialMotionState
	PendingSound uint8
	EffectExtras [16]ReactionEffectExtra
}

// StepFighter0Motion recovers source0 slots0e/16/17/18/19 and their shared11/15
// paths up to15afc. It creates children but never schedules their callbacks.
func StepFighter0Motion(state Fighter0MotionState, parameters JumpParameters) (Fighter0MotionState, error) {
	original := state
	body := &state.Body
	switch body.ActionKind {
	case 0x0e:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			finishFighter10Motion(body, parameters)
			break
		}
		if int16(body.Ground>>16) == int16(body.Y>>16) && body.Clock.Ticks == 0 {
			if err := spawnChuSpecialDust(&state.ChuSpecialMotionState); err != nil {
				return original, err
			}
		}
		moveChuSpecialHorizontal(body)
	case 0x11:
		view, err := StepFighter10Motion(Fighter10MotionState{ObjectID: state.ObjectID, Body: state.Body, Controls: state.Controls,
			Aux48: state.AlternateVelocity, ContactFlags: state.ContactFlags, Pool: state.Pool}, parameters)
		if err != nil {
			return original, err
		}
		state.Body, state.Pool, state.Controls, state.AlternateVelocity, state.ContactFlags = view.Body, view.Pool, view.Controls, view.Aux48, view.ContactFlags
	case 0x15:
		view, err := StepChuSpecialMotion(state.ChuSpecialMotionState, parameters)
		if err != nil {
			return original, err
		}
		state.ChuSpecialMotionState = view
	case 0x16:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		index := body.Clock.FrameIndex
		if index >= 13 {
			return original, errors.New("source0 flight frame exceeds recovered thirteen-entry table")
		}
		switch {
		case index <= 2:
			if body.Pose < 0xac && body.Clock.Ticks&3 == 0 {
				if err := spawnChuSpecialTrail(&state.ChuSpecialMotionState); err != nil {
					return original, err
				}
			}
			moveFighter0Facing(body)
		case index == 3:
			if body.Pose < 0xac && body.Clock.Ticks == 0 {
				if err := spawnFighter0FourEffects(&state); err != nil {
					return original, err
				}
			}
			moveFighter0Facing(body)
		case index <= 11:
			if body.VerticalVelocity >= 0 {
				body.VerticalVelocity += 0x3000
				body.Y += body.VerticalVelocity
			} else {
				body.VerticalVelocity += 0x4000
				body.Y += body.VerticalVelocity
				if body.VelocityX > 0 {
					body.VelocityX -= 0x4000
					moveFighter0Facing(body)
				}
			}
			if int16(body.Ground>>16) <= int16(body.Y>>16) {
				body.Y = body.Ground
				body.Clock.Ticks = 0
				body.Clock.FrameIndex++
				pool, _, err := SpawnMovementDust(state.Pool, EffectSource{X: body.X, Y: body.Y,
					Attributes: body.Attributes, Depth: body.Depth, Layer: body.Layer})
				if err != nil {
					return original, err
				}
				state.Pool = pool
			}
		}
	case 0x17:
		stepFighter0Charge(&state)
	case 0x18:
		if body.Clock.Ticks != 0 {
			break
		}
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		switch body.Clock.FrameIndex {
		case 3, 9:
			body.VerticalVelocity = 0
		case 5:
			body.VerticalVelocity = -0x30000
		case 7:
			body.VerticalVelocity = 0x30000
		case 11:
			body.VerticalVelocity = -0x50000
		case 13:
			body.VerticalVelocity = 0x50000
		default:
			return state, nil
		}
		cursor := uint16(uint32(body.VelocityX)>>16) + 0x100
		if cursor == 0x300 {
			cursor = 0
		}
		body.VelocityX = replaceChuSpecialHighWord(body.VelocityX, cursor)
		if cursor > 0x200 || cursor&0xff != 0 {
			return original, errors.New("source0 scatter cursor exceeds three reserved children")
		}
		child := &state.Children[cursor/0x100]
		if child.Size != 0 {
			break
		}
		initializeFighter0Child(child, state, 0x12c96, 0x40+(cursor>>4), 252)
		child.X, child.Y = body.X, addThrowCoordinateWord(body.Y, -28)
		child.VerticalVelocity, child.VelocityX, child.Facing = body.VerticalVelocity, 0x80000, body.Facing
		dx := int16(20)
		if body.Facing != 0 {
			dx, child.VelocityX = -dx, -child.VelocityX
		}
		child.X = addThrowCoordinateWord(child.X, dx)
		child.TargetMask, child.Pose = 0xffffffff, 8
	case 0x19:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			energy := int16(state.Energy >> 16)
			if energy >= 16 {
				state.Energy = replaceChuSpecialHighWord(state.Energy, uint16(energy-16))
				body.VerticalVelocity, body.VelocityX = -0x60000, 0x40000
				body.ActionKind, body.Pose, body.Request, body.EnabledStages = 0x16, 0xa3, 0, 0
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
	default:
		return original, errors.New("unsupported source0 motion action")
	}
	return state, nil
}

func moveFighter0Facing(body *FighterBodyState) {
	velocity := body.VelocityX
	if body.Facing != 0 {
		velocity = -velocity
	}
	body.X += velocity
}

func spawnFighter0FourEffects(state *Fighter0MotionState) error {
	offsets := [4]int16{-24, -8, 8, 24}
	velocities := [4]int32{-0x20000, -0x10000, 0x10000, 0x20000}
	for side := 0; side < 4; side++ {
		pool, index, err := AllocateEffect(state.Pool)
		if err != nil {
			return err
		}
		state.Pool = pool
		if index < 0 {
			continue
		}
		slot := &state.Pool.Slots[index]
		slot.Type, slot.VelocityX = 0x10, velocities[side]
		slot.X, slot.Y = addThrowCoordinateWord(state.Body.X, offsets[side]), state.Body.Y
		slot.Attributes = state.Body.Attributes&0x9800 | 0x2000
		slot.Depth, slot.Layer = state.Body.Depth, state.Body.Layer-4
		if side == 0 || side == 3 {
			state.EffectExtras[index].Phase = 8
		}
	}
	return nil
}

func initializeFighter0Child(child *ChuSpecialChild, state Fighter0MotionState, behavior uint32, attributeOffset uint16, layerOffset uint8) {
	child.Behavior, child.Size, child.Stage, child.Auxiliary76 = behavior, 0x100, 0, 255
	child.Team, child.TargetBit = state.Body.Team, state.TargetBit+1
	child.Attributes = (state.Body.Attributes+attributeOffset)&0x9fff | 0x4000
	child.Ground = replaceChuSpecialHighWord(child.Ground, uint16(state.Body.Ground>>16))
	child.Depth, child.Layer = state.Body.Depth, state.Body.Layer+layerOffset
}

func stepFighter0Charge(state *Fighter0MotionState) {
	body := &state.Body
	if body.Clock.Completed == 1 {
		body.Clock.Completed = 0
		enterChuSpecialGround(body, false)
		return
	}
	if body.Clock.FrameIndex != 4 {
		return
	}
	charge := &state.Children[1]
	if charge.Size == 0 {
		initializeFighter0Child(charge, *state, 0x132de, 0x40, 252)
		charge.VerticalVelocity = replaceChuSpecialHighWord(charge.VerticalVelocity, uint16(body.VerticalVelocity>>16))
		x := int16(body.VelocityX >> 16)
		if body.Facing != 0 {
			x = -x
		}
		charge.VelocityX = replaceChuSpecialHighWord(charge.VelocityX, uint16(x))
	}
	if energy := int16(state.Energy >> 16); energy != 0 {
		delta := int16(255) - state.Power
		delta = (delta >> 6) + 1
		state.Power += delta
		if state.Power >= 256 {
			state.Power = 255
		} else {
			state.Energy = replaceChuSpecialHighWord(state.Energy, uint16(energy-1))
		}
	}
	body.Clock.Ticks &= 63
	if body.Clock.Ticks&3 == 0 {
		state.PendingSound = 0xa6
	}
	pressed := state.Controls.Pressed
	if pressed&ButtonC != 0 {
		body.Clock.Ticks, body.Clock.FrameIndex = 0, 8
		return
	}
	mode := uint8(uint32(body.Gravity) >> 24)
	if mode != 0 && state.Controls.Held&0x70 == 0 {
		switch mode {
		case 1:
			state.AlternateVelocity = 0x40000
		case 2:
			state.AlternateVelocity = 0x60000
		default:
			return
		}
	} else if mode == 0 && pressed&ButtonB != 0 {
		state.AlternateVelocity = 0x40000
	} else if mode == 0 && pressed&ButtonA != 0 {
		state.AlternateVelocity = 0x60000
	} else {
		if body.Facing&1 == 0 && pressed&ButtonLeft != 0 || body.Facing&1 != 0 && pressed&ButtonRight != 0 {
			body.Clock.Ticks, body.Clock.FrameIndex = 0, 8
		}
		return
	}
	main := &state.Children[0]
	if main.Size != 0 {
		return
	}
	initializeFighter0Child(main, *state, 0x1313e, 0x40, 252)
	main.Y = addThrowCoordinateWord(body.Y, int16(body.VerticalVelocity>>16))
	main.X, main.VelocityX, main.Facing = body.X, state.AlternateVelocity, body.Facing
	dx := int16(body.VelocityX >> 16)
	if body.Facing != 0 {
		dx, main.VelocityX = -dx, -main.VelocityX
	}
	main.X = addThrowCoordinateWord(main.X, dx)
	main.TargetMask, main.Power = 0xffffffff, state.Power>>5
	main.Pose = uint8(main.Power)
	release := &state.Children[2]
	if release.Size != 0 {
		return
	}
	initializeFighter0Child(release, *state, 0x134c2, 0x60, 248)
	release.X, release.Y = addThrowCoordinateWord(body.X, dx), addThrowCoordinateWord(body.Y, int16(body.VerticalVelocity>>16))
	release.Power = state.Power
	body.Clock.Ticks, body.Clock.FrameIndex, state.Power = 0, 5, 0
}
