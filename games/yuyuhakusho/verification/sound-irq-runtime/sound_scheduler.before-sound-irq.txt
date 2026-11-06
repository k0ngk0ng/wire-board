package engine

// SoundSchedulerState owns the delay word, delayed request, reentry backlog
// and busy byte at F560..F564. Queue, audio-pause and playback state have their
// own owners and enter these helpers only as temporary values when needed.
type SoundSchedulerState struct {
	Delay       uint16
	DelayedCode uint8
	Backlog     uint8
	Busy        uint8
}

// BeginSoundInterrupt recovers219ac through its busy return or the219de
// driver call. Delay1 becomes0 without submitting until a later call. Failed
// enqueue retains the delayed code, even when a reentry backlog is added.
func BeginSoundInterrupt(state SoundSchedulerState, queue [4]uint8) (SoundSchedulerState, [4]uint8, bool) {
	if state.Delay != 0 {
		state.Delay--
	} else if state.DelayedCode != 0 {
		for slot, queued := range queue {
			if queued < 128 || queued == state.DelayedCode {
				queue[slot] = state.DelayedCode
				state.DelayedCode = 0
				break
			}
		}
	}
	if state.Busy != 0 {
		state.Backlog++
		return state, queue, false
	}
	state.Busy = 1
	return state, queue, true
}

// FinishSoundInterrupt recovers219e4 after the caller's driver returns. A
// nested interrupt may have changed Backlog or delayed requests in between.
func FinishSoundInterrupt(state SoundSchedulerState) SoundSchedulerState {
	state.Busy = 0
	return state
}

// BeginSoundCatchup recovers219ea through the219f6 driver call. The caller
// repeats this phase after each completed tick until no backlog remains.
func BeginSoundCatchup(state SoundSchedulerState) (SoundSchedulerState, bool) {
	if state.Backlog == 0 {
		return state, false
	}
	state.Busy = 1
	return state, true
}

// FinishSoundCatchup recovers219fc..21a04. Decrement the current byte after
// the driver, including any reentry additions and the original byte wrap.
func FinishSoundCatchup(state SoundSchedulerState) SoundSchedulerState {
	state.Busy = 0
	state.Backlog--
	return state
}
