package engine

// WorldClockState separates unconditional frame counting from the unpaused
// logic counter. Slow-motion dispatch decides when callers invoke each helper;
// these counters do not stand in for the complete world/object scheduler.
type WorldClockState struct {
	Frame            uint16   `json:"frame"`
	FrameDelay       uint16   `json:"frame_delay"`
	LogicFrame       uint16   `json:"logic_frame"`
	LogicDelay       uint16   `json:"logic_delay"`
	PauseFlags       uint8    `json:"pause_flags"`
	PauseMask        uint8    `json:"pause_mask"`
	ControllerCount  uint16   `json:"controller_count"`
	Pressed          [8]uint8 `json:"pressed"`
	AudioPause       uint8    `json:"audio_pause"`
	SlowFlag         uint8    `json:"slow_flag"`
	ScreenShake      uint16   `json:"screen_shake"`
	DurationRequest  uint16   `json:"duration_request"`
	AmplitudeRequest uint16   `json:"amplitude_request"`
	Amplitude        uint16   `json:"amplitude"`
	Decay            uint16   `json:"decay"`
	PanAmplitude     uint16   `json:"pan_amplitude"`
	PanOffset        uint16   `json:"pan_offset"`
}

// StepVideoClock recovers 0x3a76, including unsigned word wrap.
func StepVideoClock(state WorldClockState) WorldClockState {
	next, _ := AdvanceWorldClockWork(state, WorldVideoClock, nil)
	return next
}

// StepLogicClock recovers 0x3a86. Paused calls preserve both logic words.
func StepLogicClock(state WorldClockState) WorldClockState {
	next, _ := AdvanceWorldClockWork(state, WorldLogicClock, nil)
	return next
}

// ToggleWorldPause recovers 0x3a9c. Allowed controller Start edges are ORed
// together, so simultaneous presses toggle once. The original do/while also
// inspects channel zero when ControllerCount is zero.
func ToggleWorldPause(state WorldClockState) (WorldClockState, error) {
	return AdvanceWorldClockWork(state, WorldTogglePause, nil)
}

// ApplyWorldPause recovers 0x3ad6. The audio flag changes only on a bit7
// transition; repeated paused or running calls retain its previous value.
func ApplyWorldPause(state WorldClockState) WorldClockState {
	next, _ := AdvanceWorldClockWork(state, WorldApplyPause, nil)
	return next
}

// StepWorldFeedback recovers 0x39ac, including its amplitude and independent
// signed-word pan output. Pause/slow states hold requests and decay, while
// presentation output still follows the current amplitude and logic parity.
func StepWorldFeedback(state WorldClockState) WorldClockState {
	next, _ := AdvanceWorldClockWork(state, WorldFeedback, nil)
	return next
}
