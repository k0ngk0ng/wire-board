package engine

// RandomState stores the original game's four-byte feedback register. Keeping
// it explicit allows native/Wasm replay and eventual saved-world restoration;
// game decisions must consume this stream in the original order.
type RandomState uint32

func NextRandom(state RandomState) (RandomState, byte) {
	feedback := ((state >> 25) ^ (state >> 17)) & 1
	next := (state >> 1) | (feedback << 31)
	return next, byte(next >> 24)
}
