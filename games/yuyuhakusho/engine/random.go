package engine

// NextRandom recovers 0x3982. The result's low word is the random value;
// its high word retains the effective input seed's high word, as observed in
// the original callers. Both the returned value and next state are explicit
// so AI and game rules can share one deterministic sequence.
func NextRandom(seed uint32) (next, result uint32) {
	if seed == 0 {
		seed = 0x2a6d365a
	}
	scaled := seed * 41
	value := uint16(scaled) + uint16(scaled>>16)
	return uint32(value)<<16 | scaled&0xffff, seed&0xffff0000 | uint32(value)
}
