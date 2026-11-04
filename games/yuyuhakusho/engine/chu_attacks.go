package engine

import "errors"

type ChuAttackMode uint8

const (
	ChuGroundAttack ChuAttackMode = iota
	ChuAirAttack
	ChuGroundSpecial
	ChuAirSpecial
)

type ChuAttackState struct {
	Body             FighterBodyState
	Controls         FighterControls
	ObjectID         uint16
	Energy           int16 // The high word at +b8; the low word is independent.
	ChildSize        uint16
	GravityParameter int32
	Pool             EffectPool
	Throws           ThrowContactBuffer
}

// SelectChuAttack recovers all four Chu selectors and their complete
// tail-called initialization. Command recognition happens before this stage;
// movement, projectiles and animation following the selection happen later.
func SelectChuAttack(state ChuAttackState, mode ChuAttackMode) (ChuAttackState, error) {
	original := state
	var err error
	switch mode {
	case ChuGroundAttack:
		state, err = selectChuGround(state)
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
		selectChuGroundSpecial(&state)
	case ChuAirSpecial:
		selectChuAirSpecial(&state)
	default:
		err = errors.New("unsupported Chu attack selector")
	}
	if err != nil {
		return original, err
	}
	return state, nil
}

func selectChuGround(state ChuAttackState) (ChuAttackState, error) {
	if state.Body.Request == 8 || state.Body.Request == 7 && state.Body.Pose != 11 {
		back := state.Body.Request == 8
		state.Body.ActionKind, state.Body.Pose, state.Body.VelocityX = 15, 11, 0x40000
		if back {
			state.Body.Pose, state.Body.VelocityX = 12, -0x40000
		}
		pool, index, err := AllocateEffect(state.Pool)
		if err != nil {
			return state, err
		}
		state.Pool = pool
		if index >= 0 {
			slot := &state.Pool.Slots[index]
			slot.Type = 0x10
			slot.VelocityX = -0x10000
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
	direction := state.Controls.Held & 15
	pose := uint8(0x80)
	switch direction {
	case 2, 6, 10:
		pose = 0x84
		if pressed&ButtonB == 0 {
			pose = 0x86
		}
	case 4, 5, 8, 9:
		if pressed&ButtonB == 0 {
			next, err := SubmitChuThrow(ChuThrowSubmissionState{Actor: state.Body.GroundActionState, ObjectID: state.ObjectID,
				X: state.Body.X, Y: state.Body.Y, Held: uint8(state.Controls.Held), Buffer: state.Throws})
			if err != nil {
				return state, err
			}
			state.Body.GroundActionState, state.Throws = next.Actor, next.Buffer
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

func enterChuAttack(state *ChuAttackState, action, pose, stages uint8, setFacing bool) {
	state.Body.ActionKind, state.Body.Pose, state.Body.EnabledStages = action, pose, stages
	if setFacing {
		if state.Controls.Held&ButtonRight != 0 {
			state.Body.Facing = 0
		} else if state.Controls.Held&ButtonLeft != 0 {
			state.Body.Facing = 1
		}
	}
	state.Body.GroundActionState = ResetActionState(state.Body.GroundActionState)
}

func spendChuEnergy(state *ChuAttackState, cost int16) bool {
	if state.Energy < cost {
		return false
	}
	state.Energy -= cost
	return true
}

func selectChuGroundSpecial(state *ChuAttackState) {
	changed := state.Controls.Changed
	direction := state.Controls.Held & 15
	switch state.Body.Request {
	case 1, 2:
		cost, pose, button := int16(12), uint8(0xa0), ButtonB
		if state.Body.Request == 2 {
			cost, pose, button = 16, 0xa1, ButtonA
		}
		if direction != 0 || changed&button == 0 || !spendChuEnergy(state, cost) {
			return
		}
		// MOVE.B clears only the velocity long's highest byte.
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX) & 0x00ffffff)
		state.Body.Request = 0
		enterChuAttack(state, 0x15, pose, 0, false)
	case 3:
		cost, speed := int16(16), int32(0x40000)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			cost, speed = 20, 0x60000
		}
		if !spendChuEnergy(state, cost) {
			return
		}
		state.Body.VelocityX, state.Body.Request = speed, 0
		enterChuAttack(state, 0x41, 0xa2, 0, true)
	case 4:
		if !spendChuEnergy(state, 16) {
			return
		}
		state.Body.Facing ^= 1
		state.Body.VelocityX, state.Body.Layer, state.Body.Request = -0x40000, state.Body.Layer+8, 0
		enterChuAttack(state, 0x44, 0xa4, 0, false)
	case 5:
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
		state.Body.VelocityX = int32(uint32(state.Body.VelocityX)&0xffff | uint32(20)<<16)
		state.Body.VerticalVelocity = int32(uint32(state.Body.VerticalVelocity)&0xffff | uint32(uint16(0xffc8))<<16)
		state.Body.Request = 0
		enterChuAttack(state, 0x42, 0xa6, 0, false)
	case 6:
		if state.Body.Health > 111 || changed&ButtonA == 0 || !spendChuEnergy(state, 200) {
			return
		}
		state.Body.Gravity, state.Body.VelocityX, state.Body.VerticalVelocity = state.GravityParameter, 0, 0
		state.Body.Request = 0
		enterChuAttack(state, 0x46, 0xaa, 0, false)
	}
}

func selectChuAirSpecial(state *ChuAttackState) {
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
		cost, pose, vx, vy := int16(16), uint8(0xae), int32(0x3c000), int32(0x3c000)
		if changed&ButtonB == 0 {
			if changed&ButtonA == 0 {
				return
			}
			cost, pose, vx, vy = 20, 0xaf, 0x70000, 0x2c000
		}
		if !spendChuEnergy(state, cost) {
			return
		}
		state.Body.VelocityX, state.Body.VerticalVelocity, state.Body.Request = vx, vy, 0
		enterChuAttack(state, 0x45, pose, 0, true)
	}
}
