package engine

// ActionPreludeState contains the stage after character-specific input
// preparation. ScreenOffsetX is the transient projection offset, not world X.
type ActionPreludeState struct {
	Body          FighterBodyState `json:"body"`
	HitStopTicks  uint8            `json:"hit_stop_ticks"`
	Shake         uint8            `json:"shake"`
	ScreenOffsetX int16            `json:"screen_offset_x"`
	StatusDelay   uint8            `json:"status_delay"`
	Residual      int32            `json:"residual"`
	Occupied      uint8            `json:"occupied"`
}

// PrepareActionStep returns false for the hit-stop path. That path keeps
// animation and motion frozen but still changes the remaining stop and shake.
func PrepareActionStep(state ActionPreludeState) (ActionPreludeState, bool) {
	if state.HitStopTicks != 0 {
		state.HitStopTicks--
		if state.Shake > 1 {
			state.Shake--
		}
		if state.HitStopTicks&1 == 0 {
			state.ScreenOffsetX += int16(state.Shake)
		} else {
			state.ScreenOffsetX -= int16(state.Shake)
		}
		return state, false
	}
	state.Shake = 0
	if state.Body.Health >= 0 {
		switch int16(state.Body.Ground >> 16) {
		case 0x140:
			state.Occupied |= 1
		case 0x130:
			state.Occupied |= 2
		}
	}
	if state.Body.TransitionDelay != 0 {
		state.Body.TransitionDelay--
	}
	if state.StatusDelay != 0 {
		state.StatusDelay--
	} else {
		state.Body.Auxiliary7E = state.Body.Auxiliary80
	}
	motion := StepResidualMotion(ResidualMotion{Position: state.Body.X, Residual: state.Residual, Facing: state.Body.Facing})
	state.Body.X, state.Residual = motion.Position, motion.Residual
	state.Body.Clock = StepAnimationClock(state.Body.Clock)
	return state, true
}
