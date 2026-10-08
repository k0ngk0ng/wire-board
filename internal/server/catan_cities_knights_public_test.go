package server

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanCitiesKnightsPublicSelectionAndRematch(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("城市骑士公开房主")
	guest.register("城市骑士公开朋友")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2},
		{"kind": "catan", "capacity": 5},
		{"kind": "catan", "capacity": 4, "catanOptions": game.CatanOptions{Rules: "unknown", Helpers: true}},
		{"kind": "splendor", "capacity": 3},
	} {
		body["name"], body["catanScenario"] = "无效骑士", "cities-knights"
		host.post("/api/rooms", body, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"name": "城市骑士公开切换", "kind": "catan", "capacity": 4, "catanScenario": "cities-knights"}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	change := func(c *testClient, scenario string, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": scenario, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	for _, scenario := range []string{"shores", "cities-knights", "land-ho", "cities-knights", "", "cities-knights"} {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
		before, _ := json.Marshal(s.rooms[id])
		change(guest, scenario, 400)
		change(host, "unknown", 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("rejection changed state")
		}
		change(host, scenario, 200)
		r := s.rooms[id]
		if (r.CatanCitiesKnights != nil) != (scenario == "cities-knights") || (r.CatanSeafarers != nil) != (scenario == "shores") {
			t.Fatal("stale recipe", r)
		}
		for _, seat := range r.Seats {
			if seat.Ready != seat.Bot {
				t.Fatal("changed recipe retained human ready")
			}
		}
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
		change(host, scenario, 200)
		for _, seat := range s.rooms[id].Seats {
			if !seat.Ready {
				t.Fatal("same recipe cleared ready")
			}
		}
	}
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	if c := s.rooms[id].CatanCitiesKnights; c == nil || c.Layout != "variable" || c.Rules != game.CatanCitiesKnightsRules {
		t.Fatal("wrong persisted city recipe", c)
	}
	for _, options := range []game.CatanOptions{{Rules: "unknown", Helpers: true}, {Rules: "unknown", FiveSix: true, Helpers: true}} {
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": options, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
	}
	host.command(current(host), "start", nil, 200)
	r := s.rooms[id]
	g := r.Game.Catan
	if len(g.Players) != 3 || g.CitiesKnights == nil || g.CitiesKnights.Rules != game.CatanCitiesKnightsRules || len(g.Bank) != 8 || g.Bank[5] != 12 || len(g.DevDeck) != 0 || g.Paired != nil {
		t.Fatal("wrong public city start")
	}
	change(host, "", 400)
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if s.rooms[id].CatanCitiesKnights == nil || s.rooms[id].CatanScenario != "cities-knights" {
		t.Fatal("rematch lost recipe")
	}
	change(host, "", 200)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	if g := s.rooms[id].Game.Catan; g.CitiesKnights != nil || len(g.Bank) != 5 || len(g.DevDeck) != 25 {
		t.Fatal("ordinary restart retained city state")
	}
}

func TestCatanCitiesKnightsPublicRejectsMalformedWaitingState(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("骑士存档校验")
	raw := host.post("/api/rooms", map[string]any{"name": "骑士存档校验", "kind": "catan", "capacity": 3, "catanScenario": "cities-knights"}, 201)
	id := raw["id"].(string)
	for range 2 {
		host.command(current(host), "add_bot", nil, 200)
	}
	host.command(current(host), "ready", nil, 200)
	original, _ := json.Marshal(s.rooms[id])
	for _, change := range []func(*Room){
		func(r *Room) { r.CatanCitiesKnights = nil },
		func(r *Room) { r.CatanCitiesKnights.Layout = "fixed" },
		func(r *Room) { r.CatanCitiesKnights.Rules = game.CatanCitiesKnightsFiveSixRules },
		func(r *Room) { r.CatanSeafarers = &game.CatanSeafarersSetup{Scenario: "shores"} },
		func(r *Room) { r.CatanTwoRules = game.CatanTwoRules },
		func(r *Room) { r.CatanOptions.Rules = "unknown" },
	} {
		var corrupted Room
		if err := json.Unmarshal(original, &corrupted); err != nil {
			t.Fatal(err)
		}
		change(&corrupted)
		s.rooms[id] = &corrupted
		before, _ := json.Marshal(&corrupted)
		host.command(current(host), "start", nil, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("invalid city start mutated state")
		}
	}
}
