package engine

import "errors"

// SelectFighter0Attack recovers Genkai's four source0 selector returns and
// immediate constructors. Gravity's highest byte and velocity high words keep
// their original aliases in Body; no second mode or projectile-speed store is
// introduced. Continued motion, children and the shared action tail are caller
// stages. Energy fractions, Aux48 and the world RNG are not owned here.
func SelectFighter0Attack(state ChuAttackState, mode ChuAttackMode) (ChuAttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter0Ground(state)
	case ChuAirAttack:
		direction := state.Controls.Held & 15
		if direction != 0 && direction != 1 && state.Controls.Pressed&ButtonA != 0 {
			if err = submitFighter0Throw(&state, true); err != nil {
				return original, err
			}
		}
		if state.Controls.Pressed&ButtonB != 0 {
			enterChuAttack(&state, 2, 0x90, 0x10, false)
		} else if state.Controls.Pressed&ButtonA != 0 {
			pose := uint8(0x94)
			switch direction {
			case 0, 2, 4, 6, 8, 10:
				pose = 0x95
			}
			enterChuAttack(&state, 2, pose, 0x10, false)
		}
	case ChuGroundSpecial:
		selectFighter0GroundSpecial(&state)
	case ChuAirSpecial:
		selectFighter0AirSpecial(&state)
	default:
		err = errors.New("unsupported source0 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter0Ground(state ChuAttackState) (ChuAttackState, error) {
	if state.Body.Request == 8 || state.Body.Request == 7 && state.Body.Pose != 11 {
		back := state.Body.Request == 8
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x0e, 11, 0x34000
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
			if err := submitFighter0Throw(&state, false); err != nil {
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

func submitFighter0Throw(state *ChuAttackState, air bool) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source0 throw submission storage is unavailable")
	}
	x, y := int16(state.Body.X>>16), int16(state.Body.Y>>16)
	record := ThrowContactRecord{X: x, Y: y - 0x1c, Width: 0x14, Height: 0x1c,
		SourcePose: 0xb8, TargetPose: 0x40, SourceAction: 3, TargetAction: 3,
		TargetFacing: 1, Owner: state.ObjectID, OffsetX: 0x18, OffsetY: 0}
	if air {
		dx := int16(0x10)
		if state.Body.Facing != 0 {
			dx = -dx
		}
		record.X, record.Y = x+dx, y-0x30
		record.Width, record.Height = 0x10, 0x10
		record.SourcePose, record.TargetPose, record.SourceAction, record.TargetAction = 0xb9, 0x41, 9, 9
	}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = record
	state.Throws.Offset += 32
	return nil
}

func selectFighter0GroundSpecial(state *ChuAttackState) {
	changed := state.Controls.Changed
	switch state.Body.Request {
	case 1, 2:
		cost, pose, button := int16(8), uint8(0xa0), ButtonB
		if state.Body.Request == 2 {
			cost, pose, button = 12, 0xa1, ButtonA
		}
		if state.Controls.Held&15 != 0 || changed&button == 0 || !spendChuEnergy(state, cost) {
			return
		}
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX) & 0x00ffffff)
		state.Body.Request = 0
		enterChuAttack(state, 0x15, pose, 0, false)
	case 3:
		cost, pose, vy := int16(12), uint8(0xa2), int32(-0x40000)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			cost, pose, vy = 16, 0xa3, -0x60000
		}
		if !spendChuEnergy(state, cost) {
			return
		}
		state.Body.VelocityX, state.Body.VerticalVelocity, state.Body.Request = 0x40000, vy, 0
		enterChuAttack(state, 0x16, pose, 0, false)
	case 4:
		if state.ChildSize != 0 || changed&0x70 == 0 || !spendChuEnergy(state, 4) {
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
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX)&0xffff | uint32(0x1c)<<16)
		state.Body.VerticalVelocity = int32(uint32(state.Body.VerticalVelocity)&0xffff | uint32(0xffd8)<<16)
		state.Body.Request = 0
		enterChuAttack(state, 0x17, 0xa4, 0, false)
	case 5:
		if state.ChildSize != 0 {
			return
		}
		cost, pose := int16(16), uint8(0xa6)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			cost, pose = 32, 0xa7
		}
		if !spendChuEnergy(state, cost) {
			return
		}
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX) & 0xffff)
		state.Body.Request = 0
		enterChuAttack(state, 0x18, pose, 0, false)
	case 6:
		if state.Body.Health > 111 || changed&ButtonA == 0 || !spendChuEnergy(state, 200) {
			return
		}
		state.Body.VelocityX, state.Body.Request = 0x40000, 0
		enterChuAttack(state, 0x19, 0xaa, 0, false)
	}
}

func selectFighter0AirSpecial(state *ChuAttackState) {
	changed := state.Controls.Changed
	switch state.Body.Request {
	case 1, 2:
		cost, pose, button := int16(8), uint8(0xac), ButtonB
		if state.Body.Request == 2 {
			cost, pose, button = 12, 0xad, ButtonA
		}
		if state.Controls.Held&15 != 0 || changed&button == 0 || !spendChuEnergy(state, cost) {
			return
		}
		state.Body.Request = 0
		enterChuAttack(state, 2, pose, 0, false)
	case 3:
		pose, vy := uint8(0xae), int32(-0x40000)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			pose, vy = 0xaf, -0x60000
		}
		if !spendChuEnergy(state, 16) {
			return
		}
		state.Body.VelocityX, state.Body.VerticalVelocity, state.Body.Request = 0x40000, vy, 0
		enterChuAttack(state, 0x16, pose, 0, false)
	}
}
