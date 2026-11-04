package engine

import "errors"

// Fighter8AttackState views the caller's existing full Aux48 long. The slot40
// constructor clears its big-endian high word only; low16 remains caller data.
// No second counter, velocity or canonical storage owner is introduced.
type Fighter8AttackState struct {
	ChuAttackState
	AlternateVelocity int32
}

// SelectFighter8Attack recovers source8's four complete selector returns and
// ten reached special roots. Motion, children, clocks and the shared tail are
// separate stages. Energy fractions and the world RNG remain caller-owned.
func SelectFighter8Attack(state Fighter8AttackState, mode ChuAttackMode) (Fighter8AttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter8Ground(state)
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
		selectFighter8GroundSpecial(&state)
	case ChuAirSpecial:
		if state.Body.Request == 3 {
			selectFighter8Flight(&state)
		}
	default:
		err = errors.New("unsupported source8 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter8Ground(state Fighter8AttackState) (Fighter8AttackState, error) {
	if state.Body.Request == 8 || state.Body.Request == 7 && state.Body.Pose != 11 {
		back := state.Body.Request == 8
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x0f, 11, 0x34000
		if back {
			state.Body.ActionKind, state.Body.Pose = 0x11, 12
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
			if err := submitFighter8Throw(&state.ChuAttackState); err != nil {
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

func submitFighter8Throw(state *ChuAttackState) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source8 throw submission storage is unavailable")
	}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = ThrowContactRecord{
		X: int16(state.Body.X >> 16), Y: int16(state.Body.Y>>16) - 0x1c,
		Width: 0x14, Height: 0x1c, SourcePose: 0xb8, TargetPose: 0x40,
		SourceAction: 3, TargetAction: 3, TargetFacing: 1, Owner: state.ObjectID,
		OffsetX: 0x18, OffsetY: 0,
	}
	state.Throws.Offset += 32
	return nil
}

func selectFighter8GroundSpecial(state *Fighter8AttackState) {
	changed := state.Controls.Changed
	switch state.Body.Request {
	case 1:
		if state.Controls.Held&15 != 0 || !spendChuEnergy(&state.ChuAttackState, 24) {
			return
		}
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 3, 0xa0, 0, false)
	case 2:
		cost, pose := int16(24), uint8(0xa3)
		if changed&ButtonA == 0 {
			cost, pose = 16, 0xa2
			if changed&ButtonB == 0 {
				if changed&ButtonC == 0 {
					return
				}
				cost, pose = 8, 0xa1
			}
		}
		if !spendChuEnergy(&state.ChuAttackState, cost) {
			return
		}
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 3, pose, 0, false)
	case 3:
		selectFighter8Flight(state)
	case 4:
		if state.ChildSize != 0 {
			return
		}
		speed := int32(0xb00000)
		if changed&ButtonA == 0 {
			speed = 0x700000
			if changed&ButtonB == 0 {
				if changed&ButtonC == 0 {
					return
				}
				speed = 0x300000
			}
		}
		if !spendChuEnergy(&state.ChuAttackState, 8) {
			return
		}
		state.Body.VelocityX, state.Body.Request = speed, 0
		enterChuAttack(&state.ChuAttackState, 0x3f, 0xa7, 0, false)
	case 6:
		if state.Body.Health > 111 || changed&ButtonA == 0 || !spendChuEnergy(&state.ChuAttackState, 200) {
			return
		}
		state.Body.VelocityX, state.Body.Request = 0, 0
		state.AlternateVelocity = int32(uint32(state.AlternateVelocity) & 0xffff)
		enterChuAttack(&state.ChuAttackState, 0x40, 0xac, 0, false)
	}
}

func selectFighter8Flight(state *Fighter8AttackState) {
	speed := int32(0x40000)
	if state.Controls.Changed&ButtonB == 0 {
		if state.Controls.Changed&ButtonA == 0 {
			return
		}
		speed = 0x80000
	}
	if !spendChuEnergy(&state.ChuAttackState, 16) {
		return
	}
	state.Body.VelocityX, state.Body.VerticalVelocity, state.Body.Request = speed, 0, 0
	enterChuAttack(&state.ChuAttackState, 0x3e, 0xa4, 0, false)
}
