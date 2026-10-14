package snooker_test

// Real host scheduling, computer seats, manual takeover, persistence, and full
// matches. No test planner drives the HTTP match or accelerates its clock.
import (
	"fmt"
	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"os"
	"testing"
	"time"
)

func TestSnookerComputerSeatsHTTP(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, 1, game.Module(), "snooker")
			host := h.People[0]
			host.Request("POST", "/api/admin/games/snooker", map[string]bool{"enabled": true}, 200)
			options := map[string]any{"bestOf": 1}
			if n == 4 {
				options["teams"] = []int{0, 0, 1, 1}
			}
			host.Request("POST", "/api/rooms", map[string]any{"kind": "snooker", "name": "电脑与托管验收", "capacity": n, "options": options}, 201)
			for i := 1; i < n; i++ {
				host.Command("add_bot", nil, 200)
			}
			host.Command("ready", nil, 200)
			host.Command("start", nil, 200)
			// Reclaim during a moving shot is allowed; it must not cancel the flight.
			host.Command("autoplay", map[string]any{"enabled": true}, 200)
			until := time.Now().Add(30 * time.Second)
			for time.Now().Before(until) {
				s := publicState(t, host.Room())
				if s.Phase == "rolling" {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			before := publicState(t, host.Room())
			if before.Phase != "rolling" {
				t.Fatal("real bot scheduler never shot")
			}
			host.Command("autoplay", map[string]any{"enabled": false}, 200)
			after := publicState(t, host.Room())
			if after.ShotID != before.ShotID || after.Flight.ID != before.Flight.ID {
				t.Fatal("reclaim replaced flight")
			}
			if host.Room()["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
				t.Fatal("manual takeover failed")
			}
			h.Record("computer shot; human reclaimed without replacing flight")
			// Re-enable, then restart actual SQLite host while all seats are controlled.
			host.Command("autoplay", map[string]any{"enabled": true}, 200)
			h.Restart()
			if host.Room()["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
				t.Fatal("restore lost autoplay")
			}
			if os.Getenv("WIRE_BOARD_SNOOKER_HTTP_FULL") != "1" {
				return
			}
			deadline := time.Now().Add(7 * time.Minute)
			last := -1
			records := []any{}
			for time.Now().Before(deadline) {
				r := host.Room()
				s := publicState(t, r)
				shotID := 0
				if s.Flight != nil {
					shotID = s.Flight.ID
				}
				if shotID != last {
					last = shotID
					records = append(records, map[string]any{"shot": last, "phase": s.Phase, "turn": s.Turn, "scores": s.Scores})
					if last%10 == 0 {
						t.Logf("%d players: %d shots", n, last)
					}
				}
				if r["status"] == "finished" {
					if len(s.Results) != 1 || s.Results[0].Reason != "black" {
						t.Fatal("computer did not finish naturally", s.Results)
					}
					records = append(records, map[string]any{"final": s})
					trace(t, records)
					h.Record("computer/autoplay natural full match; real deadlines")
					h.Restart()
					restored := publicState(t, host.Room())
					if restored.Scores != s.Scores || restored.FramesWon != s.FramesWon {
						t.Fatal("finished match restore changed result")
					}
					return
				}
				time.Sleep(150 * time.Millisecond)
			}
			trace(t, records)
			t.Fatal("real computer match stalled")
		})
	}
}
