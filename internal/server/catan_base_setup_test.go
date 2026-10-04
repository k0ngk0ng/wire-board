package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

func provisionCatanBase(t *testing.T, s *Server, id, layout string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rooms[id].setCatanBaseConfiguration(game.CatanBaseConfiguration{Layout: layout}); err != nil {
		t.Fatal(err)
	}
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
}
func selectCatanBase(c *testClient, setup *game.CatanBaseConfiguration, status int) {
	r := current(c)
	c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "catan_base_configuration", "catanBaseConfiguration": setup, "version": r["version"], "nonce": randomID(12)}, status)
}
func TestCatanBaseConfigurationHTTPPermissionsReadinessRestartAndHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("布局房主")
	guest.register("布局客人")
	options := game.CatanOptions{FiveSix: true, Helpers: true}
	host.post("/api/rooms", map[string]any{"kind": "catan", "name": "未开放", "capacity": 6, "catanOptions": options, "catanBaseConfiguration": game.CatanBaseConfiguration{Layout: "fixed"}}, 400)
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "基础地图设置", "capacity": 6, "catanOptions": options}, 201)
	id := raw["id"].(string)
	if _, exposed := raw["catanBaseLayouts"]; exposed || raw["catanBaseConfiguration"] != nil {
		t.Fatal("unfinished choices exposed")
	}
	guest.command(current(host), "join", nil, 200)
	for i := 0; i < 3; i++ {
		host.command(current(host), "add_bot", nil, 200)
	}
	fixed := game.CatanBaseConfiguration{Layout: "fixed"}
	selectCatanBase(host, &fixed, 400)
	provisionCatanBase(t, s, id, "variable")
	if len(current(host)["catanBaseLayouts"].([]any)) != 2 {
		t.Fatal("missing choices")
	}
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	selectCatanBase(guest, &fixed, 400)
	selectCatanBase(host, nil, 400)
	selectCatanBase(host, &game.CatanBaseConfiguration{Layout: "unknown"}, 400)
	selectCatanBase(host, &game.CatanBaseConfiguration{Layout: "fixed", Rules: "catan-base-2025"}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejected config mutated room")
	}
	selectCatanBase(host, &game.CatanBaseConfiguration{Layout: "variable"}, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same layout reset ready")
		}
	}
	stale := current(host)
	selectCatanBase(host, &fixed, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("change must reset humans only")
		}
	}
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_base_configuration", "catanBaseConfiguration": fixed, "version": stale["version"], "nonce": randomID(12)}, 409)
	host.command(current(host), "start", nil, 400)
	host.command(current(host), "ready", nil, 200)
	before, _ = json.Marshal(s.rooms[id])
	ts.Close()
	s.Close()
	next, e := New(s.cfg, s.files)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	stopBotTicker(next)
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	host.base, guest.base = ts2.URL, ts2.URL
	after, _ = json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("waiting layout/readiness lost")
	}
	guest.command(current(guest), "ready", nil, 200)
	host.command(current(host), "start", nil, 200)
	r := next.rooms[id]
	if r.Game.Phase != "catan_roll" || r.Game.Catan.BaseSetup.Layout != "fixed" || len(r.Game.Catan.Players) != 5 || r.Game.Catan.BaseSetup.NeutralColor < 0 {
		t.Fatal("did not start actual five-player fixed layout")
	}
	for _, p := range r.Game.Catan.Players {
		if p.Score != 2 || p.Helper == nil {
			t.Fatal("preset score/helper")
		}
	}
	if left := r.TurnDeadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
		t.Fatal("first roll clock", left)
	}
	selectCatanBase(host, &fixed, 400)
	if _, exposed := current(host)["catanBaseLayouts"]; exposed {
		t.Fatal("running picker exposed")
	}
	// History must use frozen game metadata, not a stale waiting selection.
	r.CatanBaseConfiguration = &game.CatanBaseConfiguration{Layout: "variable", Rules: "stale"}
	host.command(current(host), "close", nil, 200)
	_, profile := host.request("GET", "/api/players/"+r.Host, nil)
	match := profile["history"].([]any)[0].(map[string]any)
	if match["catanLayout"] != "fixed" || match["catanRules"] != "catan-base-5-6-2025" || match["catanScenario"] != nil {
		t.Fatal("wrong base history identity", match)
	}
}
func TestCatanBaseConfigurationPlayerChangesAndScenarioExclusion(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("基础人数切换")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "基础人数", "capacity": 6, "catanOptions": game.CatanOptions{FiveSix: true}}, 201)
	id := raw["id"].(string)
	provisionCatanBase(t, s, id, "fixed")
	change := func(six bool) {
		r := current(host)
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: six, Helpers: true}, "version": r["version"], "nonce": randomID(12)}, 200)
	}
	change(false)
	if r := s.rooms[id]; r.Capacity != 4 || r.CatanBaseConfiguration.Layout != "variable" || r.CatanBaseConfiguration.Rules != "catan-base-2025" {
		t.Fatal("invalid smaller recipe")
	}
	selectCatanBase(host, &game.CatanBaseConfiguration{Layout: "fixed"}, 400)
	change(true)
	if r := s.rooms[id]; r.Capacity != 5 || r.CatanBaseConfiguration.Layout != "variable" || r.CatanBaseConfiguration.Rules != "catan-base-5-6-2025" {
		t.Fatal("invalid larger recipe")
	}
	selectCatanBase(host, &game.CatanBaseConfiguration{Layout: "fixed"}, 200)
	if e := s.rooms[id].setCatanSeafarers(game.CatanSeafarersSetup{Scenario: "fog"}); e == nil {
		t.Fatal("mixed base/sea configuration")
	}
	for i := 1; i < 5; i++ {
		host.command(current(host), "add_bot", nil, 200)
	}
	host.command(current(host), "ready", nil, 200)
	s.rooms[id].CatanSeafarers = &game.CatanSeafarersSetup{Scenario: "fog"}
	before, _ := json.Marshal(s.rooms[id])
	host.command(current(host), "start", nil, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("mixed start mutated state")
	}
	s.rooms[id].CatanBaseConfiguration = nil
	if e := s.rooms[id].setCatanBaseConfiguration(game.CatanBaseConfiguration{Layout: "fixed"}); e == nil {
		t.Fatal("base overwrote seafarers")
	}
}
