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
	return BeginWorldFrameWork(state, WorldFrameWorkHooks{Normalize: func(owners WorldFrameOwners, _ MainWorkHook) error { *owners.Controllers = samples; return nil }})
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
