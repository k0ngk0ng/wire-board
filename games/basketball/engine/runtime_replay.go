package engine

import "fmt"

// RecordedVideoInput describes the host's input boundary, not game results.
// PriorHeld/PendingPressed include explicit device cancellation and very short
// taps latched between video samples. Automated records platform seat handoff.
// The Go engine still computes every action, random choice, score and picture.
type RecordedVideoInput struct {
	Frame          uint64     `json:"frame"`
	Held           [4]Buttons `json:"held"`
	PriorHeld      [4]Buttons `json:"priorHeld"`
	PendingPressed [4]Buttons `json:"pendingPressed"`
	Automated      [4]bool    `json:"automated"`
}

// StepRecordedVideo reconstructs one confirmed host video frame. This is a
// local playback protocol and is never accepted as a player HTTP action.
func StepRecordedVideo(original RuntimeState, input RecordedVideoInput) (RuntimeStep, error) {
	failure := RuntimeStep{State: original}
	if input.Frame != original.VideoFrames+1 || RuntimeFinished(original) {
		return failure, fmt.Errorf("nonconsecutive or completed video replay")
	}
	s := original
	if original.Outcome == nil {
		var err error
		s, err = SetRuntimeAutomation(s, input.Automated)
		if err != nil {
			return failure, err
		}
	}
	s.Held, s.PendingPressed = input.PriorHeld, input.PendingPressed
	step, err := StepVideo(s, input.Held)
	if err != nil {
		return failure, err
	}
	if RuntimeFinished(step.State) && step.State.Presentation == nil {
		step.State.Display, err = CommitDisplayVideo(step.State.Display, 0, 0)
		if err != nil {
			return failure, err
		}
	}
	return step, nil
}
