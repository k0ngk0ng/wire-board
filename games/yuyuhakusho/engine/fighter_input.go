package engine

// FighterControls is the human-control stage consumed by the fighter rules.
// Changed is XOR, not Released. All three channels retain their extension byte.
type FighterControls struct {
	Held        Buttons `json:"held"`
	Pressed     Buttons `json:"pressed"`
	Changed     Buttons `json:"changed"`
	RawHeld     uint8   `json:"raw_held"`
	RetainTicks uint8   `json:"retain_ticks"`
}

// LatchFighterInput recovers the human branch of 0x155e8. AI and recorded
// playback are separate caller-selected paths; this routine does not replace
// either with artificial player actions.
func LatchFighterInput(state FighterControls, input ControllerFrame) FighterControls {
	state.RawHeld = uint8(input.Current)
	if state.RetainTicks == 0 {
		state.Held, state.Pressed, state.Changed = input.Current, input.Pressed, input.Changed
		return state
	}
	state.RetainTicks--
	retain := func(previous, incoming Buttons) Buttons {
		if incoming&0x70 != 0 {
			return incoming
		}
		// Direction and Start refresh immediately. Until a base action bit is
		// present in this particular channel, its old action and extension
		// state remains. Do not merge the three channels into one condition.
		return previous&0xff70 | incoming&0x8f
	}
	state.Held = retain(state.Held, input.Current)
	state.Pressed = retain(state.Pressed, input.Pressed)
	state.Changed = retain(state.Changed, input.Changed)
	return state
}
