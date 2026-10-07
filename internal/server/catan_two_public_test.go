package server

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanTwoPublicCreationSelectionAndRematch(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest, third := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("公开双人房主")
	guest.register("公开双人朋友")
	third.register("公开双人旁观")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2, "catanOptions": game.CatanOptions{Helpers: true}},
		{"kind": "catan", "capacity": 2, "catanOptions": game.CatanOptions{FiveSix: true}},
		{"kind": "catan", "capacity": 3, "catanTwoScenario": "rivers"},
		{"kind": "splendor", "capacity": 2, "catanTwoScenario": "rivers"},
		{"kind": "catan", "capacity": 2, "catanTwoScenario": "unknown"},
	} {
		body["name"] = "错误配置"
		host.post("/api/rooms", body, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "capacity": 2, "name": "公开双人"}, 201)
	id := raw["id"].(string)
	if raw["catanTwoRules"] != game.CatanTwoRules {
		t.Fatal("public creation not pinned to two-player rules")
	}
	host.command(current(host), "ready", nil, 200)
	host.command(current(host), "start", nil, 400)
	guest.command(current(host), "join", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	third.command(current(host), "join", nil, 400)
	change := func(c *testClient, scenario any, status int) map[string]any {
		r := s.rooms[id]
		body := map[string]any{"type": "catan_two_scenario", "version": r.Version, "nonce": randomID(12), "catanTwoScenario": scenario}
		c.post("/api/rooms/"+id, body, status)
		return body
	}
	for _, scenario := range []string{"rivers", "caravans", ""} {
		before, _ := json.Marshal(s.rooms[id])
		change(guest, scenario, 400)
		change(host, "unknown", 400)
		change(host, nil, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("invalid selection changed room")
		}
		request := change(host, scenario, 200)
		r := s.rooms[id]
		if r.CatanTwoScenario != scenario || r.Capacity != 2 || r.CatanTwoRules != game.CatanTwoRules {
			t.Fatal("wrong selected scenario")
		}
		for _, seat := range r.Seats {
			if seat.Ready {
				t.Fatal("changed recipe retained readiness")
			}
		}
		s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest, third}, id)
		before, _ = json.Marshal(s.rooms[id])
		host.post("/api/rooms/"+id, request, 200)
		after, _ = json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("selection replay changed state")
		}
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
		change(host, scenario, 200)
		for _, seat := range s.rooms[id].Seats {
			if !seat.Ready {
				t.Fatal("unchanged recipe cleared readiness")
			}
		}
	}
	host.command(current(host), "start", nil, 200)
	if s.rooms[id].Game.Catan.Two == nil {
		t.Fatal("base variant not constructed")
	}
	before, _ := json.Marshal(s.rooms[id])
	change(host, "rivers", 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("playing recipe changed")
	}
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	change(host, "rivers", 200)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	if s.rooms[id].Game.Catan.Rivers == nil || s.rooms[id].Game.Phase != "catan_rivers_start" {
		t.Fatal("rematch retained previous base recipe")
	}
}
