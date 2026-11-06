package rootcontrol

import "errors"

// State is the fight profile's Root-owned state. Queue, audio-pause,
// scheduler, score, PCM and chips each retain their own unique owner.
// The audited constant-zero menu/control fields are derived, not stored.
type State struct {
	Priority     uint8        `json:"priority"`
	Selected     uint8        `json:"selected"`
	ChoiceBits   uint8        `json:"choice_bits"`
	Fade         FadeState    `json:"fade"`
	PauseRestore *PauseLevels `json:"pause_restore,omitempty"`
}

func InitialState() State { return State{Selected: 255} }

// RestoreState makes the pending restore payload independent from its caller.
// The outer coordinator validates its capture/restore stage together with the
// actual World audio-pause byte; that byte is deliberately absent here.
func RestoreState(source State) (State, error) {
	if source.ChoiceBits > 1 {
		return State{}, errors.New("unsupported fight slot-choice bits")
	}
	if source.PauseRestore != nil {
		if _, err := PauseRestoreWrites(*source.PauseRestore); err != nil {
			return State{}, err
		}
		pending := *source.PauseRestore
		source.PauseRestore = &pending
	}
	return source, nil
}
