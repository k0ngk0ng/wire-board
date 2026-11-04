package engine

import "errors"

// Fighter3AttackState adds only a view of the existing full Aux48 long.
// Gravity and velocity aliases remain in Body; the caller keeps ownership of
// energy fractions, the shared RNG and all later motion/child/tail stages.
type Fighter3AttackState struct {
	ChuAttackState
	AlternateVelocity int32
}

// SelectFighter3Attack recovers Yusuke's source3 four complete selector returns,
// including the ground throw/lunge and seven special constructor entries.
func SelectFighter3Attack(state Fighter3AttackState, mode ChuAttackMode) (Fighter3AttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter3Ground(state)
	case ChuAirAttack:
		// This source has no air-throw prelude.
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
		selectFighter3GroundSpecial(&state)
	case ChuAirSpecial:
		selectFighter3AirSpecial(&state)
	default:
		err = errors.New("unsupported source3 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter3Ground(state Fighter3AttackState) (Fighter3AttackState, error) {
	if state.Body.Request == 8 || state.Body.Request == 7 && state.Body.Pose != 11 {
		back := state.Body.Request == 8
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x0f, 11, 0x40000
		if back {
			state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x11, 12, 0x34000
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
			if err := submitFighter3Throw(&state.ChuAttackState); err != nil {
				return state, err
			}
			state.Body.VelocityX = 0x40000
			enterChuAttack(&state.ChuAttackState, 0x13, 0x83, 0, true)
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

func submitFighter3Throw(state *ChuAttackState) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source3 throw submission storage is unavailable")
	}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = ThrowContactRecord{
		X: int16(state.Body.X >> 16), Y: int16(state.Body.Y>>16) - 0x20,
		Width: 0x18, Height: 0x20, SourcePose: 0xb8, TargetPose: 0x40,
		SourceAction: 3, TargetAction: 3, TargetFacing: 1, Owner: state.ObjectID,
		OffsetX: 0x18, OffsetY: 0,
	}
	state.Throws.Offset += 32
	return nil
}

func selectFighter3GroundSpecial(state *Fighter3AttackState) {
	changed := state.Controls.Changed
	switch state.Body.Request {
	case 1, 2:
		cost, pose, button := int16(12), uint8(0xa0), ButtonB
		if state.Body.Request == 2 {
			cost, pose, button = 16, 0xa1, ButtonA
		}
		if state.Controls.Held&15 != 0 || changed&button == 0 || !spendChuEnergy(&state.ChuAttackState, cost) {
			return
		}
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX) & 0x00ffffff)
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x15, pose, 0, false)
	case 3:
		if changed&0x70 == 0 || !spendChuEnergy(&state.ChuAttackState, 32) {
			return
		}
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX) & 0x00ffffff)
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x21, 0xa6, 0, false)
	case 4:
		if state.ChildSize != 0 || changed&0x70 == 0 || !spendChuEnergy(&state.ChuAttackState, 4) {
			return
		}
		variant := uint32(0)
		if changed&ButtonC == 0 {
			variant = 1
			if changed&ButtonB == 0 {
				variant = 2
			}
		}
		state.Body.Gravity = int32(uint32(state.Body.Gravity)&0x00ffffff | variant<<24)
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX)&0xffff | uint32(32)<<16)
		state.Body.VerticalVelocity = int32(uint32(state.Body.VerticalVelocity)&0xffff | uint32(0xffd2)<<16)
		state.Body.Request = 0
		enterChuAttack(&state.ChuAttackState, 0x17, 0xa4, 0, false)
	case 6:
		if state.Body.Health > 111 || changed&ButtonA == 0 || !spendChuEnergy(&state.ChuAttackState, 200) {
			return
		}
		state.Body.VelocityX, state.Body.Request = 0x40000, 0
		enterChuAttack(&state.ChuAttackState, 0x23, 0xaa, 0, false)
	}
}

func selectFighter3AirSpecial(state *Fighter3AttackState) {
	if state.Body.Request != 4 {
		return
	}
	// SUB.W wraps the ground high word before the signed comparison.
	threshold := int16(uint16(state.Body.Ground>>16) - 128)
	if threshold > int16(state.Body.Y>>16) || state.ChildSize != 0 {
		return
	}
	speed := int32(0x40000)
	if state.Controls.Changed&ButtonB == 0 {
		if state.Controls.Changed&ButtonA == 0 {
			return
		}
		speed = 0x60000
	}
	if !spendChuEnergy(&state.ChuAttackState, 24) {
		return
	}
	state.AlternateVelocity = speed
	state.Body.VelocityX = int32(uint32(state.Body.VelocityX)&0xffff | uint32(28)<<16)
	state.Body.VerticalVelocity = int32(uint32(state.Body.VerticalVelocity)&0xffff | uint32(0xffd0)<<16)
	state.Body.Request = 0
	enterChuAttack(&state.ChuAttackState, 0x22, 0xac, 0, false)
}
