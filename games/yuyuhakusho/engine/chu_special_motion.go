package engine

import "errors"

// ChuSpecialChild contains the fields initialized on the three reserved child
// slots. Behavior values identify recovered object kinds; they are never
// executed as addresses. Subsequent child updates are separate rules.
type ChuSpecialChild struct {
	Behavior                                          uint32
	Size, Stage                                       uint16
	VelocityX, VerticalVelocity                       int32
	Attributes, Depth                                 uint16
	Layer, Pose, Facing, Team, TargetBit, Auxiliary76 uint8
	X, Y, Ground                                      int32
	TargetMask                                        uint32
	Power                                             int16
}

type ChuSpecialMotionState struct {
	ObjectID                uint16
	Body                    FighterBodyState
	Controls                FighterControls
	Energy                  int32
	Power                   int16
	AlternateVelocity       int32
	ContactFlags, TargetBit uint8
	Children                [3]ChuSpecialChild
	Pool                    EffectPool
}

// StepChuSpecialMotion recovers the continuations selected by Chu attacks,
// including their immediate actions 0x14/0x43, up to the shared action tail.
// It does not advance animation timing, run that tail, or update child objects.
func StepChuSpecialMotion(state ChuSpecialMotionState, parameters JumpParameters) (ChuSpecialMotionState, error) {
	before := state
	body := &state.Body
	switch body.ActionKind {
	case 0x0f:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			if int16(body.Ground>>16) == int16(body.Y>>16) {
				enterChuSpecialGround(body, false)
			} else {
				body.ExtraJump = 0
				body.GroundActionState = ResetActionState(body.GroundActionState)
				body.Gravity, body.VelocityX, body.VerticalVelocity = parameters.Gravity, 0, 0
				body.ActionKind, body.Pose, body.EnabledStages = 2, 7, 0x14
			}
			break
		}
		if int16(body.Ground>>16) == int16(body.Y>>16) && body.Clock.Ticks == 0 {
			if err := spawnChuSpecialDust(&state); err != nil {
				return before, err
			}
		}
		moveChuSpecialHorizontal(body)
		if body.Clock.FrameIndex != 0 && body.Clock.Ticks == 0 {
			body.VelocityX >>= 1
		}
	case 0x14:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, body.Clock.Flags&4 != 0)
			break
		}
		if body.Clock.Ticks == 0 {
			if err := spawnChuSpecialDust(&state); err != nil {
				return before, err
			}
		}
		moveChuSpecialHorizontal(body)
		if body.Clock.FrameIndex >= 1 && body.Clock.Ticks == 0 {
			body.VelocityX >>= 1
		}
	case 0x15:
		body.Y = body.Ground
		if body.Clock.Completed != 1 {
			break
		}
		if body.Request != 1 && body.Request != 2 {
			enterChuSpecialGround(body, false)
			break
		}
		cost, pose := int16(8), uint8(0xa0)
		if body.Request == 2 {
			cost, pose = 12, 0xa1
		}
		energy := int16(state.Energy >> 16)
		if energy < cost {
			break
		}
		state.Energy = replaceChuSpecialHighWord(state.Energy, uint16(energy-cost))
		body.Pose = pose
		body.VelocityX = int32(uint32(body.VelocityX) & 0x00ffffff)
		body.ActionKind, body.Request, body.EnabledStages = 0x15, 0, 0
		body.GroundActionState = ResetActionState(body.GroundActionState)
	case 0x41:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex == 0 {
			break
		}
		if body.Clock.Ticks&3 == 0 {
			if err := spawnChuSpecialTrail(&state); err != nil {
				return before, err
			}
		}
		moveChuSpecialHorizontal(body)
		if body.Clock.FrameIndex >= 3 && body.Clock.Ticks == 0 {
			body.VelocityX >>= 1
		}
		if state.ContactFlags&12 != 0 {
			body.Clock.Ticks = 255
			body.Clock.FrameIndex++
		}
	case 0x42:
		stepChuSpecialCharge(&state)
	case 0x43:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.Ticks != 0 {
			break
		}
		index := body.Clock.FrameIndex
		if index != 0 && index != 3 {
			break
		}
		childIndex := 0
		if index == 0 {
			childIndex = 2
		}
		if state.Children[childIndex].Size != 0 {
			break
		}
		// These tables are the eight entries reached by charge values 0..255.
		// Invalid source states cannot silently read adjacent original code bytes.
		if state.Power < 0 || state.Power > 255 {
			return before, errors.New("Chu projectile charge is outside decoded pose tables")
		}
		power := state.Power >> 5
		child := &state.Children[childIndex]
		if index == 0 {
			initializeChuSpecialChild(child, state, 0x1361e, 0x60, 4)
			child.Facing = body.Facing
			child.X, child.Y = body.X, body.Y
			child.Power = power
			child.Pose = [8]uint8{26, 26, 27, 28, 29, 30, 31, 32}[power]
		} else {
			initializeChuSpecialChild(child, state, 0x1313e, 0x40, 252)
			child.Y = addThrowCoordinateWord(body.Y, int16(body.VerticalVelocity>>16))
			child.VelocityX = state.AlternateVelocity
			child.X = body.X
			offset := int16(body.VelocityX >> 16)
			child.Facing = body.Facing
			if body.Facing != 0 {
				offset = -offset
				child.VelocityX = -child.VelocityX
			}
			child.X = addThrowCoordinateWord(child.X, offset)
			child.TargetMask = 0xffffffff
			child.Power = power
			child.Pose = [8]uint8{33, 33, 34, 35, 4, 5, 6, 7}[power]
			state.Power = 0
		}
	case 0x44:
		if body.Clock.Completed == 1 {
			body.Clock.Completed = 0
			enterChuSpecialGround(body, false)
			break
		}
		if body.Clock.FrameIndex == 4 {
			if body.Clock.Ticks == 0 {
				body.Layer -= 8
			}
			break
		}
		moveChuSpecialHorizontal(body)
	case 0x45:
		if body.Clock.FrameIndex == 0 {
			break
		}
		if int16(body.Ground>>16)-64 <= int16(body.Y>>16) && body.Clock.Ticks&3 == 0 {
			if err := spawnChuSpecialTrail(&state); err != nil {
				return before, err
			}
		}
		moveChuSpecialHorizontal(body)
		body.Y += body.VerticalVelocity
		if state.ContactFlags&12 != 0 {
			body.ExtraJump++
			body.GroundActionState = ResetActionState(body.GroundActionState)
			body.Gravity = parameters.Gravity
			body.VerticalVelocity = (parameters.VerticalVelocity >> 1) + (parameters.VerticalVelocity >> 2)
			body.VelocityX = parameters.HorizontalVelocity
			if body.Facing != 1 {
				body.VelocityX = -body.VelocityX
			}
			body.ActionKind, body.Pose, body.EnabledStages = 2, 0xb0, 0
			break
		}
		if int16(body.Ground>>16) <= int16(body.Y>>16) {
			body.Y = body.Ground
			body.ActionKind, body.Pose, body.EnabledStages = 0x14, 0xb1, 0
			body.VelocityX >>= 1
			body.GroundActionState = ResetActionState(body.GroundActionState)
		}
	case 0x46:
		if body.Clock.FrameIndex == 0 {
			break
		}
		if int16(body.Ground>>16)-64 <= int16(body.Y>>16) && body.Clock.Ticks&3 == 0 {
			if err := spawnChuSpecialTrail(&state); err != nil {
				return before, err
			}
		}
		if body.Clock.FrameIndex <= 18 {
			body.VelocityX = 0x30000
			if body.Facing&1 != 0 {
				body.VelocityX = -body.VelocityX
			}
			body.X += body.VelocityX
			body.Y -= 0x24000
			body.VerticalVelocity = -0x48000
		} else {
			body.X += body.VelocityX
			body.VerticalVelocity += body.Gravity
			body.Y += body.VerticalVelocity
			if int16(body.Ground>>16) <= int16(body.Y>>16) {
				body.ActionKind, body.Pose, body.EnabledStages = 3, 0xab, 0
				body.GroundActionState = ResetActionState(body.GroundActionState)
			}
		}
	default:
		return before, errors.New("unsupported Chu special motion action")
	}
	return state, nil
}

func moveChuSpecialHorizontal(body *FighterBodyState) {
	velocity := body.VelocityX
	if body.Facing&1 != 0 {
		velocity = -velocity
	}
	body.X += velocity
}

func enterChuSpecialGround(body *FighterBodyState, crouch bool) {
	transition := GroundStand
	if crouch {
		transition = GroundCrouch
	}
	body.GroundActionState, _ = EnterGroundAction(body.GroundActionState, transition, 0)
}

func replaceChuSpecialHighWord(value int32, word uint16) int32 {
	return int32(uint32(word)<<16 | uint32(value)&0xffff)
}

func initializeChuSpecialChild(child *ChuSpecialChild, state ChuSpecialMotionState, behavior uint32, attributeOffset uint16, layerOffset uint8) {
	body := state.Body
	child.Behavior, child.Size, child.Stage = behavior, 0x100, 0
	child.Auxiliary76 = 255
	child.Team, child.TargetBit = body.Team, state.TargetBit+1
	child.Attributes = (body.Attributes+attributeOffset)&0x9fff | 0x2000
	child.Ground = replaceChuSpecialHighWord(child.Ground, uint16(body.Ground>>16))
	child.Depth, child.Layer = body.Depth, body.Layer+layerOffset
}

func stepChuSpecialCharge(state *ChuSpecialMotionState) {
	body := &state.Body
	child := &state.Children[1]
	if child.Size == 0 {
		initializeChuSpecialChild(child, *state, 0x132de, 0x40, 252)
		child.VerticalVelocity = replaceChuSpecialHighWord(child.VerticalVelocity, uint16(body.VerticalVelocity>>16))
		vx := int16(body.VelocityX >> 16)
		if body.Facing != 0 {
			vx = -vx
		}
		child.VelocityX = replaceChuSpecialHighWord(child.VelocityX, uint16(vx))
	}
	energy := int16(state.Energy >> 16)
	if energy != 0 {
		delta := int16(255) - state.Power
		delta = (delta >> 6) + 1
		state.Power += delta
		if state.Power >= 256 {
			state.Power = 255
		} else {
			state.Energy = replaceChuSpecialHighWord(state.Energy, uint16(energy-1))
		}
	}
	variant := uint8(uint32(body.Gravity) >> 24)
	pressed := state.Controls.Pressed
	if pressed&ButtonC != 0 {
		body.Clock.Completed = 0
		enterChuSpecialGround(body, false)
		return
	}
	speed := int32(0)
	if variant != 0 {
		if state.Controls.Held&0x70 == 0 {
			switch variant {
			case 1:
				speed = 0x40000
			case 2:
				speed = 0x60000
			default:
				return
			}
		}
	} else {
		if pressed&ButtonB != 0 {
			speed = 0x40000
		} else if pressed&ButtonA != 0 {
			speed = 0x60000
		}
	}
	if speed != 0 {
		state.AlternateVelocity = speed
		body.Pose, body.ActionKind = 0xa7, 0x43
		body.VelocityX = replaceChuSpecialHighWord(body.VelocityX, 0x40)
		body.VerticalVelocity = replaceChuSpecialHighWord(body.VerticalVelocity, 0xffc8)
		body.GroundActionState = ResetActionState(body.GroundActionState)
		return
	}
	if body.Facing&1 == 0 && pressed&ButtonLeft != 0 || body.Facing&1 != 0 && pressed&ButtonRight != 0 {
		body.Clock.Completed = 0
		enterChuSpecialGround(body, false)
	}
}

func spawnChuSpecialDust(state *ChuSpecialMotionState) error {
	pool, index, err := AllocateEffect(state.Pool)
	if err != nil {
		return err
	}
	state.Pool = pool
	if index < 0 {
		return nil
	}
	slot := &state.Pool.Slots[index]
	slot.Type = 0x10
	slot.VelocityX = -0x10000
	if state.Body.Facing != 0 {
		slot.VelocityX = 0x10000
	}
	slot.X, slot.Y = state.Body.X, state.Body.Y
	slot.Attributes = state.Body.Attributes&0x9800 | 0x2000
	slot.Depth, slot.Layer = state.Body.Depth, state.Body.Layer-4
	return nil
}

func spawnChuSpecialTrail(state *ChuSpecialMotionState) error {
	pool, index, err := AllocateEffect(state.Pool)
	if err != nil {
		return err
	}
	state.Pool = pool
	if index < 0 {
		return nil
	}
	slot := &state.Pool.Slots[index]
	slot.Type = 0x20
	slot.X, slot.Y = state.Body.X, state.Body.Ground
	slot.Depth, slot.Layer = state.Body.Depth, state.Body.Layer-4
	slot.Attributes = state.Body.Attributes&0x8000 | 0x4000
	if state.Body.Facing != 0 {
		slot.Attributes |= 0x800
	}
	return nil
}
