package server

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanScenarioPublicSelectionAndRematch(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("多人剧本房主")
	guest.register("多人剧本朋友")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2, "catanScenario": "rivers"},
		{"kind": "catan", "capacity": 3, "catanScenario": "unknown"},
		{"kind": "splendor", "capacity": 3, "catanScenario": "caravans"},
		{"kind": "catan", "capacity": 3, "catanScenario": "rivers", "catanOptions": game.CatanOptions{Helpers: true}},
		{"kind": "catan", "capacity": 4, "catanScenario": "caravans", "catanOptions": game.CatanOptions{Helpers: true, AllHelpers: true}},
		{"kind": "catan", "capacity": 5, "catanScenario": "rivers", "catanOptions": game.CatanOptions{}},
		{"kind": "catan", "capacity": 6, "catanScenario": "caravans", "catanOptions": game.CatanOptions{}},
	} {
		body["name"] = "无效剧本"
		host.post("/api/rooms", body, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "capacity": 4, "name": "河流与商队", "catanScenario": "rivers"}, 201)
	id := raw["id"].(string)
	if raw["catanScenario"] != "rivers" {
		t.Fatal("creation did not expose selected scenario")
	}
	guest.command(current(host), "join", nil, 200)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 400) // A four-seat room still needs three actual players.
	host.command(current(host), "add_bot", nil, 200)
	change := func(c *testClient, scenario any, status int) map[string]any {
		body := map[string]any{"type": "catan_scenario", "catanScenario": scenario, "version": s.rooms[id].Version, "nonce": randomID(12)}
		c.post("/api/rooms/"+id, body, status)
		return body
	}
	for _, scenario := range []string{"caravans", "", "rivers"} {
		before, _ := json.Marshal(s.rooms[id])
		change(guest, scenario, 400)
		change(host, "unknown", 400)
		change(host, nil, 400)
		if s.rooms[id].CatanScenario != "" {
			for _, options := range []game.CatanOptions{{Helpers: true}, {FiveSix: true, Helpers: true}} {
				host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": options, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
			}
		}
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("invalid selection changed room")
		}
		request := change(host, scenario, 200)
		for _, seat := range s.rooms[id].Seats {
			if seat.Ready != seat.Bot {
				t.Fatal("recipe change must clear only human readiness")
			}
		}
		s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
		if s.rooms[id].CatanScenario != scenario {
			t.Fatal("scenario lost after restart")
		}
		before, _ = json.Marshal(s.rooms[id])
		host.post("/api/rooms/"+id, request, 200)
		after, _ = json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("replayed selection changed room")
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
	if s.rooms[id].Game.Catan.Rivers == nil || len(s.rooms[id].Game.Catan.Players) != 3 {
		t.Fatal("start must construct river map for actual player count")
	}
	before, _ := json.Marshal(s.rooms[id])
	change(host, "caravans", 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("playing recipe changed")
	}
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	change(host, "caravans", 200)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	if s.rooms[id].Game.Catan.Caravans == nil || s.rooms[id].Game.Catan.Rivers != nil {
		t.Fatal("rematch retained previous river recipe")
	}
}

func TestCatanScenarioStartRejectsMixedSavedConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("剧本恢复检查")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "capacity": 3, "name": "恢复检查", "catanScenario": "caravans"}, 201)
	id := raw["id"].(string)
	for range 2 {
		host.command(current(host), "add_bot", nil, 200)
	}
	host.command(current(host), "ready", nil, 200)
	valid := *s.rooms[id]
	for _, mutate := range []func(*Room){
		func(r *Room) { r.CatanScenario = "unknown" },
		func(r *Room) { r.CatanTwoRules = game.CatanTwoRules },
		func(r *Room) { r.CatanTwoScenario = "rivers" },
		func(r *Room) { r.CatanBaseConfiguration = &game.CatanBaseConfiguration{} },
		func(r *Room) { r.CatanSeafarers = &game.CatanSeafarersSetup{} },
		func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} },
		func(r *Room) { r.CatanHarbors = &game.CatanHarborsSetup{} },
		func(r *Room) { r.CatanFriendlyRobber = &game.CatanFriendlyRobberSetup{} },
		func(r *Room) { r.CatanNewWorldMap = &game.CatanNewWorldMap{} },
		func(r *Room) { r.CatanOptions.Helpers = true },
		func(r *Room) { r.Capacity = 5; r.CatanOptions.FiveSix = true },
	} {
		trial := valid
		mutate(&trial)
		s.rooms[id] = &trial // Explicit malformed saved-room fixture, never a public recipe.
		before, _ := json.Marshal(s.rooms[id])
		host.command(current(host), "start", nil, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("invalid recipe fell through to a different game")
		}
	}
	s.rooms[id] = &valid
	host.command(current(host), "start", nil, 200)
}
