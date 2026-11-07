package engine

import (
	"errors"
	"math/bits"
)

type MainWorkHook func(MainWorkOperation) error
type WorldClockTask uint8

const (
	WorldVideoClock WorldClockTask = iota + 1
	WorldLogicClock
	WorldTogglePause
	WorldApplyPause
	WorldFeedback
)

type WorldClockWorkHook func(MainWorkOperation, *WorldClockState) error

type frameWork struct {
	clock *WorldClockState
	hook  WorldClockWorkHook
	err   error
}

func (w *frameWork) step(phase string, cost uint16, apply func()) {
	if w.err != nil {
		return
	}
	if w.hook != nil {
		w.err = w.hook(MainWorkOperation{Phase: phase, CPUCycles: cost}, w.clock)
	}
	if w.err == nil && apply != nil {
		apply()
	}
}
func (w *frameWork) spend(phase string, costs ...uint16) {
	for _, cost := range costs {
		w.step(phase, cost, nil)
	}
}
func (w *frameWork) branch(phase string, taken bool) {
	cost := uint16(8)
	if taken {
		cost = 10
	}
	w.spend(phase, cost)
}
func (w *frameWork) video() {
	c := w.clock
	var delayed bool
	w.step("video-delay-read", 12, func() { delayed = c.FrameDelay != 0 })
	w.branch("video-no-delay", !delayed)
	if delayed {
		w.step("video-delay-decrement", 16, func() { c.FrameDelay-- })
	}
	w.step("video-frame-increment", 16, func() { c.Frame++ })
	w.spend("video-clock-return", 16)
}
func (w *frameWork) logic() {
	c := w.clock
	var paused, delayed bool
	w.step("logic-pause-read", 12, func() { paused = c.PauseFlags&128 != 0 })
	w.branch("logic-pause-hold", paused)
	if !paused {
		w.step("logic-delay-read", 12, func() { delayed = c.LogicDelay != 0 })
		w.branch("logic-no-delay", !delayed)
		if delayed {
			w.step("logic-delay-decrement", 16, func() { c.LogicDelay-- })
		}
		w.step("logic-frame-increment", 16, func() { c.LogicFrame++ })
	}
	w.spend("logic-clock-return", 16)
}
func (w *frameWork) toggle() {
	c := w.clock
	var enabled bool
	var mask, pressed uint8
	w.step("pause-enabled-read", 12, func() { enabled = c.PauseFlags&1 != 0 })
	w.spend("pause-enabled-test", 10)
	w.branch("pause-enabled", enabled)
	if !enabled {
		w.step("pause-disable-toggle", 20, func() { c.PauseFlags &^= 64 })
		w.spend("pause-toggle-return", 16)
		return
	}
	if c.ControllerCount > 8 {
		w.err = errors.New("pause controller count is outside supported inputs")
		return
	}
	w.step("pause-mask-read", 12, func() { mask = c.PauseMask })
	w.spend("pause-input-setup", 4, 8, 4)
	for index := uint16(0); index < 8; index++ {
		w.spend("pause-channel-test", 6)
		selected := mask&(1<<index) != 0
		w.branch("pause-channel-disabled", !selected)
		if selected {
			w.step("pause-start-accumulate", 14, func() { pressed |= c.Pressed[index] })
		}
		w.spend("pause-channel-next", 4)
		var more bool
		w.step("pause-channel-count", 12, func() { more = index+1 < c.ControllerCount })
		w.branch("pause-channel-loop", more)
		if w.err != nil || !more {
			break
		}
		if index == 7 {
			w.err = errors.New("pause controller count is outside supported inputs")
		}
	}
	w.spend("pause-start-test", 4)
	w.branch("pause-no-start", pressed&128 == 0)
	if pressed&128 != 0 {
		w.step("pause-toggle", 20, func() { c.PauseFlags ^= 64 })
	}
	w.spend("pause-toggle-return", 16)
}
func (w *frameWork) pause() {
	c := w.clock
	var requested, previous bool
	w.step("pause-request-read", 16, func() { requested = c.PauseFlags&64 != 0 })
	w.branch("pause-running-branch", !requested)
	if requested {
		w.step("pause-state-set", 20, func() { previous = c.PauseFlags&128 != 0; c.PauseFlags |= 128 })
		w.branch("pause-already-paused", previous)
		if !previous {
			w.step("pause-audio-request", 16, func() { c.AudioPause = 1 })
		}
	} else {
		w.step("pause-state-clear", 20, func() { previous = c.PauseFlags&128 != 0; c.PauseFlags &^= 128 })
		w.branch("pause-already-running", !previous)
		if previous {
			w.step("resume-audio-request", 16, func() { c.AudioPause = 128 })
		}
	}
	w.spend("pause-apply-return", 16)
}
func (w *frameWork) feedbackAmplitude() {
	c := w.clock
	var paused, slow bool
	w.step("feedback-pause-read", 12, func() { paused = c.PauseFlags&128 != 0 })
	w.branch("feedback-pause-hold", paused)
	if !paused {
		w.step("feedback-slow-read", 12, func() { slow = c.SlowFlag != 0 })
		w.branch("feedback-slow-hold", slow)
	}
	if !paused && !slow {
		var request uint16
		w.step("feedback-amplitude-request-read", 12, func() { request = c.AmplitudeRequest })
		w.branch("feedback-no-amplitude-request", request == 0)
		if request != 0 {
			w.step("feedback-amplitude-request-clear", 16, func() { c.AmplitudeRequest = 0 })
			w.step("feedback-amplitude-set", 12, func() { c.Amplitude = request })
			w.step("feedback-decay-clear", 16, func() { c.Decay = 0 })
		}
		var duration uint16
		w.step("feedback-duration-read", 12, func() { duration = c.DurationRequest })
		w.branch("feedback-no-duration-request", duration == 0)
		if duration != 0 {
			w.step("feedback-duration-clear", 16, func() { c.DurationRequest = 0 })
			w.spend("feedback-dividend-clear", 4)
			var amplitude uint16
			w.step("feedback-dividend-read", 12, func() { amplitude = c.Amplitude })
			if w.err == nil && w.hook != nil {
				cost := uint16(136 - 2*bits.OnesCount16(uint16(uint32(amplitude)/(2*uint32(duration)))))
				w.err = w.hook(MainWorkOperation{Phase: "feedback-divide", CPUCycles: cost, InternalRefresh: true}, c)
			}
			w.step("feedback-decay-set", 12, func() { c.Decay = amplitude / duration })
			w.spend("feedback-decay-ready", 10)
		} else {
			var amplitude uint16
			w.step("feedback-decay-amplitude-read", 12, func() { amplitude = c.Amplitude })
			w.branch("feedback-decay-has-amplitude", amplitude != 0)
			if amplitude == 0 {
				w.step("feedback-empty-shake-clear", 12, func() { c.ScreenShake = 0 })
				w.spend("feedback-amplitude-return", 16)
				return
			}
			var decayed uint16
			var positive bool
			w.step("feedback-decay-subtract", 12, func() { positive = amplitude > c.Decay; decayed = amplitude - c.Decay })
			w.branch("feedback-decay-positive", positive)
			if !positive {
				w.step("feedback-decay-zero", 4, func() { decayed = 0 })
			}
			w.step("feedback-decay-store", 12, func() { c.Amplitude = decayed })
		}
	}
	var amplitude uint16
	var odd bool
	w.step("feedback-shake-read", 12, func() { amplitude = c.Amplitude })
	w.step("feedback-shake-shift", 22, func() { amplitude >>= 8 })
	w.step("feedback-shake-parity", 16, func() { odd = c.LogicFrame&1 != 0 })
	w.branch("feedback-shake-even", !odd)
	if odd {
		w.step("feedback-shake-zero", 4, func() { amplitude = 0 })
	}
	w.step("feedback-shake-store", 12, func() { c.ScreenShake = amplitude })
	w.spend("feedback-amplitude-return", 16)
}
func (w *frameWork) feedback() {
	w.spend("feedback-amplitude-call", 18)
	w.feedbackAmplitude()
	w.spend("feedback-pan-jump", 10)
	c := w.clock
	var pan uint16
	var odd bool
	w.step("feedback-pan-read", 12, func() { pan = c.PanAmplitude })
	w.branch("feedback-pan-absent", pan == 0)
	if pan != 0 {
		w.step("feedback-pan-shift", 22, func() { pan >>= 8 })
		w.step("feedback-pan-half", 8, func() { pan >>= 1 })
		w.step("feedback-pan-parity", 16, func() { odd = c.LogicFrame&1 != 0 })
		w.branch("feedback-pan-even", !odd)
		if odd {
			w.step("feedback-pan-negate", 4, func() { pan = 0 - pan })
		}
		w.step("feedback-pan-store", 12, func() { c.PanOffset = pan })
	}
	w.spend("feedback-pan-return", 16)
}
func clockWorkLive(clock *WorldClockState, task WorldClockTask, hook WorldClockWorkHook) error {
	w := frameWork{clock: clock, hook: hook}
	switch task {
	case WorldVideoClock:
		w.video()
	case WorldLogicClock:
		w.logic()
	case WorldTogglePause:
		w.toggle()
	case WorldApplyPause:
		w.pause()
	case WorldFeedback:
		w.feedback()
	default:
		return errors.New("unknown world clock task")
	}
	return w.err
}
func AdvanceWorldClockWork(state WorldClockState, task WorldClockTask, hook WorldClockWorkHook) (WorldClockState, error) {
	original := state
	if err := clockWorkLive(&state, task, hook); err != nil {
		return original, err
	}
	return state, nil
}

func randomWorkLive(seed *uint32, hook MainWorkHook) (uint32, error) {
	w := frameWork{hook: func(op MainWorkOperation, _ *WorldClockState) error {
		if hook != nil {
			return hook(op)
		}
		return nil
	}}
	w.spend("random-temporary-save", 12)
	var value uint32
	w.step("random-seed-read", 16, func() { value = *seed })
	w.branch("random-seed-nonzero", value != 0)
	if value == 0 {
		w.step("random-default-seed", 12, func() { value = 0x2a6d365a })
	}
	var original uint32
	w.step("random-seed-copy", 4, func() { original = value })
	w.step("random-scale-four", 12, func() { value <<= 2 })
	w.step("random-add-one", 8, func() { value += original })
	w.step("random-scale-eight", 14, func() { value <<= 3 })
	w.step("random-add-one", 8, func() { value += original })
	var folded uint16
	w.step("random-low-word", 4, func() { folded = uint16(value) })
	w.spend("random-high-word", 4)
	w.step("random-fold", 4, func() { folded += uint16(value >> 16) })
	w.spend("random-next-low", 4, 4)
	w.step("random-seed-store", 16, func() { *seed = uint32(folded)<<16 | value&65535 })
	w.spend("random-temporary-restore", 12)
	w.spend("random-return", 16)
	return original&0xffff0000 | uint32(folded), w.err
}
func NextRandomWork(seed uint32, hook MainWorkHook) (uint32, uint32, error) {
	original := seed
	result, err := randomWorkLive(&seed, hook)
	if err != nil {
		return original, 0, err
	}
	return seed, result, nil
}

type WorldFrameOwners struct {
	Clock                  *WorldClockState
	Controllers            *[8]ControllerFrame
	RandomSeed             *uint32
	SlowGate, ResetEnabled *uint8
}
type WorldFrameWorkHooks struct {
	Work      func(MainWorkOperation, WorldFrameOwners) error
	Normalize func(WorldFrameOwners, MainWorkHook) error
}

func beginWorldFrameLive(owners WorldFrameOwners, hooks WorldFrameWorkHooks) (WorldFrameOutcome, error) {
	work := func(op MainWorkOperation) error {
		if hooks.Work != nil {
			return hooks.Work(op, owners)
		}
		return nil
	}
	w := frameWork{clock: owners.Clock, hook: func(op MainWorkOperation, _ *WorldClockState) error { return work(op) }}
	w.spend("world-video-clock-call", 18)
	if w.err == nil {
		w.err = clockWorkLive(owners.Clock, WorldVideoClock, w.hook)
	}
	var gate, slow bool
	w.step("world-slow-gate-read", 12, func() { gate = *owners.SlowGate != 0 })
	w.branch("world-slow-gate-disabled", !gate)
	if gate {
		w.step("world-slow-read", 12, func() { slow = owners.Clock.SlowFlag != 0 })
		w.branch("world-slow-hold", slow)
	}
	if w.err != nil {
		return WorldFrameOutcome{}, w.err
	}
	if gate && slow {
		return WorldFrameOutcome{}, nil
	}
	w.spend("world-normalize-call", 20)
	if w.err != nil {
		return WorldFrameOutcome{}, w.err
	}
	if hooks.Normalize == nil {
		return WorldFrameOutcome{}, errors.New("world input normalization work is missing")
	}
	if err := hooks.Normalize(owners, work); err != nil {
		return WorldFrameOutcome{}, err
	}
	// Clock.Pressed is the existing public clock API's low-byte projection.
	// Its canonical frame data is the normalized Controllers array.
	for i, c := range owners.Controllers {
		owners.Clock.Pressed[i] = uint8(c.Pressed)
	}
	for _, call := range []struct {
		phase string
		cost  uint16
		task  WorldClockTask
	}{{"world-pause-toggle-call", 18, WorldTogglePause}, {"world-pause-apply-call", 18, WorldApplyPause}, {"world-logic-clock-call", 18, WorldLogicClock}} {
		w.spend(call.phase, call.cost)
		if w.err == nil {
			w.err = clockWorkLive(owners.Clock, call.task, w.hook)
		}
	}
	w.spend("world-reset-check-call", 18)
	var resetEnabled bool
	w.step("world-reset-enabled", 12, func() { resetEnabled = *owners.ResetEnabled != 0 })
	w.branch("world-reset-disabled", !resetEnabled)
	if resetEnabled {
		for index := 0; index < 4; index++ {
			var pressed bool
			var held Buttons
			w.step("world-reset-start-test", 16, func() { pressed = (*owners.Controllers)[index].Pressed&ButtonStart != 0 })
			if index < 3 {
				w.branch("world-reset-no-start", !pressed)
			} else {
				w.branch("world-reset-no-final-start", !pressed)
			}
			if pressed {
				w.step("world-reset-held-read", 12, func() { held = (*owners.Controllers)[index].Current })
				w.spend("world-reset-held-mask", 8, 8)
				chord := held&0x70 == 0x70
				w.branch("world-reset-chord", func() bool {
					if index == 3 {
						return !chord
					}
					return chord
				}())
				if chord {
					w.spend("world-reset-stack", 8, 12)
					if w.err != nil {
						return WorldFrameOutcome{}, w.err
					}
					return WorldFrameOutcome{InputConsumed: true, ResetRequested: true}, nil
				}
			}
		}
	}
	w.spend("world-reset-return", 16)
	w.spend("world-random-call", 20)
	if w.err == nil {
		_, w.err = randomWorkLive(owners.RandomSeed, work)
	}
	w.spend("world-feedback-call", 20)
	if w.err == nil {
		w.err = clockWorkLive(owners.Clock, WorldFeedback, w.hook)
	}
	if w.err != nil {
		return WorldFrameOutcome{}, w.err
	}
	return WorldFrameOutcome{InputConsumed: true}, nil
}
func BeginWorldFrameWork(state WorldFrameState, hooks WorldFrameWorkHooks) (WorldFrameState, WorldFrameOutcome, error) {
	original := state
	owners := WorldFrameOwners{Clock: &state.Clock, Controllers: &state.Controllers, RandomSeed: &state.RandomSeed, SlowGate: &state.SlowGate, ResetEnabled: &state.ResetEnabled}
	out, err := beginWorldFrameLive(owners, hooks)
	if err != nil {
		return original, WorldFrameOutcome{}, err
	}
	return state, out, nil
}

// AdvanceWorldFrameOnOwners borrows a detached caller's actual frame fields.
// It permits the main coordinator to change those same fields inside an IRQ;
// errors may follow partial work, so the enclosing World/audio transaction
// commits only after the whole requested boundary succeeds.
func AdvanceWorldFrameOnOwners(owners WorldFrameOwners, hooks WorldFrameWorkHooks) (WorldFrameOutcome, error) {
	if owners.Clock == nil || owners.Controllers == nil || owners.RandomSeed == nil || owners.SlowGate == nil || owners.ResetEnabled == nil {
		return WorldFrameOutcome{}, errors.New("missing live world frame owner")
	}
	return beginWorldFrameLive(owners, hooks)
}
