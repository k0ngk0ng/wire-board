package engine

// NextRandom recovers 0x3982. The result's low word is the random value;
// its high word retains the effective input seed's high word, as observed in
// the original callers. Both the returned value and next state are explicit
// so AI and game rules can share one deterministic sequence.
func NextRandom(seed uint32) (next, result uint32) {
	next, result, _ = NextRandomWork(seed, nil)
	return next, result
}
