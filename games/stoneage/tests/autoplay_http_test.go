package stoneage_test

// Actual platform scheduling, unlike the manually stepped strategy fixtures.
// Failure inventory predates implementation: idle bots, frozen preparation,
// missing modes, broken reclaim, restarts and repeated finalization (case 12).
import (
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func TestStoneageNativeAutoPlayAllModesHTTP(t *testing.T) {
	for _, n := range []int{2, 4, 6, 8, 10} {
		t.Run(fmt.Sprintf("%dv%d", n/2, n/2), func(t *testing.T) {
			t.Parallel()
			h := gametest.NewHTTP(t, 2, acceleratedHTTPModule(), "stoneage")
			a, b := h.People[0], h.People[1]
			created := a.Request("POST", "/api/rooms", map[string]any{"kind": "stoneage", "name": "石器原生托管验收", "capacity": n}, 201)
			id := created["id"].(string)
			b.Request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": created["version"], "nonce": "stoneage-native-join"}, 200)
			for i := 2; i < n; i++ {
				a.Command("add_bot", nil, 200)
			}
			a.Command("ready", nil, 200)
			b.Command("ready", nil, 200)
			a.Command("start", nil, 200)
			h.Spectator.Request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
			nativeHTTPCommand(t, a, "autoplay", map[string]any{"enabled": true})
			nativeHTTPCommand(t, b, "autoplay", map[string]any{"enabled": true})
			phases := map[string]bool{}
			reclaimed := false
			restarted := false
			until := time.Now().Add(7 * time.Minute)
			for time.Now().Before(until) {
				r := a.Room()
				v := httpView(t, a)
				phases[v.Phase] = true
				if r["status"] == "finished" {
					break
				}
				if n == 4 && v.Phase == "planning" && !reclaimed {
					nativeHTTPCommand(t, b, "autoplay", map[string]any{"enabled": false})
					nativeManualDecision(t, b)
					nativeHTTPCommand(t, b, "autoplay", map[string]any{"enabled": true})
					reclaimed = true
					h.Record("manual handoff and resubmission through real session")
				}
				if n == 6 && v.Phase == "resolving" && !restarted {
					h.Restart()
					restarted = true
				}
				time.Sleep(100 * time.Millisecond)
			}
			r := a.Room()
			if r["status"] != "finished" {
				t.Fatal("native bot scheduler stalled", httpView(t, a).Phase)
			}
			if !phases["selection"] || !phases["arming"] || !phases["planning"] || !phases["resolving"] {
				t.Fatal("missing native phase", phases)
			}
			result := r["result"].(map[string]any)
			if n > 2 && result["rated"] != false {
				t.Fatal("computer match rated")
			}
			h.Record("native computer/autoplay complete match")
			report(t, fmt.Sprintf("native-autoplay-%dv%d", n/2, n/2), map[string]any{"clockScale": 4, "phases": phases, "reclaimed": reclaimed, "restarted": restarted, "room": r})
		})
	}
}
