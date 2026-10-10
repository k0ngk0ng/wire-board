package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
)

func provisionCatanFriendlyRobber(t *testing.T, s *Server, id string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rooms[id].setCatanFriendlyRobber(game.CatanFriendlyRobberSetup{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
}
func selectCatanFriendlyRobber(c *testClient, enabled bool, status int) {
	c.post("/api/rooms/"+current(c)["id"].(string), map[string]any{"type": "catan_friendly_robber", "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: enabled}, "version": current(c)["version"], "nonce": randomID(12)}, status)
}
func TestCatanFriendlyRobberConfigurationHTTPPermissionsRestartAndHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("友善房主")
	guest.register("友善客人")
	// Two-player rivers with the friendly robber is supported now; allHelpers
	// without helpers stays invalid, which keeps the rejected-creation shape.
	host.post("/api/rooms", map[string]any{"kind": "catan", "name": "未接通的双人河流组合", "capacity": 2, "catanTwoScenario": "rivers", "catanOptions": game.CatanOptions{AllHelpers: true}, "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}}, 400)
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "友善设置", "capacity": 4}, 201)
	id := raw["id"].(string)
	if raw["catanFriendlyRobber"] != nil {
		t.Fatal("unfinished option exposed")
	}
	guest.command(current(host), "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	selectCatanFriendlyRobber(host, false, 200)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	provisionCatanFriendlyRobber(t, s, id)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("provision did not reset readiness")
		}
	}
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	selectCatanFriendlyRobber(guest, false, 400)
	for _, setup := range []any{nil, game.CatanFriendlyRobberSetup{Enabled: true, Rules: "wrong"}} {
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_friendly_robber", "catanFriendlyRobber": setup, "version": current(host)["version"], "nonce": randomID(12)}, 400)
	}
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid configuration changed room")
	}
	selectCatanFriendlyRobber(host, true, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same config reset readiness")
		}
	}
	stale := current(host)
	selectCatanFriendlyRobber(host, false, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("disable did not reset readiness")
		}
	}
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_friendly_robber", "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}, "version": stale["version"], "nonce": randomID(12)}, 409)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{Helpers: true}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	selectCatanFriendlyRobber(host, true, 200)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: true}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	selectCatanFriendlyRobber(host, true, 200)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	selectCatanFriendlyRobber(host, true, 200)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: true}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{}, "version": current(host)["version"], "nonce": randomID(12)}, 200)
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
	if r.Game.Catan.FriendlyRobber == nil || len(r.Game.Catan.Players) != 3 || r.Game.View(0)["catan"].(map[string]any)["victoryTarget"] != 10 {
		t.Fatal("formal start missing variant")
	}
	selectCatanFriendlyRobber(host, false, 400)
	r.CatanFriendlyRobber = &game.CatanFriendlyRobberSetup{Enabled: false, Rules: "stale"}
	host.command(current(host), "close", nil, 200)
	_, profile := host.request("GET", "/api/players/"+r.Host, nil)
	history := profile["history"].([]any)[0].(map[string]any)
	if history["catanExpansions"].([]any)[0] != "friendly_robber" || history["catanExpansionRules"].(map[string]any)["friendly_robber"] != game.CatanFriendlyRobberRules {
		t.Fatal("history used stale draft", history)
	}
}

func TestCatanFriendlyRobberConfigurationCombinationGates(t *testing.T) {
	for _, kind := range []string{"sea", "world", "fishing"} {
		t.Run(kind, func(t *testing.T) {
			r := &Room{Kind: "catan", Status: "waiting", Capacity: 3}
			switch kind {
			case "sea":
				r.CatanSeafarers = &game.CatanSeafarersSetup{Scenario: "unknown"}
			case "world":
				r.CatanNewWorldMap = &game.CatanNewWorldMap{}
			case "fishing":
				r.CatanFishing = true
			}
			before, _ := json.Marshal(r)
			if err := r.setCatanFriendlyRobber(game.CatanFriendlyRobberSetup{Enabled: true}); err == nil {
				t.Fatal("unverified combination enabled")
			}
			after, _ := json.Marshal(r)
			if string(before) != string(after) {
				t.Fatal("failed provision changed room")
			}
			if err := r.setCatanFriendlyRobber(game.CatanFriendlyRobberSetup{}); err != nil {
				t.Fatal("cannot retain disabled draft", err)
			}
		})
	}
	r := &Room{Kind: "catan", Status: "waiting", Capacity: 3}
	if err := r.setCatanFriendlyRobber(game.CatanFriendlyRobberSetup{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(r)
	if err := r.setCatanSeafarers(game.CatanSeafarersSetup{Scenario: "unknown"}); err == nil {
		t.Fatal("sea added after friendly")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("reverse combination rejection changed room")
	}
}

func TestCatanFriendlyRobberRejectsUnverifiedSavedDraftAtStart(t *testing.T) {
	for _, kind := range []string{"sea", "fishing", "version"} {
		t.Run(kind, func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h := newClient(t, ts.URL)
			h.register("组合验证")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "旧配置", "capacity": 3}, 201)
			id := raw["id"].(string)
			provisionCatanFriendlyRobber(t, s, id)
			for range 2 {
				h.command(current(h), "add_bot", nil, 200)
			}
			h.command(current(h), "ready", nil, 200)
			r := s.rooms[id]
			// Simulate an old or internally provisioned draft bypassing today's setter.
			switch kind {
			case "sea":
				r.CatanSeafarers = &game.CatanSeafarersSetup{Scenario: "unknown"}
			case "fishing":
				r.CatanFishing = true
			case "version":
				r.CatanFriendlyRobber.Rules = "future"
			}
			before, _ := json.Marshal(r)
			h.command(current(h), "start", nil, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("rejected start changed saved draft")
			}
		})
	}
}
