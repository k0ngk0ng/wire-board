package engine

import "errors"

type ChuInputState struct {
	Envelope FighterInputEnvelopeState
	AI       ChuAIState
}

// StepChuInput recovers the complete 0x15540 input call for Chu, including
// its real AI and the post-AI recording continuation. Envelope.Controls is the
// single shared control state; AI-only counters and world RNG advance only
// when the original route calls that strategy. Both states roll back on error.
func StepChuInput(state ChuInputState) (ChuInputState, error) {
	return stepFighterInput(state, StepChuAI)
}

// StepFighterInput uses the same complete envelope and recording continuation
// with the recovered character-specific AI. Human input never calls a strategy.
func StepFighterInput(state ChuInputState) (ChuInputState, error) {
	return stepFighterInput(state, StepFighterAI)
}

func stepFighterInput(state ChuInputState, strategy func(ChuAIState) (ChuAIState, error)) (ChuInputState, error) {
	original := state
	envelope, route, err := PrepareFighterInputEnvelope(state.Envelope)
	if err != nil {
		return original, err
	}
	state.Envelope = envelope
	state.AI.Actor.Controls = envelope.Controls
	switch route {
	case FighterInputComplete:
		return state, nil
	case FighterInputNeedsAI, FighterInputNeedsAIThenRecord:
		ai, err := strategy(state.AI)
		if err != nil {
			return original, err
		}
		state.AI = ai
		state.Envelope.Controls = ai.Actor.Controls
		if route == FighterInputNeedsAIThenRecord {
			state.Envelope, err = RecordFighterInputEnvelope(state.Envelope)
			if err != nil {
				return original, err
			}
		}
		state.AI.Actor.Controls = state.Envelope.Controls
		return state, nil
	default:
		return original, errors.New("unsupported complete fighter input route")
	}
}
