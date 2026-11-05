// Package pcm contains the original game's direct4-bit delta and dual-DAC
// clamp arithmetic. It is not a CPU, sound driver, timing or filtered player.
package pcm

// DecoderState has one canonical8-bit accumulated value. Input bytes and the
// returned two samples are transient values, not a firmware/RAM representation.
type DecoderState struct {
	Accumulator uint8
}

func NewDecoder() DecoderState { return DecoderState{Accumulator: 128} }

// The original two256-entry lookup tables are mathematical expansions of
// these sixteen signed deltas. Addition wraps at8 bits, including nibble8.
var deltas = [16]int16{0, 1, 2, 4, 8, 16, 32, 64, -128, -1, -2, -4, -8, -16, -32, -64}

// DecodeByte emits high-nibble then low-nibble samples. The caller owns payload
// position, active lifetime, sample/bank timing and command acknowledgements.
func DecodeByte(state DecoderState, encoded uint8) (DecoderState, [2]uint8) {
	high := uint8(int16(state.Accumulator) + deltas[encoded>>4])
	low := uint8(int16(high) + deltas[encoded&15])
	return DecoderState{Accumulator: low}, [2]uint8{high, low}
}

// MixDual recovers all512 entries of the original DAC output lookup. The
// nonlinear8-bit clamp precedes YM2612/filter/resampling;128 is neutral.
func MixDual(a, b uint8) uint8 {
	sum := int(a) + int(b) - 128
	if sum < 0 {
		return 0
	}
	if sum > 255 {
		return 255
	}
	return uint8(sum)
}
