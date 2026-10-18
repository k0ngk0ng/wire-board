package engine

// OriginalResultStep includes BA40's result decision and its four BC96 actor
// calls. A changed scene/phase describes a boundary, not a completed menu.
type OriginalResultStep struct {
	State      OpeningState
	Transition MatchTransitionStep
}

func StepOriginalResultPresentation(original OpeningState, buttons uint8) (OriginalResultStep, error) {
	core, err := StepOriginalMatchResult(original.Transition, buttons)
	if err != nil {
		return OriginalResultStep{State: original}, err
	}
	r := OriginalResultStep{State: original, Transition: core}
	r.State.Transition = core.State
	for i := uint8(0); i < 4; i++ {
		sounds, err := stepOriginalResultActor(&r.State, i)
		if err != nil {
			return OriginalResultStep{State: original}, err
		}
		r.Transition.SoundIDs = append(r.Transition.SoundIDs, sounds...)
	}
	r.Transition.State = r.State.Transition
	return r, nil
}

// Keep the original BA40 adapter. Its scripts do not issue contextual scene
// commands; reward/ending drivers retain the explicit context through the API.
func stepOriginalResultActor(s *OpeningState, actor uint8) ([]uint8, error) {
	r, err := StepOriginalResultActor(*s, ResultActorContext{}, actor)
	if err != nil {
		return nil, err
	}
	*s = r.State
	return r.SoundIDs, nil
}
