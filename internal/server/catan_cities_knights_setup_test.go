package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func provisionCatanCitiesKnights(t *testing.T, s *Server, id string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rooms[id].setCatanCitiesKnights(game.CatanCitiesKnightsSetup{}); err != nil {
		t.Fatal(err)
	}
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
}
func selectCatanCitiesKnights(c *testClient, setup *game.CatanCitiesKnightsSetup, status int) {
	r := current(c)
	c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": setup, "version": r["version"], "nonce": randomID(12)}, status)
}
func TestCatanCitiesKnightsConfigurationHTTPPermissionsRestartAndHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("骑士房主")
	guest.register("骑士客人")
	config := game.CatanCitiesKnightsSetup{Layout: "variable"}
	host.post("/api/rooms", map[string]any{"kind": "catan", "name": "未开放", "capacity": 4, "catanCitiesKnights": config}, 400)
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "骑士设置", "capacity": 4}, 201)
	id := raw["id"].(string)
	if raw["catanCitiesKnights"] != nil {
		t.Fatal("unfinished setting exposed")
	}
	guest.command(current(host), "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	selectCatanCitiesKnights(host, &config, 400)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	provisionCatanCitiesKnights(t, s, id)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("enable must reset human readiness")
		}
	}
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	selectCatanCitiesKnights(guest, &config, 400)
	selectCatanCitiesKnights(host, nil, 400)
	selectCatanCitiesKnights(host, &game.CatanCitiesKnightsSetup{Layout: "fixed"}, 400)
	selectCatanCitiesKnights(host, &game.CatanCitiesKnightsSetup{Rules: game.CatanCitiesKnightsFiveSixRules}, 400)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{Helpers: true}, "version": current(host)["version"], "nonce": randomID(12)}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejected configuration mutated room")
	}
	selectCatanCitiesKnights(host, &config, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same configuration reset readiness")
		}
	}
	stale := current(host)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: true}, "version": stale["version"], "nonce": randomID(12)}, 200)
	r := s.rooms[id]
	if r.Capacity != 5 || r.CatanCitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules {
		t.Fatal("wrong five-six recipe")
	}
	for _, seat := range r.Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("extension change must reset ready")
		}
	}
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": config, "version": stale["version"], "nonce": randomID(12)}, 409)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	host.command(current(host), "start", nil, 400) // Three seats cannot start five-six recipe.
	for range 2 {
		host.command(current(host), "add_bot", nil, 200)
	}
	before, _ = json.Marshal(s.rooms[id])
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	after, _ = json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("waiting configuration/readiness lost")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	host.base, guest.base = ts2.URL, ts2.URL
	host.command(current(host), "start", nil, 200)
	r = next.rooms[id]
	if r.Game.Phase != "catan_setup_settlement" || len(r.Game.Catan.Players) != 5 || r.Game.Catan.CitiesKnights == nil || r.Game.Catan.CitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules || len(r.Game.Catan.Bank) != 8 || r.Game.Catan.Bank[5] != 18 {
		t.Fatal("formal start did not use cities-knights five-player rules")
	}
	if left := r.TurnDeadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
		t.Fatal("initial setup clock")
	}
	selectCatanCitiesKnights(host, &config, 400)
	// Archive from the actual state even if an old/malformed room draft lingers.
	r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{Rules: "stale", Layout: "fixed"}
	r.CatanOptions = game.CatanOptions{Helpers: true}
	host.command(current(host), "close", nil, 200)
	_, profile := host.request("GET", "/api/players/"+r.Host, nil)
	match := profile["history"].([]any)[0].(map[string]any)
	if match["catanLayout"] != "variable" || match["catanRules"] != game.CatanCitiesKnightsFiveSixRules || match["catanScenario"] != nil {
		t.Fatal("wrong frozen history identity", match)
	}
	ex := match["catanExpansions"].([]any)
	if len(ex) != 1 || ex[0] != "cities_knights" {
		t.Fatal("missing expansion identity")
	}
	o := match["catanOptions"].(map[string]any)
	if o["helpers"] == true || o["fiveSix"] != true {
		t.Fatal("history trusted mutable room options")
	}
}
func TestCatanCitiesKnightsConfigurationRecipeChangesAndMixedRejection(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("骑士切换")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "骑士人数", "capacity": 6, "catanOptions": game.CatanOptions{FiveSix: true}}, 201)
	id := raw["id"].(string)
	provisionCatanCitiesKnights(t, s, id)
	for _, six := range []bool{false, true} {
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: six}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
		want := game.CatanCitiesKnightsRules
		if six {
			want = game.CatanCitiesKnightsFiveSixRules
		}
		if s.rooms[id].CatanCitiesKnights.Rules != want {
			t.Fatal("recipe did not follow extension")
		}
	}
	r := s.rooms[id]
	if err := r.setCatanSeafarers(game.CatanSeafarersSetup{Scenario: "fog"}); err == nil {
		t.Fatal("unverified seafarers combination")
	}
	if err := r.setCatanBaseConfiguration(game.CatanBaseConfiguration{}); err == nil {
		t.Fatal("mixed base layout")
	}
	for range 4 {
		host.command(current(host), "add_bot", nil, 200)
	}
	host.command(current(host), "ready", nil, 200)
	for _, kind := range []string{"base", "sea", "world"} {
		r = s.rooms[id]
		switch kind {
		case "base":
			r.CatanBaseConfiguration = &game.CatanBaseConfiguration{Layout: "variable"}
		case "sea":
			r.CatanSeafarers = &game.CatanSeafarersSetup{Scenario: "fog"}
		case "world":
			r.CatanNewWorldMap = &game.CatanNewWorldMap{}
		}
		before, _ := json.Marshal(r)
		host.command(current(host), "start", nil, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("mixed start mutated room")
		}
		r.CatanBaseConfiguration = nil
		r.CatanSeafarers = nil
		r.CatanNewWorldMap = nil
	}
	other := *r
	other.CatanCitiesKnights = nil
	other.CatanOptions.Helpers = true
	if err := other.setCatanCitiesKnights(game.CatanCitiesKnightsSetup{}); err == nil {
		t.Fatal("Helpers provision accepted")
	}
	other.CatanOptions.Helpers = false
	other.Status = "playing"
	if err := other.setCatanCitiesKnights(game.CatanCitiesKnightsSetup{}); err == nil {
		t.Fatal("late configuration accepted")
	}
}
