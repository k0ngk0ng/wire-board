package engine

import "errors"

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
	if state.FrameDelay != 0 {
		state.FrameDelay--
	}
	state.Frame++
	return state
}

// StepLogicClock recovers 0x3a86. Paused calls preserve both logic words.
func StepLogicClock(state WorldClockState) WorldClockState {
	if state.PauseFlags&0x80 == 0 {
		if state.LogicDelay != 0 {
			state.LogicDelay--
		}
		state.LogicFrame++
	}
	return state
}

// ToggleWorldPause recovers 0x3a9c. Allowed controller Start edges are ORed
// together, so simultaneous presses toggle once. The original do/while also
// inspects channel zero when ControllerCount is zero.
func ToggleWorldPause(state WorldClockState) (WorldClockState, error) {
	if state.PauseFlags&1 == 0 {
		state.PauseFlags &^= 0x40
		return state, nil
	}
	if state.ControllerCount > 8 {
		return state, errors.New("pause controller count is outside supported inputs")
	}
	count := state.ControllerCount
	if count == 0 {
		count = 1
	}
	var pressed uint8
	for i := uint16(0); i < count; i++ {
		if state.PauseMask&(1<<i) != 0 {
			pressed |= state.Pressed[i]
		}
	}
	if pressed&0x80 != 0 {
		state.PauseFlags ^= 0x40
	}
	return state, nil
}

// ApplyWorldPause recovers 0x3ad6. The audio flag changes only on a bit7
// transition; repeated paused or running calls retain its previous value.
func ApplyWorldPause(state WorldClockState) WorldClockState {
	if state.PauseFlags&0x40 != 0 {
		if state.PauseFlags&0x80 == 0 {
			state.AudioPause = 1
		}
		state.PauseFlags |= 0x80
	} else {
		if state.PauseFlags&0x80 != 0 {
			state.AudioPause = 0x80
		}
		state.PauseFlags &^= 0x80
	}
	return state
}

// StepWorldFeedback recovers 0x39ac, including its amplitude and independent
// signed-word pan output. Pause/slow states hold requests and decay, while
// presentation output still follows the current amplitude and logic parity.
func StepWorldFeedback(state WorldClockState) WorldClockState {
	if state.PauseFlags&0x80 == 0 && state.SlowFlag == 0 {
		if state.AmplitudeRequest != 0 {
			state.Amplitude, state.AmplitudeRequest, state.Decay = state.AmplitudeRequest, 0, 0
		}
		if state.DurationRequest != 0 {
			state.Decay = state.Amplitude / state.DurationRequest
			state.DurationRequest = 0
		} else if state.Amplitude > state.Decay {
			state.Amplitude -= state.Decay
		} else {
			state.Amplitude = 0
		}
	}
	state.ScreenShake = state.Amplitude >> 8
	if state.LogicFrame&1 != 0 {
		state.ScreenShake = 0
	}
	if state.PanAmplitude != 0 {
		state.PanOffset = (state.PanAmplitude >> 8) >> 1
		if state.LogicFrame&1 != 0 {
			state.PanOffset = 0 - state.PanOffset
		}
	}
	return state
}
