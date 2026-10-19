package battlecity_test

// Real authenticated HTTP/SQLite acceptance for the production battlecity
// module. These tests never mutate snapshots or replace the host clock.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	battlecity "github.com/k0ngk0ng/wire-board/games/battlecity/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func httpView(t *testing.T, c *gametest.Client) battlecity.View {
	t.Helper()
	b, err := json.Marshal(gametest.Data(c.Room()))
	if err != nil {
		t.Fatal(err)
	}
	var v battlecity.View
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

func waitPhase(t *testing.T, c *gametest.Client, phase string, timeout time.Duration) battlecity.View {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		v := httpView(t, c)
		if v.Phase == phase {
			return v
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("phase %s not reached; last %+v", phase, httpView(t, c).Phase)
	return battlecity.View{}
}

func TestBattlecityHTTPLifecycle(t *testing.T) {
	for _, players := range []int{2, 6} {
		t.Run(fmt.Sprint(players), func(t *testing.T) {
			h := gametest.NewHTTP(t, players, battlecity.Module(), "battlecity")
			h.AssertUnpublished()
			h.Start(map[string]any{"map": "classic-01"})
			view := httpView(t, h.People[0])
			if view.Phase != battlecity.PhaseSelect || len(view.Tanks) != players || view.TeamSize != players/2 || view.Epoch == "" {
				t.Fatalf("bad select view: %+v", view)
			}
			// A spectator cannot choose.
			rejected(t, h.Spectator, map[string]any{"type": "select", "epoch": view.Epoch, "tank": "balanced"})
			for i, c := range h.People {
				c.Action(map[string]any{"type": "select", "epoch": view.Epoch, "tank": tankOrderFor(i)}, 200)
			}
			view = waitPhase(t, h.People[0], battlecity.PhaseCountdown, 2*time.Second)
			if view.Phase != battlecity.PhaseCountdown {
				t.Fatalf("no countdown: %s", view.Phase)
			}
			view = waitPhase(t, h.People[0], battlecity.PhasePlaying, 6*time.Second)
			if view.Bases[0].Alive == false || view.Bases[1].Alive == false {
				t.Fatal("base died at start")
			}
			if !view.Controls.CanPlay {
				t.Fatal("seated player cannot play")
			}
			if httpView(t, h.Spectator).Controls.CanPlay {
				t.Fatal("spectator can play")
			}
			rejected(t, h.Spectator, map[string]any{"type": "input", "epoch": view.Epoch, "seq": 1, "move": "up", "fire": false})
			// Wrong epoch and replayed sequence are rejected before they reach
			// the authoritative simulation.
			rejected(t, h.People[0], map[string]any{"type": "input", "epoch": "wrong", "seq": 1, "move": "up", "fire": false})
			rejected(t, h.People[0], map[string]any{"type": "input", "epoch": view.Epoch, "seq": 0, "move": "up", "fire": false})
			for i, c := range h.People {
				v := httpView(t, c)
				seq := v.Tanks[i].Seq + 1
				if seq < 2 {
					seq = 1
				}
				c.Action(map[string]any{"type": "input", "epoch": v.Epoch, "seq": seq, "move": "none", "fire": false}, 200)
			}
			time.Sleep(250 * time.Millisecond)
			after := httpView(t, h.People[0])
			if after.Epoch != view.Epoch {
				t.Fatal("epoch changed on input")
			}
			for i := range after.Tanks {
				if after.Tanks[i].Seq == 0 {
					t.Fatalf("seat %d sequence not applied: %+v", i, after.Tanks[i])
				}
			}
			// SQLite restore must keep the same match identity and seats.
			h.Restart()
			restored := waitPhase(t, h.People[0], battlecity.PhasePlaying, 5*time.Second)
			if restored.Epoch != view.Epoch || restored.TeamSize != players/2 {
				t.Fatalf("restore changed match: %+v", restored)
			}
		})
	}
}

func tankOrderFor(seat int) string {
	order := []string{"balanced", "basic", "fast", "power", "armor"}
	return order[seat%len(order)]
}

func TestBattlecityHTTPRejectsWrongCapacityAndMap(t *testing.T) {
	h := gametest.NewHTTP(t, 2, battlecity.Module(), "battlecity")
	h.AssertUnpublished()
	admin := h.People[0]
	admin.Request("POST", "/api/admin/games/battlecity", map[string]bool{"enabled": true}, 200)
	admin.Request("POST", "/api/rooms", map[string]any{"kind": "battlecity", "name": "odd", "capacity": 3}, 400)
	admin.Request("POST", "/api/rooms", map[string]any{
		"kind": "battlecity", "name": "bad map", "capacity": 2,
		"options": map[string]any{"map": "missing"},
	}, 400)
}

func TestBattlecityHTTPHumanBotRealtime(t *testing.T) {
	h := gametest.NewHTTP(t, 1, battlecity.Module(), "battlecity")
	h.AssertUnpublished()
	admin := h.People[0]
	admin.Request("POST", "/api/admin/games/battlecity", map[string]bool{"enabled": true}, 200)
	admin.Request("POST", "/api/rooms", map[string]any{
		"kind": "battlecity", "name": "human bot", "capacity": 2,
		"options": map[string]any{"map": "classic-01"},
	}, 201)
	admin.Command("add_bot", nil, 200)
	admin.Command("ready", nil, 200)
	admin.Command("start", nil, 200)
	v := httpView(t, admin)
	if v.Phase != battlecity.PhaseSelect || len(v.Tanks) != 2 {
		t.Fatalf("bad select: %+v", v)
	}
	admin.Action(map[string]any{"type": "select", "epoch": v.Epoch, "tank": "balanced"}, 200)
	v = waitPhase(t, admin, battlecity.PhasePlaying, 8*time.Second)
	if v.Phase != battlecity.PhasePlaying {
		t.Fatalf("no playing: %s", v.Phase)
	}
	before := v.Tanks[0].Seq
	admin.Action(map[string]any{"type": "input", "epoch": v.Epoch, "seq": before + 1, "move": "down", "fire": false}, 200)
	time.Sleep(250 * time.Millisecond)
	after := httpView(t, admin)
	if after.Tanks[0].Seq <= before {
		t.Fatalf("human input not applied in human/bot room: before %d after %+v", before, after.Tanks[0])
	}
	t.Logf("human seq %d -> %d, bot seq %d", before, after.Tanks[0].Seq, after.Tanks[1].Seq)
}
