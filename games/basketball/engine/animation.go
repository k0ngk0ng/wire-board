package engine

import "errors"

// Action identifies a basketball action. These constants name the original
// actions already identified; listing an ID does not imply its handler is done.
type Action uint8

const (
	ActionIdle Action = iota
	ActionWalk
	ActionRun
	ActionBrake
	ActionJump
	ActionRunningJump
	ActionLand
	ActionFalling Action = 0x0f
	ActionAttackA Action = 0x60
	ActionAttackB Action = 0x64
)

type Pose struct {
	Frame    uint8 `json:"frame"`
	Mirrored bool  `json:"mirrored"`
}

// Pose identifiers refer to the independently recovered original frame atlas.
const (
	PoseStanding uint8 = iota
	PoseWalkFirst
	PoseWalkSecond
	PoseRunning
	PoseAirborne
	PoseCrouched
	PoseAttackAWindup   uint8 = 0x26
	PoseAttackAExtended uint8 = 0x27
	PoseAttackBFirst    uint8 = 0x28
	PoseAttackBSecond   uint8 = 0x29
	PoseAttackBThird    uint8 = 0x2a
)

type AnimationFrame struct {
	Pose  Pose  `json:"pose"`
	Ticks uint8 `json:"ticks"`
}

type AnimationEnd uint8

const (
	RepeatForever AnimationEnd = iota
	RepeatRemaining
	RepeatUntilCount
)

// AnimationSpec is ordinary animation data, independent of ROM locations.
// A finite sequence can request an action only after its final pose duration.
type AnimationSpec struct {
	Frames      []AnimationFrame `json:"frames"`
	End         AnimationEnd     `json:"end"`
	RepeatLimit uint8            `json:"repeatLimit"`
}

type AnimationState struct {
	Cursor        uint8  `json:"cursor"`
	Elapsed       uint8  `json:"elapsed"`
	Loops         uint8  `json:"loops"`
	NextAction    Action `json:"nextAction"`
	HasNextAction bool   `json:"hasNextAction"`
}

type AnimationStep struct {
	State        AnimationState `json:"state"`
	Pose         Pose           `json:"pose"`
	ChangeAction bool           `json:"changeAction"`
	Action       Action         `json:"action"`
}

// AdvanceAnimation preserves the original order: display the current pose,
// advance its timer, then interpret the ending. The displayed pose consequently
// remains the final old pose on the update that schedules the next action.
func AdvanceAnimation(spec AnimationSpec, state AnimationState, facingLeft bool) (AnimationStep, error) {
	if len(spec.Frames) == 0 || len(spec.Frames) > 255 || int(state.Cursor) >= len(spec.Frames) || spec.End > RepeatUntilCount {
		return AnimationStep{}, errors.New("invalid animation sequence or cursor")
	}
	for _, frame := range spec.Frames {
		if frame.Ticks == 0 || frame.Pose.Frame > 127 {
			return AnimationStep{}, errors.New("invalid animation frame")
		}
	}
	frame := spec.Frames[state.Cursor]
	result := AnimationStep{State: state, Pose: Pose{Frame: frame.Pose.Frame, Mirrored: frame.Pose.Mirrored != facingLeft}}
	result.State.Elapsed++
	if result.State.Elapsed < frame.Ticks {
		return result, nil
	}
	result.State.Elapsed = 0
	result.State.Cursor++
	if int(result.State.Cursor) < len(spec.Frames) {
		return result, nil
	}
	result.State.Cursor = 0
	switch spec.End {
	case RepeatForever:
		return result, nil
	case RepeatRemaining:
		if result.State.Loops > 0 {
			result.State.Loops--
			return result, nil
		}
	case RepeatUntilCount:
		if result.State.Loops < spec.RepeatLimit {
			result.State.Loops++
			return result, nil
		}
	}
	if result.State.HasNextAction {
		result.ChangeAction = true
		result.Action = result.State.NextAction
		result.State.HasNextAction = false
		result.State.NextAction = ActionIdle
		// Preserve the original completed-loop marker until the new action
		// initializes its animation. It is not a new repeat request.
		result.State.Loops = 255
	}
	return result, nil
}
