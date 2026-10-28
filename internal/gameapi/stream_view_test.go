package gameapi

import "testing"

// Stream cursors require a realtime module and explicit recovery. Existing
// modules without the optional callback keep the ordinary View contract.
func TestStreamViewCapabilityRequiresRealtimeRecovery(t *testing.T) {
	m := pausedClockModule()
	m.OnPausedTimer = nil
	m.StreamView = func(Match, int, string) (StreamViewResult, error) { return StreamViewResult{}, nil }
	if m.Validate() == nil {
		t.Fatal("stream registered without recovery")
	}
	m.SyncSnapshot = func(Match, int) (any, error) { return nil, nil }
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	m.Realtime = false
	if m.Validate() == nil {
		t.Fatal("non-realtime stream registered")
	}
	m.StreamView = nil
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
}
