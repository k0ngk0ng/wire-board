package gameapi

// Failure inventory: implicit opt-in changes ordinary modules, or a turn-based
// module gains the realtime-only release exception through registration.
import (
	"encoding/json"
	"testing"
)

func TestInputReleaseCapabilityValidation(t *testing.T) {
	m := pausedClockModule()
	m.OnPausedTimer = nil
	if m.IsInputRelease != nil {
		t.Fatal("release capability must be opt-in")
	}
	m.IsInputRelease = func(Context, Match, json.RawMessage) bool { return true }
	if e := m.Validate(); e != nil {
		t.Fatal(e)
	}
	m.Realtime = false
	if e := m.Validate(); e == nil {
		t.Fatal("non-realtime input release capability registered")
	}
	m.IsInputRelease = nil
	if e := m.Validate(); e != nil {
		t.Fatal("ordinary module changed", e)
	}
}
