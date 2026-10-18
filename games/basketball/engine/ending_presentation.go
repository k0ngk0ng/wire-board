package engine

// StepOriginalEndingPresentation executes the complete Scene7 phase/actor
// kernel. The caller owns the preceding text update and camera/draw boundary;
// rendering, resource waits and video cadence remain outside this component.
func StepOriginalEndingPresentation(original OpeningState, context EndingPhaseContext, pressed Buttons) (EndingPhaseStep, error) {
	initial := EndingPhaseStep{State: original, Context: context}
	r, e := StepOriginalEndingPhase(original, context, pressed)
	if e != nil {
		return initial, e
	}
	if r.Reset {
		return r, nil
	}
	for actor := uint8(0); actor < 4; actor++ {
		step, e := StepOriginalResultActor(r.State, r.Context.Actor, actor)
		if e != nil {
			return initial, e
		}
		r.State, r.Context.Actor = step.State, step.Context
		for _, sound := range step.SoundIDs {
			r.Requests = append(r.Requests, EndingPhaseRequest{Kind: EndingSound, Value: sound})
		}
	}
	return r, nil
}
