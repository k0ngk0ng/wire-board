package engine

import "github.com/k0ngk0ng/wire-board/games/basketball/campaign"

type OriginalRewardPresentationStep struct {
	OriginalRewardPhaseStep
	Context ResultActorContext
}

// StepOriginalRewardPresentation preserves BD8E's actor tail on every call,
// including the call that changes Scene to2. Scene1's outer RNG and renderer
// finalization belong to its caller, not to this phase handler.
func StepOriginalRewardPresentation(original OpeningState, progress campaign.Progress, context ResultActorContext, pressed uint8) (OriginalRewardPresentationStep, error) {
	initial := OriginalRewardPresentationStep{OriginalRewardPhaseStep: OriginalRewardPhaseStep{State: original, Progress: progress}, Context: context}
	phase, err := StepOriginalRewardPhase(original, progress, pressed)
	if err != nil {
		return initial, err
	}
	r := OriginalRewardPresentationStep{OriginalRewardPhaseStep: phase, Context: context}
	for actor := uint8(0); actor < 3; actor++ {
		step, err := StepOriginalResultActor(r.State, r.Context, actor)
		if err != nil {
			return initial, err
		}
		r.State, r.Context = step.State, step.Context
		r.SoundIDs = append(r.SoundIDs, step.SoundIDs...)
	}
	return r, nil
}
