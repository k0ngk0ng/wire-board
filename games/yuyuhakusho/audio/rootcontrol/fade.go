// Package rootcontrol owns the independent Root control fields. FadeState is
// the two-counter component; other Root owners are composed by the caller.
package rootcontrol

import "errors"

// FadeState is embedded exactly once in the final Root state. It owns only
// these independent byte counters, never score, PCM or another owner view.
type FadeState struct {
	Steps     uint8 `json:"steps"`
	Remaining uint8 `json:"remaining"`
}

type FadeCounter uint8

const (
	FadeRemainingCounter FadeCounter = iota + 1
	FadeStepsCounter
)

// FadeWrite is a temporary semantic effect, in its actual counter order.
type FadeWrite struct {
	Counter FadeCounter
	Value   uint8
}

type FadeAction uint8

const (
	FadeNoAction FadeAction = iota
	StopDACFlags
	FadeMusicStep
	RequestFullClear
)

// FadeResult is consumed within the current detached transaction. Apply its
// bounded Writes in order, then dispatch Action to the corresponding owner.
// It is not persistent history or another copy of the Root/score state.
type FadeResult struct {
	State      FadeState
	Writes     [2]FadeWrite
	WriteCount uint8
	Action     FadeAction
}

var ErrFadeUnsupported = errors.New("unsupported fade-start command")

// StartFade handles only the original 01/05 fade-start decisions. It writes
// remaining before steps, then asks the score owner to clear DAC music flags.
// DAC parameters and the current independent PCM playback stay caller-owned.
func StartFade(current FadeState, command uint8) (FadeResult, error) {
	var steps, remaining uint8
	switch command {
	case 1:
		steps, remaining = 40, 3
	case 5:
		steps, remaining = 50, 32
	default:
		return FadeResult{}, ErrFadeUnsupported
	}
	return FadeResult{
		State:      FadeState{Steps: steps, Remaining: remaining},
		Writes:     [2]FadeWrite{{Counter: FadeRemainingCounter, Value: remaining}, {Counter: FadeStepsCounter, Value: steps}},
		WriteCount: 2,
		Action:     StopDACFlags,
	}, nil
}

// AdvanceFade advances only the counter phase of one actual fade invocation.
// The caller suppresses this call when pause/gate skips fade. The terminal
// action requests the separate ordered 04 transaction; it does not clear
// priority, selected, choice, queue, pause, score, PCM or chip state here.
func AdvanceFade(current FadeState) FadeResult {
	result := FadeResult{State: current}
	if current.Steps == 0 {
		return result
	}
	if current.Remaining != 0 {
		result.State.Remaining--
		result.Writes[0] = FadeWrite{Counter: FadeRemainingCounter, Value: result.State.Remaining}
		result.WriteCount = 1
		return result
	}
	result.State.Steps--
	result.Writes[0] = FadeWrite{Counter: FadeStepsCounter, Value: result.State.Steps}
	result.WriteCount = 1
	if result.State.Steps == 0 {
		result.Action = RequestFullClear
		return result
	}
	result.State.Remaining = 3
	result.Writes[1] = FadeWrite{Counter: FadeRemainingCounter, Value: 3}
	result.WriteCount = 2
	result.Action = FadeMusicStep
	return result
}
