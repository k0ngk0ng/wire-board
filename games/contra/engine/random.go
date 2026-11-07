package contra

// RandomState is the shared original $34 work byte, not a replacement PRNG.
// The original changes it in the idle loop, an enemy branch and reset paths.
// This type owns no clock and never advances automatically per game/video tick.
type RandomState struct {
	Value uint8 `json:"value"`
}

// Accumulate applies the recovered binary ADC/store arithmetic and returns its
// carry-out. loadedOperand and carryIn must come from the caller's recovered
// execution boundary, not a fixed number of additions or a new frame counter.
//
// This is an uninterrupted arithmetic fragment, not a CPU/NMI scheduler. If an
// interrupt falls between ADC and STA, the caller must retain the computed sum
// separately and commit it at the original store boundary; if it falls between
// LDA and ADC, use the previously loaded operand. It must not update World once
// per tick or consume expected after-values from a reference recording.
func (s *RandomState) Accumulate(loadedOperand uint8, carryIn bool) bool {
	sum := uint16(s.Value) + uint16(loadedOperand)
	if carryIn {
		sum++
	}
	s.Value = uint8(sum)
	return sum > 255
}
