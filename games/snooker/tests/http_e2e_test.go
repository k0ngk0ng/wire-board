package snooker_test

// Real HTTP lifecycle evidence; no private debug routes or fake clock actions.
import (
	"fmt"
	"reflect"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func awaitStopped(t *testing.T, h *gametest.HTTP) map[string]any {
	t.Helper()
	until := time.Now().Add(15 * time.Second)
	for time.Now().Before(until) {
		r := h.People[0].Room()
		if gametest.Data(r)["phase"] != "rolling" {
			return r
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("shot system timer never resolved")
	return nil
}
func TestSnookerHTTPPublicationShotRestoreAndIdempotence(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, game.Module(), "snooker")
			h.AssertUnpublished()
			h.Start(map[string]any{"bestOf": 1})
			d := gametest.Data(h.People[0].Room())
			chooser := int(d["turn"].(float64))
			h.People[chooser].Action(map[string]any{"type": "break_choice", "team": 0, "window": d["window"]}, 200)
			d = gametest.Data(h.People[0].Room())
			initial := d["balls"]
			h.People[n-1].Action(map[string]any{"type": "shoot", "angle": 0, "power": .06, "window": d["window"]}, 400)
			request := h.People[0].ActionRequest(map[string]any{"type": "shoot", "angle": 0, "power": .06, "window": d["window"]})
			id := h.People[0].Room()["id"].(string)
			h.People[0].Request("POST", "/api/rooms/"+id, request, 200)
			moving := h.People[0].Room()
			md := gametest.Data(moving)
			if md["phase"] != "rolling" || !reflect.DeepEqual(md["scores"], []any{float64(0), float64(0)}) {
				t.Fatal("premature score")
			}
			h.People[0].Request("POST", "/api/rooms/"+id, request, 200)
			if gametest.Data(h.People[0].Room())["flight"].(map[string]any)["id"] != float64(1) {
				t.Fatal("duplicate stroke")
			}
			h.Record("natural opening stroke is rolling, score still unchanged")
			h.Restart()
			restored := gametest.Data(h.People[0].Room())
			if !reflect.DeepEqual(md["flight"], restored["flight"]) {
				t.Fatal("restart recalculated or lost trajectory")
			}
			room := awaitStopped(t, h)
			done := gametest.Data(room)
			if done["phase"] != "foul" || done["scores"].([]any)[1] != float64(4) {
				t.Fatal("missing once-only foul", done)
			}
			actor := int(done["turn"].(float64))
			h.People[actor].Action(map[string]any{"type": "foul_choice", "choice": "restore", "window": done["window"]}, 200)
			again := gametest.Data(h.People[0].Room())
			if !reflect.DeepEqual(again["balls"], initial) || again["turn"] != float64(0) || again["scores"].([]any)[1] != float64(4) {
				t.Fatal("original offender/balls/penalty did not restore")
			}
			h.People[0].Request("POST", "/api/rooms/"+id, request, 200)
			if !reflect.DeepEqual(again, gametest.Data(h.People[0].Room())) {
				t.Fatal("old nonce scored again")
			}
			h.Record("original position restored with penalty retained and original offender")
			h.Restart()
			if !reflect.DeepEqual(again, gametest.Data(h.People[0].Room())) {
				t.Fatal("restored decision not persistent")
			}
		})
	}
}
