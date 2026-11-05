package engine

import "errors"

// SelectFighter4Attack recovers the four complete source4 selector returns.
func SelectFighter4Attack(state ChuAttackState, mode ChuAttackMode) (ChuAttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter4Ground(state)
	case ChuAirAttack:
		if state.Controls.Pressed&ButtonB != 0 {
			enterChuAttack(&state, 2, 0x90, 0x10, false)
		} else if state.Controls.Pressed&ButtonA != 0 {
			pose := uint8(0x94)
			switch state.Controls.Held & 15 {
			case 0, 2, 4, 6, 8, 10:
				pose = 0x95
			}
			enterChuAttack(&state, 2, pose, 0x10, false)
		}
	case ChuGroundSpecial:
		selectFighter4GroundSpecial(&state)
	case ChuAirSpecial:
		selectFighter4AirSpecial(&state)
	default:
		err = errors.New("unsupported source4 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter4Ground(state ChuAttackState) (ChuAttackState, error) {
	if state.Body.Request == 8 || state.Body.Request == 7 && state.Body.Pose != 11 {
		back := state.Body.Request == 8
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x0e, 11, 0x3c000
		if back {
			state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x12, 12, 0x38000
		}
		pool, index, err := AllocateEffect(state.Pool)
		if err != nil {
			return state, err
		}
		state.Pool = pool
		if index >= 0 {
			slot := &state.Pool.Slots[index]
			slot.Type, slot.VelocityX = 0x10, -0x10000
			if back {
				slot.VelocityX = -slot.VelocityX
			}
			if state.Body.Facing != 0 {
				slot.VelocityX = -slot.VelocityX
			}
			slot.X, slot.Y = state.Body.X, state.Body.Y
			slot.Attributes = state.Body.Attributes&0x9800 | 0x2000
			slot.Depth, slot.Layer = state.Body.Depth, state.Body.Layer-4
		}
		state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
		return state, nil
	}
	pressed := state.Controls.Pressed
	if pressed&(ButtonB|ButtonA) == 0 {
		return state, nil
	}
	pose := uint8(0x80)
	switch state.Controls.Held & 15 {
	case 2, 6, 10:
		pose = 0x84
		if pressed&ButtonB == 0 {
			pose = 0x86
		}
	case 4, 5, 8, 9:
		if pressed&ButtonB == 0 {
			if err := submitFighter4Throw(&state); err != nil {
				return state, err
			}
			enterChuAttack(&state, 3, 0x83, 0, true)
			return state, nil
		}
		pose = 0x81
	default:
		if pressed&ButtonB == 0 {
			pose = 0x82
		}
	}
	enterChuAttack(&state, 3, pose, 0, true)
	return state, nil
}

func submitFighter4Throw(state *ChuAttackState) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source4 throw submission storage is unavailable")
	}
	record := ThrowContactRecord{X: int16(state.Body.X >> 16), Y: int16(state.Body.Y>>16) - 0x1c,
		Width: 0x16, Height: 0x1c, SourcePose: 0xb8, TargetPose: 0x40,
		SourceAction: 3, TargetAction: 3, TargetFacing: 1, Owner: state.ObjectID, OffsetX: 0x18}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = record
	state.Throws.Offset += 32
	return nil
}

func selectFighter4GroundSpecial(state *ChuAttackState) {
	request, changed := state.Body.Request, state.Controls.Changed
	cost, action, pose := int16(8), uint8(0x25), uint8(0xa0)
	switch request {
	case 1:
	case 2:
		cost, action, pose = 32, 3, 0xa2
	case 3:
		cost, action, pose = 24, 0x27, 0xa5
		if changed&ButtonA == 0 {
			if changed&ButtonB == 0 {
				return
			}
			cost, action, pose = 16, 0x26, 0xa4
		}
	case 4:
		if changed&(ButtonA|ButtonB) == 0 {
			return
		}
		cost, action, pose = 24, 0x2c, 0xa8
	case 6:
		if state.Body.Health > 111 || state.ChildSize != 0 || changed&(ButtonB|ButtonA) == 0 {
			return
		}
		cost, action, pose = 200, 0x29, 0xa6
	default:
		return
	}
	if !spendChuEnergy(state, cost) {
		return
	}
	switch request {
	case 1:
		state.Body.VelocityX = 0x28000
		if state.Body.Facing&1 != 0 {
			state.Body.VelocityX = -state.Body.VelocityX
		}
	case 4:
		state.Body.VerticalVelocity, state.Body.Gravity, state.Body.VelocityX = -0x68000, 0x3200, 0x18000
		if changed&ButtonA == 0 {
			state.Body.Gravity = 0x5000
		}
		if state.Body.Facing&1 != 0 {
			state.Body.VelocityX = -state.Body.VelocityX
		}
	}
	state.Body.Request = 0
	enterChuAttack(state, action, pose, 0, false)
}

func selectFighter4AirSpecial(state *ChuAttackState) {
	if state.Body.Request != 3 {
		return
	}
	cost, pose := int16(24), uint8(0xad)
	if state.Controls.Changed&ButtonA == 0 {
		if state.Controls.Changed&ButtonB == 0 {
			return
		}
		cost, pose = 16, 0xac
	}
	if !spendChuEnergy(state, cost) {
		return
	}
	state.Body.Request = 0
	enterChuAttack(state, 0x28, pose, 0, false)
}

// FinishFighter4Rush recovers 0x19092. The pending request is preserved.
func FinishFighter4Rush(state ChuAttackState) ChuAttackState {
	enterChuAttack(&state, 3, 0xa1, 0, false)
	return state
}

// BeginFighter4DragonFollowup recovers 0x191ba after caller-owned motion work.
// MovementSource and its parameter view remain unchanged by this constructor.
func BeginFighter4DragonFollowup(state ChuAttackState) ChuAttackState {
	state.Body.Request = 0
	enterChuAttack(&state, 0x2a, 0xa7, 0, false)
	return state
}
