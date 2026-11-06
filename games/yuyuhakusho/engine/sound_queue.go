package engine

// SoundPriorityTable contains the 255 entries addressable by a nonzero sound
// code minus one. It is immutable input passed by value, not a ROM dependency.
type SoundPriorityTable [255]uint8

type SoundPriorityState struct {
	CurrentPriority uint8    `json:"priority"`
	SelectedSound   uint8    `json:"selected"`
	Queue           [4]uint8 `json:"queue"`
}

// ConsumeSoundQueue recovers the complete priority consumer at 0x37c44. Its
// invocation timing, selected-sound playback and synthesis are separate stages.
// Queue positions retain their original A..D ordering and are scanned D..A.
func ConsumeSoundQueue(state SoundPriorityState, priorities SoundPriorityTable) SoundPriorityState {
	// The observed implementation is the same canonical rule on local value
	// owners; the legacy pure interface retains its exact result contract.
	_ = ConsumeSoundQueueObserved(SoundQueueOwners{Priority: &state.CurrentPriority, Selected: &state.SelectedSound, Queue: &state.Queue}, priorities, nil)
	return state
}
