package engine

// WorldFrameState is the pre-object frame context. Controllers are retained
// between sampling steps; Clock.Pressed is their low-byte pause-input view.
// Samples passed here come from a trusted device/logic sampler, not a client
// assertion of its own edges or authoritative game state.
type WorldFrameState struct {
	Clock        WorldClockState
	Controllers  [8]ControllerFrame
	RandomSeed   uint32
	SlowGate     uint8
	ResetEnabled uint8
}

type WorldFrameOutcome struct {
	InputConsumed  bool
	ResetRequested bool
}

// BeginWorldFrame recovers the rule prefix of3a2e through3a5a. Normalization
// itself belongs to the caller. The slow gate uses the entry SlowFlag, while
// subsequent object callbacks read its possibly changed world value afresh.
func BeginWorldFrame(state WorldFrameState, samples [8]ControllerFrame) (WorldFrameState, WorldFrameOutcome, error) {
	original := state
	state.Clock = StepVideoClock(state.Clock)
	if state.SlowGate != 0 && state.Clock.SlowFlag != 0 {
		return state, WorldFrameOutcome{}, nil
	}
	state.Controllers = samples
	for i, frame := range samples {
		state.Clock.Pressed[i] = uint8(frame.Pressed)
	}
	clock, err := ToggleWorldPause(state.Clock)
	if err != nil {
		return original, WorldFrameOutcome{}, err
	}
	state.Clock = StepLogicClock(ApplyWorldPause(clock))
	outcome := WorldFrameOutcome{InputConsumed: true}
	if state.ResetEnabled != 0 {
		// The original reset chord checks four fixed base channels, independent
		// of the pause-channel mask and configured normalization count.
		for _, frame := range samples[:4] {
			if frame.Pressed&ButtonStart != 0 && frame.Current&0x70 == 0x70 {
				outcome.ResetRequested = true
				return state, outcome, nil
			}
		}
	}
	state.RandomSeed, _ = NextRandom(state.RandomSeed)
	state.Clock = StepWorldFeedback(state.Clock)
	return state, outcome, nil
}

// FinishWorldPalette recovers the actual2300→conditional233c order used by
// both the legacy VBlank wrapper and the battle RAM-IRQ/32d6 wrapper. A late
// missing interpolation table does not leak the preceding progression step.
func FinishWorldPalette(state ArenaPaletteState, library ArenaRuntimeLibrary) (ArenaPaletteState, error) {
	original := state
	state = AdvancePaletteFade(state)
	if state.Flags&1 != 0 {
		var err error
		state, err = ApplyPaletteFade(state, library)
		if err != nil {
			return original, err
		}
	}
	return state, nil
}
