package engine

import "errors"

// Fighter7AttackState views the caller's canonical Aux48 long during selection.
// Its high byte shares storage with the remaining 24 bits; no separate variant
// or counter survives this call.
type Fighter7AttackState struct {
	ChuAttackState
	AlternateVelocity int32
}

// SelectFighter7Attack recovers source7's four complete selector returns.
// Command recognition, continued motion, children and the shared frame tail
// remain separate stages. Energy is the signed high word of its existing long.
func SelectFighter7Attack(state Fighter7AttackState, mode ChuAttackMode) (Fighter7AttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter7Ground(state)
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
		selectFighter7GroundSpecial(&state)
	case ChuAirSpecial:
		selectFighter7AirSpecial(&state)
	default:
		err = errors.New("unsupported source7 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter7Ground(state Fighter7AttackState) (Fighter7AttackState, error) {
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
	case 2:
		pose = 0x84
		if pressed&ButtonB == 0 {
			pose = 0x86
		}
	case 6, 10:
		pose = 0x84
		if pressed&ButtonB == 0 {
			state.Body.VelocityX = 0x40000
			enterChuAttack(&state.ChuAttackState, 0x13, 0x87, 0, true)
			return state, nil
		}
	case 4, 5, 8, 9:
		if pressed&ButtonB == 0 {
			if err := submitFighter7Throw(&state.ChuAttackState); err != nil {
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

func submitFighter7Throw(state *ChuAttackState) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source7 throw submission storage is unavailable")
	}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = ThrowContactRecord{
		X: int16(state.Body.X >> 16), Y: int16(state.Body.Y>>16) - 0x24,
		Width: 0x18, Height: 0x24, SourcePose: 0xb8, TargetPose: 0x40,
		SourceAction: 3, TargetAction: 3, TargetFacing: 1, Owner: state.ObjectID,
		OffsetX: 0x10, OffsetY: 0,
	}
	state.Throws.Offset += 32
	return nil
}

func selectFighter7GroundSpecial(state *Fighter7AttackState) {
	changed := state.Controls.Changed
	switch state.Body.Request {
	case 1:
		vx, vy := int32(0x40000), int32(-0x20000)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			vx, vy = 0x60000, -0x30000
		}
		if !spendChuEnergy(&state.ChuAttackState, 16) {
			return
		}
		if state.Body.Facing&1 != 0 {
			vx = -vx
		}
		state.Body.VelocityX, state.Body.VerticalVelocity, state.Body.Gravity = vx, vy, 0x3800
		state.AlternateVelocity = int32(uint32(state.AlternateVelocity) & 0x00ffffff)
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x39, 0xa0, 0, false)
	case 2:
		if state.ChildSize != 0 || changed&(ButtonC|ButtonB|ButtonA) == 0 || !spendChuEnergy(&state.ChuAttackState, 16) {
			return
		}
		distance := uint32(0x30)
		if changed&ButtonC == 0 {
			distance = 0x70
			if changed&ButtonB == 0 {
				distance = 0xb0
			}
		}
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX)&0xffff | distance<<16)
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x3b, 0xa6, 0, false)
	case 3:
		selectFighter7Warp(state)
	case 4, 6:
		if state.Body.Health > 111 || state.Controls.Held&ButtonDown == 0 || !spendChuEnergy(&state.ChuAttackState, 200) {
			return
		}
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x3d, 0xaa, 0, false)
	}
}

func selectFighter7AirSpecial(state *Fighter7AttackState) {
	switch state.Body.Request {
	case 2:
		cost, pose, vx, vy := int16(16), uint8(0xae), int32(0x40000), int32(0x40000)
		if state.Controls.Changed&ButtonB == 0 {
			if state.Controls.Changed&ButtonA == 0 {
				return
			}
			cost, pose, vx, vy = 20, 0xaf, 0x80000, 0x30000
		}
		if !spendChuEnergy(&state.ChuAttackState, cost) {
			return
		}
		state.Body.VelocityX, state.Body.VerticalVelocity, state.Body.Request = vx, vy, 0
		enterChuAttack(&state.ChuAttackState, 0x45, pose, 0, true)
	case 3:
		selectFighter7Warp(state)
	}
}

func selectFighter7Warp(state *Fighter7AttackState) {
	changed := state.Controls.Changed
	if changed&(ButtonC|ButtonB|ButtonA) == 0 || !spendChuEnergy(&state.ChuAttackState, 16) {
		return
	}
	variant := uint32(2)
	if changed&ButtonC == 0 {
		variant = 1
		if changed&ButtonB == 0 {
			variant = 0
		}
	}
	state.AlternateVelocity = int32(uint32(state.AlternateVelocity)&0x00ffffff | variant<<24)
	state.Body.Request = 0
	enterChuAttack(&state.ChuAttackState, 0x3a, 0xa4, 0, false)
}
