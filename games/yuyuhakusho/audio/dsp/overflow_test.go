package dsp

import (
	"math"
	"testing"
)

// Review failure found after the initial native suite: the committed raw-input
// cursor can be ahead of Time. Its absolute origin must be checked separately.
// This regression test precedes the cursor-overflow validation fix.
func TestCommittedRawInputCursorCannotOverflowOrigin(t *testing.T) {
	source := oracle(t).Cases[0]
	for _, test := range []struct {
		origin, time uint64
		ahead        uint32
	}{
		{math.MaxUint64, 0, 1},
		{math.MaxUint64 - FrameCycles - 10, FrameCycles, 11},
	} {
		config, state := source.Config, source.Initial
		config.MasterOrigin, state.Time, state.InputAhead = test.origin, test.time, test.ahead
		if _, err := New(config, state); err == nil {
			t.Fatal("committed raw-input cursor overflow accepted", test)
		}
	}
}
