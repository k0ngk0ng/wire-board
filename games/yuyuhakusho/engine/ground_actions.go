package engine

import "errors"

// GroundActionState retains the fields touched or consumed by the recovered
// ground constructors. Auxiliary fields keep source offsets until their wider
// meaning is established. This is game state, not a CPU register or RAM image.
type GroundActionState struct {
	Clock            AnimationClock `json:"clock"`
	ActionKind       uint8          `json:"action_kind"`
	Pose             uint8          `json:"pose"`
	Facing           uint8          `json:"facing"`
	Auxiliary64      uint8          `json:"auxiliary_64"`
	Request          uint8          `json:"request"`
	EnabledStages    uint8          `json:"enabled_stages"`
	ActionFlags      uint8          `json:"action_flags"`
	Team             uint8          `json:"team"`
	BaseTeam         uint8          `json:"base_team"`
	Auxiliary77      uint8          `json:"auxiliary_77"`
	VerticalVelocity int32          `json:"vertical_velocity"`
	Commands         [16]uint8      `json:"commands"`
}

// ResetActionState recovers the tail-called reset at 0x19c1c. In particular it
// does not clear the old animation flags, motion, commands or action request.
func ResetActionState(state GroundActionState) GroundActionState {
	state.Clock.FrameIndex = 0
	state.Clock.Duration = 8
	state.Clock.Ticks = 255
	state.Clock.Completed = 0
	state.Auxiliary64 = 0
	state.ActionFlags = 0
	state.Auxiliary77 = 0
	state.Team = state.BaseTeam
	return state
}

type GroundTransition uint8

const (
	GroundStand GroundTransition = iota
	GroundRise
	GroundCrouch
	GroundGuard
	GroundTurn
	GroundCrouchTurn
	GroundWalk
)

func EnterGroundAction(state GroundActionState, transition GroundTransition, held Buttons) (GroundActionState, error) {
	pose := uint8(0)
	action := uint8(0)
	switch transition {
	case GroundStand:
		pose = 0
	case GroundRise:
		pose = 1
	case GroundCrouch:
		pose = 2
	case GroundGuard:
		pose = 9
		if held&ButtonDown != 0 {
			pose = 10
		}
	case GroundTurn, GroundCrouchTurn:
		pose = 3
		if transition == GroundCrouchTurn {
			pose = 4
		}
		state.Facing = 1 - state.Facing
		state.Commands = [16]uint8{}
		state.Request = 0
	case GroundWalk:
		pose = 5
		action = 1
	default:
		return state, errors.New("unsupported ground transition")
	}
	state.ActionKind, state.Pose, state.EnabledStages = action, pose, 0x0b
	return ResetActionState(state), nil
}

// PrepareJump is the original preparatory pose, not the jump itself. During
// this pose the velocity word's high byte stores the directional request; the
// actual jump constructor later replaces the complete velocity from parameters.
func PrepareJump(state GroundActionState, direction uint8) GroundActionState {
	state.ActionKind, state.Pose, state.EnabledStages = 0, 0x0d, 0x0b
	state.VerticalVelocity = int32(uint32(state.VerticalVelocity)&0x00ffffff | uint32(direction)<<24)
	return ResetActionState(state)
}
