package stoneage_test

// HTTP scenarios precede implementation; see FAILURE_SCENARIOS.md.
import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	stoneage "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func httpView(t *testing.T, c *gametest.Client) stoneage.View {
	t.Helper()
	var v stoneage.View
	if e := json.Unmarshal(raw(gametest.Data(c.Room())), &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func httpSuggestion(t *testing.T, c *gametest.Client) map[string]any {
	t.Helper()
	a := stoneage.Suggest(httpView(t, c))
	if a == nil {
		return nil
	}
	return obj(t, a)
}
func httpSelection(t *testing.T, h *gametest.HTTP) {
	t.Helper()
	for pass := 0; pass < 3; pass++ {
		for _, c := range h.People {
			a := httpSuggestion(t, c)
			if a == nil {
				t.Fatal("no selection suggestion")
			}
			c.Action(a, 200)
		}
	}
	if httpView(t, h.People[0]).Phase != "planning" {
		t.Fatal("selection did not finish")
	}
}
func TestStoneageHTTPAllModesCompleteAndRestart(t *testing.T) {
	for _, n := range []int{2, 4, 6, 8, 10} {
		t.Run(fmt.Sprintf("%dv%d", n/2, n/2), func(t *testing.T) {
			h := gametest.NewHTTP(t, n, acceleratedHTTPModule(), "stoneage")
			h.AssertPublished()
			h.Start(map[string]any{})
			first := gametest.Data(h.People[0].Room())["pool"]
			for _, c := range h.People {
				if !reflect.DeepEqual(first, gametest.Data(c.Room())["pool"]) {
					t.Fatal("different pool")
				}
			}
			h.Spectator.Action(map[string]any{"type": "lock", "session": httpView(t, h.People[0]).Session, "window": 1}, 400)
			httpSelection(t, h)
			for rounds := 0; rounds < 26; rounds++ {
				if h.People[0].Room()["status"] == "finished" {
					break
				}
				for _, c := range h.People {
					a := httpSuggestion(t, c)
					if a != nil {
						c.Action(a, 200)
					}
				}
				if rounds == 0 {
					before := gametest.Data(h.People[0].Room())
					h.Restart()
					after := gametest.Data(h.People[0].Room())
					if !reflect.DeepEqual(before, after) {
						t.Fatal("restart changed round, configuration, inventory or RNG projection")
					}
					h.Record("restored first combat round")
				}
				until := time.Now().Add(60 * time.Second)
				for httpView(t, h.People[0]).Phase == "resolving" && time.Now().Before(until) {
					time.Sleep(20 * time.Millisecond)
				}
				if httpView(t, h.People[0]).Phase == "resolving" {
					t.Fatal("round timer stalled")
				}
			}
			end := h.People[0].Room()
			if end["status"] != "finished" {
				t.Fatal("real HTTP match did not finish")
			}
			h.Record("complete natural match")
			report(t, fmt.Sprintf("http-complete-%dv%d", n/2, n/2), end)
		})
	}
}
func TestStoneageHTTPNoncePrivacyAndSubmittedRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 4, stoneage.Module(), "stoneage")
	h.Start(map[string]any{})
	httpSelection(t, h)
	c := h.People[0]
	a := httpSuggestion(t, c)
	payload := c.ActionRequest(a)
	r := c.Room()
	url := "/api/rooms/" + r["id"].(string)
	c.Request("POST", url, payload, 200)
	before := gametest.Data(c.Room())
	c.Request("POST", url, payload, 200)
	if !reflect.DeepEqual(before, gametest.Data(c.Room())) {
		t.Fatal("nonce replay changed game")
	}
	for _, other := range []*gametest.Client{h.People[1], h.Spectator} {
		v := gametest.Data(other.Room())
		players := v["players"].([]any)
		if players[0].(map[string]any)["plan"] != nil {
			t.Fatal("private plan leaked")
		}
	}
	h.Restart()
	if !reflect.DeepEqual(before, gametest.Data(c.Room())) {
		t.Fatal("submitted instruction lost")
	}
	c.Action(a, 400)
	h.People[1].Command("autoplay", map[string]any{"enabled": true}, 200)
	h.People[1].Command("autoplay", map[string]any{"enabled": false}, 200)
	h.Record("idempotent submitted plan, private projections, restart and reclaim")
	report(t, "http-nonce-private-restore", c.Room())
}
