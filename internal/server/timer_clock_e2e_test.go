package server

// Failures before clock changes: a ticker's scheduled timestamp is reused
// after waiting for the host mutex; fresh inputs see an already-expired public
// clock; the fast realtime, precise non-realtime and slow clocks behave
// differently. Authenticated HTTP rooms run the production scheduler and SQLite.
import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestModuleTimerClockAfterHostLockWaitHTTP(t *testing.T) {
	for _, mode := range []string{"realtime", "precise", "ordinary"} {
		t.Run(mode, func(t *testing.T) {
			module := realtimeFixture(mode == "realtime", 100*time.Millisecond, 0, "")
			module.PreciseTimers = mode == "precise"
			type sample struct{ clock, called time.Time }
			samples := make(chan sample, 32)
			timer := module.OnTimer
			module.OnTimer = func(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
				next, err := timer(c, m)
				// Keep the next wakeup due even when the scheduler's period and
				// the fixture's original 100ms deadline have different phases.
				next.SystemDeadline = c.Now.Add(time.Millisecond).UnixMilli()
				select {
				case samples <- sample{c.Now, time.Now()}:
				default:
				}
				return next, err
			}
			f := newRealtimeTable(t, module, true)
			// Let setup notifications drain so the blocked wakeup comes from
			// the running timer loop rather than initial bot-search scheduling.
			select {
			case <-samples:
			case <-time.After(3 * time.Second):
				t.Fatal("production clock did not start")
			}
			f.s.mu.Lock()
			for len(samples) != 0 {
				<-samples
			}
			blocked := 700 * time.Millisecond
			if mode == "ordinary" {
				blocked = 1250 * time.Millisecond
			}
			time.Sleep(blocked) // Deliberately queue a production clock wakeup.
			released := time.Now()
			f.s.mu.Unlock()
			select {
			case got := <-samples:
				lag := got.called.Sub(got.clock)
				passed := !got.clock.Before(released.Add(-10 * time.Millisecond))
				streamEvidence(t, "timer-clock-"+mode, map[string]any{"passed": passed, "scope": "authenticated production scheduler after a deliberately blocked host mutex; no gameplay/performance claim", "blockedMS": blocked.Milliseconds(), "clockLagMS": lag.Milliseconds(), "clockAtMS": got.clock.UnixMilli(), "lockReleasedMS": released.UnixMilli(), "callbackAtMS": got.called.UnixMilli()})
				if !passed {
					t.Fatal("scheduler replayed the queued ticker timestamp", "lag", lag, "clock", got.clock, "released", released)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("production clock did not resume after releasing host lock")
			}
			if p := realtimeData(t, current(f.a)); p.Frames < 1 {
				t.Fatal("fresh clock did not reach the public HTTP room")
			}
			// The callback's ordinary match remains serializable/persistable.
			f.s.mu.Lock()
			encoded, err := json.Marshal(f.s.rooms[f.id].Module)
			f.s.mu.Unlock()
			if err != nil || !json.Valid(encoded) {
				t.Fatal("clock correction broke persisted match", err)
			}
		})
	}
}
