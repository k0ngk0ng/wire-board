package engine

import (
	"fmt"
	"os"
	"testing"
)

// New preparation choices expose original character orderings beyond the old
// fixed roster. Failure inventory: a permitted roster selects an unrecovered
// AI/ball/prop path, its match never finishes, or recovery changes its trajectory.
// The explicit gate covers only currently offered teams/slots/courts; it does
// not stand in for all 32 characters or arbitrary human techniques. Courts5/6 are
// additionally gated here before the server/UI may expose its preparation choice.
func TestRuntimePreparedConfigurationsCompleteMatchAndRestore(t *testing.T) {
	if os.Getenv("BASKETBALL_PREPARATION_MATRIX") != "1" {
		t.Skip("set BASKETBALL_PREPARATION_MATRIX=1 for offered preparation configurations")
	}
	for court := uint8(0); court < 7; court++ {
		for roster := 0; roster < 4; roster++ {
			for _, mode := range []struct {
				name        string
				controllers [4]int8
			}{
				{"solo", [4]int8{0, -1, -1, -1}},
				{"cooperative", [4]int8{0, 1, -1, -1}},
				{"versus", [4]int8{0, -1, 2, -1}},
				{"all-computer", [4]int8{-1, -1, -1, -1}},
			} {
				config := DefaultRuntimeConfig()
				config.Match.Court = court
				config.Match.Seed = 0x12345678
				config.Match.Mode = 128
				config.Match.ControllerAssignments = mode.controllers
				left, right := uint8(roster&1), uint8((roster>>1)&1)
				config.Match.Characters = [4]uint8{left, left ^ 1, right, right ^ 1}
				t.Run(fmt.Sprintf("court%d-roster%d-%s", court, roster, mode.name), func(t *testing.T) { t.Parallel(); verifyColdVideoRuntime(t, config, true) })
			}
		}
	}
}
