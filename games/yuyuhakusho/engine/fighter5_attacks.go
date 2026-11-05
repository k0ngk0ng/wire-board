package engine

import "errors"

// Fighter5AttackState retains one full long for +48. Its high word is used as
// a counter; the +4c byte variants remain views of Body.Gravity.
type Fighter5AttackState struct {
	ChuAttackState
	AlternateVelocity int32
}

// SelectFighter5Attack recovers Sensui's four complete source5 selector returns.
// Command recognition and the subsequent motion/child/tail stages run elsewhere.
func SelectFighter5Attack(state Fighter5AttackState, mode ChuAttackMode) (Fighter5AttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter5Ground(state)
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
		selectFighter5GroundSpecial(&state)
	case ChuAirSpecial:
		selectFighter5AirSpecial(&state)
	default:
		err = errors.New("unsupported source5 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter5Ground(state Fighter5AttackState) (Fighter5AttackState, error) {
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
			if err := submitFighter5Throw(&state.ChuAttackState); err != nil {
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

func submitFighter5Throw(state *ChuAttackState) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source5 throw submission storage is unavailable")
	}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = ThrowContactRecord{
		X: int16(state.Body.X >> 16), Y: int16(state.Body.Y>>16) - 0x28,
		Width: 0x18, Height: 0x28, SourcePose: 0xb8, TargetPose: 0x40,
		SourceAction: 3, TargetAction: 3, TargetFacing: 1, Owner: state.ObjectID,
		OffsetX: 0x18, OffsetY: 0,
	}
	state.Throws.Offset += 32
	return nil
}

func selectFighter5GroundSpecial(state *Fighter5AttackState) {
	changed := state.Controls.Changed
	switch state.Body.Request {
	case 1:
		if !spendChuEnergy(&state.ChuAttackState, 16) {
			return
		}
		pose := uint8(0xa4)
		if state.Controls.Held&ButtonDown != 0 {
			pose = 0xa6
		}
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x2e, pose, 0, false)
	case 3:
		if changed&(ButtonB|ButtonA) == 0 || !spendChuEnergy(&state.ChuAttackState, 8) {
			return
		}
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x2d, 0xa8, 0, false)
	case 4, 6:
		request := state.Body.Request
		if state.ChildSize != 0 || request == 6 && state.Body.Health > 111 {
			return
		}
		variant, ok := fighter5ProjectileVariant(changed)
		if !ok {
			return
		}
		cost := int16(4)
		if request == 6 {
			cost = 50
		}
		if !spendChuEnergy(&state.ChuAttackState, cost) {
			return
		}
		if request == 4 {
			setFighter5Projectile(state, variant, 0x2f)
		} else {
			state.Body.Gravity = int32(uint32(state.Body.Gravity)&0x00ffffff | uint32(variant)<<24)
			state.AlternateVelocity = int32(uint32(state.AlternateVelocity) & 0xffff)
			state.Body.Request = 0
			enterChuAttack(&state.ChuAttackState, 0x31, 0xa3, 0, false)
		}
	}
}

func fighter5ProjectileVariant(changed Buttons) (uint8, bool) {
	switch {
	case changed&ButtonA != 0:
		return 0, true
	case changed&ButtonB != 0:
		return 1, true
	case changed&ButtonC != 0:
		return 2, true
	default:
		return 0, false
	}
}

func selectFighter5AirSpecial(state *Fighter5AttackState) {
	if state.Body.Request != 4 || state.ChildSize != 0 {
		return
	}
	threshold := int16(uint16(state.Body.Ground>>16) - 128)
	if threshold > int16(state.Body.Y>>16) || state.Controls.Changed&(ButtonA|ButtonB|ButtonC) == 0 || !spendChuEnergy(&state.ChuAttackState, 4) {
		return
	}
	state.Body.VelocityX, state.Body.VerticalVelocity = 0x10000, 0x10000
	state.Body.Gravity = int32(uint32(state.Body.Gravity)&0x00ffffff | uint32(3)<<24)
	state.Body.Request = 0
	enterChuAttack(&state.ChuAttackState, 0x30, 0xac, 0, false)
}

// FinishFighter5Combination recovers 1926a/192ae. The motion caller chooses
// alternate when the preceding pose is not a4; neither constructor costs energy.
func FinishFighter5Combination(state Fighter5AttackState, alternate bool) Fighter5AttackState {
	pose := uint8(0xa5)
	if alternate {
		pose = 0xa7
	}
	state.Body.Request = 0
	enterChuAttack(&state.ChuAttackState, 3, pose, 0, false)
	return state
}

// BeginFighter5FinalProjectile recovers 19408/19426/19444 for variants 0..2.
// The existing +48 counter is left unchanged by these constructors.
func BeginFighter5FinalProjectile(state Fighter5AttackState, variant uint8) (Fighter5AttackState, error) {
	if variant > 2 {
		return state, errors.New("unsupported source5 final projectile variant")
	}
	setFighter5Projectile(&state, variant, 0x32)
	return state, nil
}

func setFighter5Projectile(state *Fighter5AttackState, variant, action uint8) {
	state.Body.VelocityX, state.Body.VerticalVelocity = 0x10000, 0x10000
	switch variant {
	case 0:
		state.Body.VelocityX, state.Body.VerticalVelocity = 0, 0x48000
	case 1:
		state.Body.VelocityX, state.Body.VerticalVelocity = 0x28000, 0
	}
	state.Body.Gravity = int32(uint32(state.Body.Gravity)&0x00ffffff | uint32(variant)<<24)
	state.Body.Request = 0
	enterChuAttack(&state.ChuAttackState, action, 0xa0+variant, 0, false)
}
