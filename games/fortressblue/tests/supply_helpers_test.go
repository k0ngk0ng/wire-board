package tests

import (
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"testing"
)

// Keep existing physics/status scenarios at an actual action opportunity. This
// uses real system timers and never edits a stored stage, deadline or inventory.
func completeSupplyTransport(t *testing.T, mod gameapi.Module, m gameapi.Match) gameapi.Match {
	t.Helper()
	for step := 0; step < 20; step++ {
		s, _ := timeline(t, m)
		flying := false
		for _, drop := range s.Supplies {
			if !drop.Landed {
				flying = true
			}
		}
		if !flying || m.Result != nil {
			return m
		}
		if m.SystemDeadline <= 0 {
			t.Fatal("transport has no system deadline")
		}
		m = timerAt(t, mod, m, m.SystemDeadline)
	}
	t.Fatal("transport failed to settle")
	return m
}
