package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
)

func provisionCatanHarbors(t *testing.T, s *Server, id string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rooms[id].setCatanHarbors(game.CatanHarborsSetup{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
}
func selectCatanHarbors(c *testClient, enabled bool, status int) {
	c.post("/api/rooms/"+current(c)["id"].(string), map[string]any{"type": "catan_harbors", "catanHarbors": game.CatanHarborsSetup{Enabled: enabled}, "version": current(c)["version"], "nonce": randomID(12)}, status)
}
func TestCatanHarborsConfigurationHTTPPermissionsRestartAndHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("港口房主")
	guest.register("港口客人")
	// Two-player rooms support the award now; allHelpers without helpers is invalid.
	host.post("/api/rooms", map[string]any{"kind": "catan", "name": "不支持的双人", "capacity": 2, "catanOptions": game.CatanOptions{AllHelpers: true}, "catanHarbors": game.CatanHarborsSetup{Enabled: true}}, 400)
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "港口设置", "capacity": 4}, 201)
	id := raw["id"].(string)
	if raw["catanHarbors"] != nil {
		t.Fatal("unfinished option exposed")
	}
	guest.command(current(host), "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	selectCatanHarbors(host, false, 200)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	provisionCatanHarbors(t, s, id)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("provision did not reset readiness")
		}
	}
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	selectCatanHarbors(guest, false, 400)
	for _, setup := range []any{nil, game.CatanHarborsSetup{Enabled: true, Rules: "wrong"}} {
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_harbors", "catanHarbors": setup, "version": current(host)["version"], "nonce": randomID(12)}, 400)
	}
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid configuration changed room")
	}
	selectCatanHarbors(host, true, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same config reset readiness")
		}
	}
	stale := current(host)
	selectCatanHarbors(host, false, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("disable did not reset readiness")
		}
	}
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_harbors", "catanHarbors": game.CatanHarborsSetup{Enabled: true}, "version": stale["version"], "nonce": randomID(12)}, 409)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{Helpers: true}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	selectCatanHarbors(host, true, 200)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: true}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	selectCatanHarbors(host, true, 200)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	selectCatanHarbors(host, true, 200)
	before, _ = json.Marshal(s.rooms[id])
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: true}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	after, _ = json.Marshal(s.rooms[id])
	if s.rooms[id].Capacity != 4 || !s.rooms[id].CatanHarbors.Enabled {
		t.Fatal("extended transition lost award")
	}
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
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
		t.Fatal("waiting save lost")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	host.base, guest.base = ts2.URL, ts2.URL
	host.command(current(host), "start", nil, 200)
	r := next.rooms[id]
	if r.Game.Catan.Harbors == nil || len(r.Game.Catan.Players) != 3 || r.Game.View(0)["catan"].(map[string]any)["victoryTarget"] != 11 {
		t.Fatal("formal start missing variant")
	}
	selectCatanHarbors(host, false, 400)
	r.CatanHarbors = &game.CatanHarborsSetup{Enabled: false, Rules: "stale"}
	host.command(current(host), "close", nil, 200)
	_, profile := host.request("GET", "/api/players/"+r.Host, nil)
	history := profile["history"].([]any)[0].(map[string]any)
	if history["catanExpansions"].([]any)[0] != "harbors" || history["catanExpansionRules"].(map[string]any)["harbors"] != game.CatanHarborsRules {
		t.Fatal("history used stale draft", history)
	}
}

func TestCatanHarborsConfigurationCombinedTargetsAndArchives(t *testing.T) {
	for _, ck := range []bool{false, true} {
		s, ts := setupServer(t)
		stopBotTicker(s)
		host := newClient(t, ts.URL)
		host.register("港口组合")
		raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "港口组合", "capacity": 3}, 201)
		id := raw["id"].(string)
		provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: "wonders"})
		if ck {
			provisionCatanCitiesKnights(t, s, id)
		}
		provisionCatanHarbors(t, s, id)
		for _, choice := range summary(s.rooms[id])["catanSeafarersChoices"].([]game.CatanSeafarersScenario) {
			if choice.ID == "wonders" && choice.VictoryPoints != 11+map[bool]int{false: 0, true: 2}[ck] {
				t.Fatal("waiting target", choice)
			}
		}
		for range 2 {
			host.command(current(host), "add_bot", nil, 200)
		}
		host.command(current(host), "ready", nil, 200)
		host.command(current(host), "start", nil, 200)
		r := s.rooms[id]
		if r.Game.Catan.Harbors == nil || (r.Game.Catan.CitiesKnights != nil) != ck {
			t.Fatal("combination start")
		}
		r.CatanHarbors = nil
		r.CatanSeafarers = nil
		r.CatanCitiesKnights = nil
		host.command(current(host), "close", nil, 200)
		_, profile := host.request("GET", "/api/players/"+r.Host, nil)
		history := profile["history"].([]any)[0].(map[string]any)
		versions := history["catanExpansionRules"].(map[string]any)
		if versions["harbors"] != game.CatanHarborsRules || versions["seafarers"] != game.CatanSeafarersRules || (versions["cities_knights"] != nil) != ck {
			t.Fatal("combined history dropped version", history)
		}
	}
}
