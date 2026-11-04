package engine

import "errors"

type FighterBoundsState struct {
	X, Y               int32
	Left, Right        int16
	Used               uint16 // Byte offset into each sample array.
	XSamples, YSamples [16]int32
}

// StepFighterBounds recovers the full 0x208d8 function. It clips only X's high
// word, then submits both complete fixed-point positions. Once full, the last
// sample is overwritten repeatedly. The outer caller owns its health filter.
func StepFighterBounds(state FighterBoundsState) (FighterBoundsState, error) {
	if state.Used < 63 && state.Used&3 != 0 {
		return state, errors.New("unaligned fighter position sample offset")
	}
	if state.Used >= 63 {
		state.Used = 60
	}
	x := int16(state.X >> 16)
	if x < state.Left {
		x = state.Left
	}
	if x > state.Right {
		x = state.Right
	}
	state.X = int32(uint32(uint16(x))<<16 | uint32(state.X)&0xffff)
	index := state.Used / 4
	state.XSamples[index], state.YSamples[index] = state.X, state.Y
	state.Used += 4
	return state, nil
}
