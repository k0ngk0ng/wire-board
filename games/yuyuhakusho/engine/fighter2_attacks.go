package engine

import "errors"

// SelectFighter2Attack recovers Kuwabara's source2 four selector returns.
// Ground request1 has eleven structural direction entries and two separately
// CPU-verified constructor aliases12/14.11/13/15 are outside this semantic
// contract: they execute unrelated/context-dependent code or do not return in
// the diagnostic budget. An error here does not describe an original noop.
// Gravity and velocity aliases keep their existing canonical owners.
func SelectFighter2Attack(state ChuAttackState, mode ChuAttackMode) (ChuAttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter2Ground(state)
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
		err = selectFighter2GroundSpecial(&state)
	case ChuAirSpecial:
		// The original air selector has no direction, Changed or child gate.
		if state.Body.Request == 1 && spendChuEnergy(&state, 12) {
			state.Body.Request = 0
			enterChuAttack(&state, 2, 0xa3, 0, false)
		}
	default:
		err = errors.New("unsupported source2 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter2Ground(state ChuAttackState) (ChuAttackState, error) {
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
			if err := submitFighter2Throw(&state); err != nil {
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

func submitFighter2Throw(state *ChuAttackState) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source2 throw submission storage is unavailable")
	}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = ThrowContactRecord{
		X: int16(state.Body.X >> 16), Y: int16(state.Body.Y>>16) - 0x24,
		Width: 0x1a, Height: 0x24, SourcePose: 0xb8, TargetPose: 0x40,
		SourceAction: 3, TargetAction: 3, TargetFacing: 1, Owner: state.ObjectID,
		OffsetX: 0x18, OffsetY: 0,
	}
	state.Throws.Offset += 32
	return nil
}

func selectFighter2GroundSpecial(state *ChuAttackState) error {
	changed := state.Controls.Changed
	switch state.Body.Request {
	case 1:
		var pose uint8
		switch state.Controls.Held & 15 {
		case 0, 1, 3, 7, 12:
			pose = 0xa0
		case 4, 5, 8, 9, 14:
			pose = 0xa1
		case 2, 6, 10:
			pose = 0xa2
		default:
			return errors.New("source2 request1 direction is outside closed CPU-verified selector semantics")
		}
		if spendChuEnergy(state, 12) {
			state.Body.Request = 0
			enterChuAttack(state, 3, pose, 0, false)
		}
	case 3:
		if state.ChildSize != 0 {
			return nil
		}
		speed := int32(0x40000)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return nil
			}
			speed = 0x80000
		}
		if !spendChuEnergy(state, 18) {
			return nil
		}
		state.Body.VelocityX = speed
		state.Body.Gravity = int32(uint32(state.Body.Gravity) & 0xffff)
		state.Body.Request = 0
		enterChuAttack(state, 0x20, 0xa8, 0, false)
	case 4:
		if state.ChildSize != 0 || changed&0x70 == 0 || !spendChuEnergy(state, 20) {
			return nil
		}
		offset := uint32(0x30)
		if changed&ButtonC == 0 {
			offset = 0x70
			if changed&ButtonB == 0 {
				offset = 0xb0
			}
		}
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX)&0xffff | offset<<16)
		state.Body.Request = 0
		enterChuAttack(state, 0x1e, 0xa6, 0, false)
	case 5:
		pose := uint8(0xa4)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return nil
			}
			pose = 0xa5
		}
		if spendChuEnergy(state, 16) {
			state.Body.Request = 0
			enterChuAttack(state, 0x1f, pose, 0, false)
		}
	case 6:
		if state.Body.Health <= 111 && changed&ButtonA != 0 && spendChuEnergy(state, 200) {
			state.Body.Request = 0
			enterChuAttack(state, 3, 0xaa, 0, false)
		}
	}
	return nil
}
