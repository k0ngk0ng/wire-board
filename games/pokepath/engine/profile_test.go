package engine

import "testing"

func TestCoreTrainerRecordRouteTagsPreserveReferenceBoundaries(t *testing.T) {
	for route, expected := range map[int]string{0: "R1-1 W1", 20: "???", 30: "???", 31: "???", 32: "R7-4 W1", 33: "???", 34: "R8-2 W1", 35: "???", 45: "R10-4 W1", 46: "R11-1 W1"} {
		state := coreNew(t, 1)
		state.Route = route
		state.World.WaveElapsedMS = 1000
		recordTrainerWave(state, true, 123)
		if state.Progression.Profile.MaxGoldPerWave.RouteTag != expected || state.Progression.Profile.MaxGoldPerTime.RouteTag != expected {
			t.Fatalf("route %d uses %s, want %s", route, state.Progression.Profile.MaxGoldPerWave.RouteTag, expected)
		}
	}
}
