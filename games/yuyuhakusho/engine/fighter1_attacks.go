package engine

import "errors"

// Fighter1AttackState recovers source1 selection and immediate initialization.
// Power is the caller's existing +7c word. Energy fractions, children, RNG and
// continued motion remain owned by the caller.
type Fighter1AttackState struct {
	ChuAttackState
	Power int16
}

// SelectFighter1Attack recovers the four complete source1 selector returns.
func SelectFighter1Attack(state Fighter1AttackState, mode ChuAttackMode) (Fighter1AttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter1Ground(state)
	case ChuAirAttack:
		if state.Controls.Pressed&ButtonB != 0 {
			enterChuAttack(&state.ChuAttackState, 2, 0x90, 0x10, false)
		} else if state.Controls.Pressed&ButtonA != 0 {
			pose := uint8(0x94)
			switch state.Controls.Held & 15 {
			case 0, 2, 4, 6, 8, 10:
				pose = 0x95
			}
			enterChuAttack(&state.ChuAttackState, 2, pose, 0x10, false)
		}
	case ChuGroundSpecial:
		selectFighter1GroundSpecial(&state)
	case ChuAirSpecial:
		// The original 0x1668c is exactly RTS; no state is read or changed.
	default:
		err = errors.New("unsupported source1 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter1Ground(state Fighter1AttackState) (Fighter1AttackState, error) {
	if state.Body.Request == 8 || state.Body.Request == 7 && state.Body.Pose != 11 {
		back := state.Body.Request == 8
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x0f, 11, 0x40000
		if back {
			state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x11, 12, 0x38000
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
			if err := submitFighter1Throw(&state); err != nil {
				return state, err
			}
			enterChuAttack(&state.ChuAttackState, 3, 0x83, 0, true)
			return state, nil
		}
		pose = 0x81
	default:
		if pressed&ButtonB == 0 {
			pose = 0x82
		}
	}
	enterChuAttack(&state.ChuAttackState, 3, pose, 0, true)
	return state, nil
}

func submitFighter1Throw(state *Fighter1AttackState) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source1 throw submission storage is unavailable")
	}
	record := ThrowContactRecord{X: int16(state.Body.X >> 16), Y: int16(state.Body.Y>>16) - 0x24,
		Width: 0x18, Height: 0x24, SourcePose: 0xb8, TargetPose: 0x40,
		SourceAction: 3, TargetAction: 3, TargetFacing: 1, Owner: state.ObjectID, OffsetX: 0x18}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = record
	state.Throws.Offset += 32
	return nil
}

func selectFighter1GroundSpecial(state *Fighter1AttackState) {
	request, changed := state.Body.Request, state.Controls.Changed
	if request >= 1 && request <= 4 && state.ChildSize != 0 {
		return
	}
	cost, action, pose := int16(16), uint8(3), uint8(0xa5)
	switch request {
	case 1:
		action, pose = 0x1c, 0xa0
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			cost, pose = 24, 0xa1
		}
	case 2:
		action, pose = 0x1b, 0xa2
		if changed&(ButtonB|ButtonA) == 0 {
			return
		}
	case 3:
	case 4:
		cost, action, pose = 24, 0x1a, 0xa3
		if changed&(ButtonC|ButtonB|ButtonA) == 0 {
			return
		}
	case 6:
		if state.Body.Health > 111 || changed&ButtonA == 0 {
			return
		}
		cost, action, pose = 200, 0x1d, 0xa6
	default:
		return
	}
	if !spendChuEnergy(&state.ChuAttackState, cost) {
		return
	}
	switch request {
	case 1:
		state.Power = 0
	case 2:
		state.Body.VelocityX = 0x40000
		if changed&ButtonB == 0 {
			state.Body.VelocityX = 0x60000
		}
	case 4:
		word := uint32(0x30)
		if changed&ButtonC == 0 {
			word = 0x70
			if changed&ButtonB == 0 {
				word = 0xb0
			}
		}
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX)&0xffff | word<<16)
	}
	state.Body.Request = 0
	enterChuAttack(&state.ChuAttackState, action, pose, 0, false)
}
