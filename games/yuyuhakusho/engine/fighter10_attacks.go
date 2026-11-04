package engine

import "errors"

// Fighter10AttackState owns source10 selection and immediate initialization.
// Aux48High is only the highest byte of the caller's canonical Aux48 long;
// the caller retains its low 24 bits. EnergyFraction and RandomSeed pass through
// unchanged. Child updates, continued motion and the shared tail are separate.
type Fighter10AttackState struct {
	ChuAttackState
	Aux48High      uint8
	EnergyFraction uint16
	RandomSeed     uint32
}

// SelectFighter10Attack recovers the four original source10 selector returns,
// including ordinary/air throws and the seven reached special constructors.
// The mode constants are shared with the existing source9 selector contract.
func SelectFighter10Attack(state Fighter10AttackState, mode ChuAttackMode) (Fighter10AttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter10Ground(state)
	case ChuAirAttack:
		direction := state.Controls.Held & 15
		if direction != 0 && direction != 1 && state.Controls.Pressed&ButtonA != 0 {
			err = submitFighter10Throw(&state, true)
			if err != nil {
				return original, err
			}
		}
		if state.Controls.Pressed&ButtonB != 0 {
			enterChuAttack(&state.ChuAttackState, 2, 0x90, 0x10, false)
		} else if state.Controls.Pressed&ButtonA != 0 {
			pose := uint8(0x94)
			switch direction {
			case 0, 2, 4, 6, 8, 10:
				pose = 0x95
			}
			enterChuAttack(&state.ChuAttackState, 2, pose, 0x10, false)
		}
	case ChuGroundSpecial:
		selectFighter10GroundSpecial(&state)
	case ChuAirSpecial:
		selectFighter10AirSpecial(&state)
	default:
		err = errors.New("unsupported source10 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter10Ground(state Fighter10AttackState) (Fighter10AttackState, error) {
	if state.Body.Request == 8 || state.Body.Request == 7 && state.Body.Pose != 11 {
		back := state.Body.Request == 8
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x0f, 11, 0x38000
		if back {
			state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x11, 12, 0x40000
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
			if err := submitFighter10Throw(&state, false); err != nil {
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

func submitFighter10Throw(state *Fighter10AttackState, air bool) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source10 throw submission storage is unavailable")
	}
	x, y := int16(state.Body.X>>16), int16(state.Body.Y>>16)
	dx := int16(8)
	record := ThrowContactRecord{X: x, Y: y - 0x28, Width: 0x18, Height: 0x28,
		SourcePose: 0xb8, TargetPose: 0x41, SourceAction: 3, TargetAction: 9,
		TargetFacing: 1, Owner: state.ObjectID, OffsetX: 0x20, OffsetY: -0x30}
	if air {
		dx = 0x18
		record.Y, record.Height = y-0x30, 8
		record.SourcePose, record.SourceAction = 0xb9, 9
		record.OffsetX, record.OffsetY = 0x18, 0
	}
	if state.Body.Facing != 0 {
		dx = -dx
	}
	record.X = x + dx
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = record
	state.Throws.Offset += 32
	return nil
}

func selectFighter10GroundSpecial(state *Fighter10AttackState) {
	changed := state.Controls.Changed
	switch state.Body.Request {
	case 1:
		if !spendChuEnergy(&state.ChuAttackState, 24) {
			return
		}
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 3, 0xa1, 0, false)
	case 2:
		if changed&0x70 == 0 || !spendChuEnergy(&state.ChuAttackState, 16) {
			return
		}
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x2d, 0xa0, 0, false)
	case 3:
		pose := uint8(0xa2)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			pose = 0xa3
		}
		if !spendChuEnergy(&state.ChuAttackState, 16) {
			return
		}
		state.Body.VelocityX, state.Body.Request = 0x48000, 0
		enterChuAttack(&state.ChuAttackState, 0x47, pose, 0, false)
	case 6:
		if state.Body.Health > 111 || changed&ButtonA == 0 || !spendChuEnergy(&state.ChuAttackState, 200) {
			return
		}
		state.Aux48High, state.Body.Request = 0, 0
		enterChuAttack(&state.ChuAttackState, 0x49, 0xaa, 0, false)
	}
}

func selectFighter10AirSpecial(state *Fighter10AttackState) {
	if state.Body.Request != 2 {
		return
	}
	cost, pose, vx, vy := int16(16), uint8(0xac), int32(0x38000), int32(0x38000)
	if state.Controls.Changed&ButtonB == 0 {
		if state.Controls.Changed&ButtonA == 0 {
			return
		}
		cost, pose, vx, vy = 20, 0xad, 0x60000, 0x28000
	}
	if !spendChuEnergy(&state.ChuAttackState, cost) {
		return
	}
	state.Body.VelocityX, state.Body.VerticalVelocity, state.Body.Request = vx, vy, 0
	enterChuAttack(&state.ChuAttackState, 0x48, pose, 0, true)
}
