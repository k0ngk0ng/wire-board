package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func newPublicExplorerHTTP(t *testing.T, n int) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	return newPublicExplorerScenarioHTTP(t, n, "land-ho")
}

func newPublicExplorerScenarioHTTP(t *testing.T, n int, scenario string, knights ...bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	city := len(knights) > 0 && knights[0]
	return newPublicExplorerRecipeHTTP(t, n, scenario, city, false)
}

func newPublicExplorerRecipeHTTP(t *testing.T, n int, scenario string, knights, events bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("初航公开玩家%d", p))
	}
	recipe := map[string]any{"name": "探险公开完整局", "kind": "catan", "capacity": n, "catanScenario": scenario}
	if knights {
		recipe["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{Layout: "variable"}
	}
	if events {
		recipe["catanEvents"] = game.CatanEventCatalogue
	}
	raw := clients[0].post("/api/rooms", recipe, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[p]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	r := s.rooms[id]
	if r.CatanTwoRules != "" || r.Game.Catan.Two != nil || r.Game.Catan.Explorer == nil || r.Game.Catan.Explorer.Board.Scenario != scenario || r.Game.Catan.Explorer.Board.Players != n || (r.Game.Catan.EventDeck != nil) != events {
		t.Fatal("wrong public opening")
	}
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, clients, id
}

func assertPublicExplorerHistory(t *testing.T, s *Server, clients []*testClient, id string) {
	t.Helper()
	r := s.rooms[id]
	winner := r.Game.Winners[0]
	// Repeated archive attempts must keep one result and the actual game recipe.
	s.mu.Lock()
	err := s.save(r)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	status, profile := clients[len(clients)-1].request("GET", "/api/players/"+r.Seats[winner].ID, nil)
	if status != 200 {
		t.Fatal(status)
	}
	history := profile["history"].([]any)
	if len(history) != 1 {
		t.Fatal("duplicate history")
	}
	record := history[0].(map[string]any)
	if record["catanScenario"] != r.Game.Catan.Explorer.Board.Scenario || record["catanLayout"] != r.Game.Catan.Explorer.Board.Layout || record["catanExpansionRules"].(map[string]any)["explorers_pirates"] != r.Game.Catan.Explorer.Board.Rules {
		t.Fatal("wrong scenario history", record)
	}
	if lairs := r.Game.Catan.Explorer.Lairs; lairs != nil && lairs.NumberRecipe != "" {
		if record["catanExpansionRules"].(map[string]any)["lair_numbers"] != game.CatanExplorerLairRecipe {
			t.Fatal("lost lair recipe history")
		}
	}
	if r.Game.Catan.CitiesKnights != nil {
		if record["catanExpansionRules"].(map[string]any)["cities_knights"] != r.Game.Catan.CitiesKnightsSetup().Rules {
			t.Fatal("lost knights history")
		}
	}
	stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
	if stats["played"] != float64(1) || stats["wins"] != float64(1) {
		t.Fatal("duplicate score")
	}
}

func TestCatanExplorerPublicSelectionAndRematch(t *testing.T) {
	for _, n := range []int{2, 3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
			host.register("初航房主")
			guest.register("初航朋友")
			raw := host.post("/api/rooms", map[string]any{"name": "初航设置", "kind": "catan", "capacity": n, "catanScenario": "land-ho"}, 201)
			id := raw["id"].(string)
			guest.command(current(host), "join", nil, 200)
			change := func(c *testClient, kind, key, value string, status int) {
				c.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			for _, c := range []*testClient{host, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			change(host, "catan_scenario", "catanScenario", "land-ho", 200)
			if !s.rooms[id].Seats[0].Ready || !s.rooms[id].Seats[1].Ready {
				t.Fatal("same scenario cleared readiness")
			}
			before, _ := json.Marshal(s.rooms[id])
			change(guest, "catan_scenario", "catanScenario", "land-ho", 400)
			change(host, "catan_scenario", "catanScenario", "unknown-mission", 400)
			host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{Helpers: true}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid command changed room")
			}
			if n == 2 {
				change(host, "catan_two_scenario", "catanTwoScenario", "rivers", 200)
			} else {
				change(host, "catan_scenario", "catanScenario", "shores", 200)
			}
			change(host, "catan_scenario", "catanScenario", "land-ho", 200)
			if s.rooms[id].CatanTwoRules != "" || s.rooms[id].CatanSeafarers != nil || s.rooms[id].Seats[0].Ready || s.rooms[id].Seats[1].Ready {
				t.Fatal("previous recipe or readiness survived")
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
			// A larger room may start with two actual players, using the two-player printed opening.
			for _, c := range []*testClient{host, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			if s.rooms[id].Game.Catan.Explorer.Board.Players != 2 {
				t.Fatal("capacity used instead of players")
			}
			change(host, "catan_scenario", "catanScenario", "land-ho", 400)
			host.command(current(host), "close", nil, 200)
			host.command(current(host), "rematch", nil, 200)
			if s.rooms[id].CatanScenario != "land-ho" {
				t.Fatal("rematch lost scenario")
			}
			if n == 2 {
				change(host, "catan_two_scenario", "catanTwoScenario", "", 200)
			} else {
				change(host, "catan_scenario", "catanScenario", "", 200)
				host.command(current(host), "add_bot", nil, 200)
			}
			for _, c := range []*testClient{host, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			if s.rooms[id].Game.Catan.Explorer != nil || (s.rooms[id].Game.Catan.Two != nil) != (n == 2) {
				t.Fatal("rematch used old explorer state")
			}
		})
	}
}

func TestCatanExplorerPublicRejectsUnsupportedRecipes(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	c := newClient(t, ts.URL)
	c.register("初航组合检查")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2, "catanTwoScenario": "rivers"},
		{"kind": "catan", "capacity": 3, "catanOptions": game.CatanOptions{Helpers: true}},
		{"kind": "catan", "capacity": 5, "catanOptions": game.CatanOptions{FiveSix: true}},
		{"kind": "splendor", "capacity": 2},
	} {
		body["name"] = "不支持组合"
		body["catanScenario"] = "land-ho"
		c.post("/api/rooms", body, 400)
	}
	r := Room{Kind: "catan", Capacity: 3, Status: "waiting", CatanScenario: "land-ho"}
	for _, corrupt := range []func(*Room){
		func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} },
		func(r *Room) { r.CatanTwoRules = game.CatanTwoRules },
		func(r *Room) { r.CatanSeafarers = &game.CatanSeafarersSetup{} },
		func(r *Room) { r.CatanNewWorldMap = &game.CatanNewWorldMap{} },
		func(r *Room) { r.CatanHarbors = &game.CatanHarborsSetup{} },
		func(r *Room) { r.CatanFriendlyRobber = &game.CatanFriendlyRobberSetup{} },
		func(r *Room) { r.CatanBaseConfiguration = &game.CatanBaseConfiguration{} },
		func(r *Room) { r.Capacity = 1 },
	} {
		bad := r
		corrupt(&bad)
		if bad.validateCatanScenario() == nil {
			t.Fatal("invalid saved configuration accepted")
		}
	}
}
