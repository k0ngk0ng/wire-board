package stoneage_test

import (
	"testing"
	"time"

	stoneage "github.com/k0ngk0ng/wire-board/games/stoneage/server"

	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// Real HTTP, SQLite and the production scheduler: no accelerated module clock.
func TestStoneageNativeRoundHandoffHTTP(t *testing.T) {
	h := gametest.NewHTTP(t, 2, stoneage.Module(), "stoneage")
	h.Start(map[string]any{})
	httpSelection(t, h)
	measurements := []map[string]any{}
	for round := 1; round <= 4; round++ {
		started := time.Now()
		for i, c := range h.People {
			v := httpView(t, c)
			if v.Phase != "planning" || v.Round != round {
				t.Fatal("missing planning window", v.Phase, v.Round)
			}
			started = time.Now()
			c.Action(map[string]any{"type": "submit", "session": v.Session, "window": v.Window,
				"plan": guards(i)}, 200)
		}
		v := httpView(t, h.People[0])
		if v.Playback == nil {
			t.Fatal("guard round did not create a timeline")
		}
		duration := v.Playback.Duration
		until := time.Now().Add(3 * time.Second)
		for v.Phase == "resolving" && time.Now().Before(until) {
			time.Sleep(10 * time.Millisecond)
			v = httpView(t, h.People[0])
		}
		lag := time.Since(started).Milliseconds() - int64(duration)
		measurements = append(measurements, map[string]any{"round": round, "timelineMs": duration, "handoffLagMs": lag})
		if v.Phase != "planning" || v.Round != round+1 || v.Players[0].Submitted || v.Players[1].Submitted {
			t.Fatal("round handoff did not unlock both players", v.Phase, v.Round)
		}
		if lag > 250 {
			report(t, "native-round-handoff", measurements)
			t.Fatalf("round %d waited %dms after the timeline; expected <=250ms", round, lag)
		}
	}
	h.Record("four native-speed rounds; precise handoff without waiting for whole-second ticks")
	report(t, "native-round-handoff", measurements)
}
