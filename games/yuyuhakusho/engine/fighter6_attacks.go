package engine

import "errors"

// Fighter6AttackState views the caller's existing Aux48 long. The source6
// slot33 constructor clears its big-endian high word and retains its low16.
// It introduces no second canonical velocity or counter owner.
type Fighter6AttackState struct {
	ChuAttackState
	AlternateVelocity int32
}

// SelectFighter6Attack recovers the complete four source6 selector returns.
// Air throws precede dash attempts; a failed energy check preserves earlier
// legal throw and Request8 effects before ordinary selection continues.
func SelectFighter6Attack(state Fighter6AttackState, mode ChuAttackMode) (Fighter6AttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectFighter6Ground(state)
	case ChuAirAttack:
		state, err = selectFighter6Air(state)
	case ChuGroundSpecial:
		selectFighter6Special(&state, false)
	case ChuAirSpecial:
		selectFighter6Special(&state, true)
	default:
		err = errors.New("unsupported source6 attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectFighter6Ground(state Fighter6AttackState) (Fighter6AttackState, error) {
	if state.Body.Request == 8 || state.Body.Request == 7 && state.Body.Pose != 11 {
		back := state.Body.Request == 8
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x0e, 11, 0x40000
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
	action, pose := uint8(3), uint8(0x80)
	switch state.Controls.Held & 15 {
	case 2, 6, 10:
		pose = 0x84
		if pressed&ButtonB == 0 {
			action, pose, state.Body.VelocityX = 0x13, 0x86, 0x40000
		}
	case 4, 5, 8, 9:
		if pressed&ButtonB == 0 {
			if err := submitFighter6Throw(&state, false); err != nil {
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
	enterChuAttack(&state.ChuAttackState, action, pose, 0, true)
	return state, nil
}

func selectFighter6Air(state Fighter6AttackState) (Fighter6AttackState, error) {
	direction := state.Controls.Held & 15
	if direction != 0 && direction != 1 && state.Controls.Pressed&ButtonA != 0 {
		if err := submitFighter6Throw(&state, true); err != nil {
			return state, err
		}
	}
	if state.Controls.Held&ButtonC != 0 && state.Controls.Pressed&ButtonB != 0 {
		state.Body.Request = 8
	}
	if state.Body.Request == 8 && spendChuEnergy(&state.ChuAttackState, 8) {
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x11, 12, 0x40000
		state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
		return state, nil
	}
	if state.Body.Pose != 11 && state.Body.Request == 7 && spendChuEnergy(&state.ChuAttackState, 8) {
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 0x0e, 0xae, 0x40000
		state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
		return state, nil
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
	return state, nil
}

func submitFighter6Throw(state *Fighter6AttackState, air bool) error {
	index := int(state.Throws.Offset / 32)
	if state.Throws.Offset&31 != 0 || index >= len(state.Throws.Records) {
		return errors.New("source6 throw submission storage is unavailable")
	}
	x, y := int16(state.Body.X>>16), int16(state.Body.Y>>16)
	record := ThrowContactRecord{X: x, Y: y - 0x22, Width: 0x18, Height: 0x22,
		SourcePose: 0xb8, TargetPose: 0x40, SourceAction: 3, TargetAction: 3,
		TargetFacing: 0, Owner: state.ObjectID, OffsetX: -0x18}
	if air {
		dx := int16(0x10)
		if state.Body.Facing != 0 {
			dx = -dx
		}
		record.X, record.Y, record.Width, record.Height = x+dx, y-0x10, 0x0c, 0x10
		record.SourceAction, record.TargetPose, record.TargetAction = 9, 0x41, 9
	}
	state.Throws.Records = append([]ThrowContactRecord(nil), state.Throws.Records...)
	state.Throws.Records[index] = record
	state.Throws.Offset += 32
	return nil
}

func selectFighter6Special(state *Fighter6AttackState, air bool) {
	request, changed := state.Body.Request, state.Controls.Changed
	cost, action, pose := int16(8), uint8(0x33), uint8(0xac)
	switch request {
	case 1:
		cost, action, pose = 16, 0x35, 0xa0
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			cost, action, pose = 24, 0x36, 0xa2
		}
	case 2:
		cost, action, pose = 24, 0x37, 0xa4
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			cost, pose = 32, 0xa5
		}
	case 3:
		if air {
			floor := int16(state.Body.Ground>>16) - 0x80
			if floor > int16(state.Body.Y>>16) {
				return
			}
		}
	case 4:
		if !air {
			return
		}
		action, pose = 0x34, 0xad
	case 6:
		if air || state.Body.Health > 111 || changed&(ButtonB|ButtonA) != ButtonB|ButtonA {
			return
		}
		cost, action, pose = 200, 0x38, 0xa8
	default:
		return
	}
	if !spendChuEnergy(&state.ChuAttackState, cost) {
		return
	}
	switch request {
	case 3:
		state.Body.VerticalVelocity = -0x60000
		state.AlternateVelocity = int32(uint32(state.AlternateVelocity) & 0xffff)
	case 4:
		state.Body.VerticalVelocity = 0x60000
	case 6:
		state.Body.VelocityX = 0
	}
	state.Body.Request = 0
	enterChuAttack(&state.ChuAttackState, action, pose, 0, false)
}
