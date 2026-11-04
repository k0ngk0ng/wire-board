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
	candidate := state.CurrentPriority
	for slot := len(state.Queue) - 1; slot >= 0; slot-- {
		code := state.Queue[slot]
		state.Queue[slot] = 0
		if code == 0 {
			continue
		}
		priority := priorities[code-1]
		if priority == 255 {
			state.SelectedSound = code
			// The special branch bypasses the final priority write. Remaining
			// lower slots keep only further special requests for later calls.
			for lower := slot - 1; lower >= 0; lower-- {
				queued := state.Queue[lower]
				if queued == 0 || priorities[queued-1] != 255 {
					state.Queue[lower] = 0
				}
			}
			return state
		}
		if priority&127 >= candidate&127 {
			candidate = priority
			state.SelectedSound = code
		}
	}
	// Selection compares masked priorities; the unmasked candidate's sign
	// independently decides whether the current-priority byte is committed.
	if candidate < 128 {
		state.CurrentPriority = candidate
	}
	return state
}
