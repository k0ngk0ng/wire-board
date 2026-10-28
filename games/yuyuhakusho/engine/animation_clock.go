package engine

// AnimationClock preserves the original byte-sized animation timing state.
// Flags belong to the current animation record. Completed is retained until
// the action state machine explicitly consumes or resets it.
type AnimationClock struct {
	FrameIndex uint8 `json:"frame_index"`
	Duration   uint8 `json:"duration"`
	Ticks      uint8 `json:"ticks"`
	Flags      uint8 `json:"flags"`
	Completed  uint8 `json:"completed"`
}

// StepAnimationClock advances the current record before the animation-data
// stage selects the record to display. It does not choose actions or sprites.
func StepAnimationClock(state AnimationClock) AnimationClock {
	state.Ticks++
	if state.Duration <= state.Ticks {
		state.Ticks = 0
		state.FrameIndex++
		if state.Flags&0x80 != 0 {
			state.FrameIndex = 0
			state.Completed = 1
		}
	}
	return state
}

// ResidualMotion is the decaying displacement applied before the active
// action's own motion. Position and Residual use signed 16.16 fixed point.
type ResidualMotion struct {
	Position int32 `json:"position"`
	Residual int32 `json:"residual"`
	Facing   uint8 `json:"facing"`
}

// StepResidualMotion keeps the original final negative remainder: the step
// which crosses zero still moves, while subsequent non-positive steps stop.
func StepResidualMotion(state ResidualMotion) ResidualMotion {
	if state.Residual > 0 {
		state.Residual -= 0x4800
		if state.Facing == 1 {
			state.Position += state.Residual
		} else {
			state.Position -= state.Residual
		}
	}
	return state
}
