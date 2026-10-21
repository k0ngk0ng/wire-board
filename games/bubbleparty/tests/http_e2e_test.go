package bubbleparty_test

// Real authenticated HTTP/SQLite acceptance for the production bubbleparty
// module. Tests use the real clock and never mutate snapshots.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func httpView(t *testing.T, c *gametest.Client) bubbleparty.View {
	t.Helper()
	b, err := json.Marshal(gametest.Data(c.Room()))
	if err != nil {
		t.Fatal(err)
	}
	var v bubbleparty.View
	if err = json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func rejected(t *testing.T, c *gametest.Client, action map[string]any) {
	t.Helper()
	r := c.Room()
	body, _ := json.Marshal(c.ActionRequest(action))
	req, err := http.NewRequest("POST", c.Base+"/api/rooms/"+r["id"].(string), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 400 && res.StatusCode != 409 {
		t.Fatalf("invalid action accepted: %d", res.StatusCode)
	}
}

func confirmCharacters(t *testing.T, people []*gametest.Client) {
	t.Helper()
	for _, c := range people {
		v := httpView(t, c)
		if v.Phase == bubbleparty.PhaseSelect {
			c.Action(map[string]any{"type": "character", "epoch": v.Epoch, "character": 0, "confirm": true}, 200)
		}
	}
}

func waitPlaying(t *testing.T, c *gametest.Client, timeout time.Duration) bubbleparty.View {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		v := httpView(t, c)
		if v.Phase == bubbleparty.PhasePlaying {
			return v
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("playing phase not reached; last %s", httpView(t, c).Phase)
	return bubbleparty.View{}
}

func TestBubblepartyHTTPLifecycle(t *testing.T) {
	for _, players := range []int{2, 8} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			h := gametest.NewHTTP(t, players, bubbleparty.Module(), "bubbleparty")
			h.AssertUnpublished()
			h.Start(map[string]any{"map": "community10", "roundSeconds": 180})
			confirmCharacters(t, h.People)
			view := httpView(t, h.People[0])
			if view.Phase != bubbleparty.PhaseCountdown || len(view.Players) != players || view.TeamSize != players/2 || view.Epoch == "" {
				t.Fatalf("bad countdown view: %+v", view)
			}
			if httpView(t, h.Spectator).CanAct {
				t.Fatal("spectator can act")
			}
			rejected(t, h.Spectator, map[string]any{"type": "input", "epoch": view.Epoch, "seq": 1, "move": "up", "bomb": false})
			view = waitPlaying(t, h.People[0], 6*time.Second)
			if view.MapID != "community10" || !view.CanAct {
				t.Fatalf("bad playing view: %+v", view)
			}
			rejected(t, h.People[0], map[string]any{"type": "input", "epoch": "wrong", "seq": 1, "move": "up", "bomb": false})
			rejected(t, h.People[0], map[string]any{"type": "input", "epoch": view.Epoch, "seq": 0, "move": "up", "bomb": false})
			for i, c := range h.People {
				v := httpView(t, c)
				seq := v.Players[i].Seq + 1
				c.Action(map[string]any{"type": "input", "epoch": v.Epoch, "seq": seq, "move": "none", "bomb": false}, 200)
			}
			time.Sleep(250 * time.Millisecond)
			after := httpView(t, h.People[0])
			if after.Epoch != view.Epoch || after.Players[0].Seq == 0 {
				t.Fatalf("input not applied: %+v", after.Players[0])
			}
			h.Restart()
			restored := waitPlaying(t, h.People[0], 5*time.Second)
			if restored.Epoch != view.Epoch || restored.MapID != "community10" {
				t.Fatalf("restore changed match: %+v", restored)
			}
		})
	}
}

func TestBubblepartyHTTPRejectsCapacityAndMap(t *testing.T) {
	h := gametest.NewHTTP(t, 2, bubbleparty.Module(), "bubbleparty")
	h.AssertUnpublished()
	admin := h.People[0]
	admin.Request("POST", "/api/admin/games/bubbleparty", map[string]bool{"enabled": true}, 200)
	admin.Request("POST", "/api/rooms", map[string]any{"kind": "bubbleparty", "name": "odd", "capacity": 3}, 400)
	admin.Request("POST", "/api/rooms", map[string]any{
		"kind": "bubbleparty", "name": "bad map", "capacity": 2,
		"options": map[string]any{"map": "missing"},
	}, 400)
}

func TestBubblepartyHTTPHumanBotRealtime(t *testing.T) {
	h := gametest.NewHTTP(t, 1, bubbleparty.Module(), "bubbleparty")
	h.AssertUnpublished()
	admin := h.People[0]
	admin.Request("POST", "/api/admin/games/bubbleparty", map[string]bool{"enabled": true}, 200)
	admin.Request("POST", "/api/rooms", map[string]any{
		"kind": "bubbleparty", "name": "human bot", "capacity": 2,
		"options": map[string]any{"map": "waterpark", "roundSeconds": 180},
	}, 201)
	admin.Command("add_bot", nil, 200)
	admin.Command("ready", nil, 200)
	admin.Command("start", nil, 200)
	confirmCharacters(t, h.People)
	view := waitPlaying(t, admin, 6*time.Second)
	before := view.Players[0].Seq
	admin.Action(map[string]any{"type": "input", "epoch": view.Epoch, "seq": before + 1, "move": "down", "bomb": false}, 200)
	time.Sleep(300 * time.Millisecond)
	after := httpView(t, admin)
	if after.Players[0].Seq <= before {
		t.Fatalf("human input not applied: before %d after %+v", before, after.Players[0])
	}
}
