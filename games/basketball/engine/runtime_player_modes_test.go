package engine

import "testing"

// New platform modes require two human actors, not just the previously verified
// one-human or all-AI configurations. Failure inventory: original AI may choose
// missing branches with a human teammate/opponent; control mappings or saves
// may change the deterministic trace; natural two-period result may not finish.
func TestRuntimeCooperativeCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, 1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeVersusCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, -1, 2, -1}
	verifyColdVideoRuntime(t, c, true)
}
