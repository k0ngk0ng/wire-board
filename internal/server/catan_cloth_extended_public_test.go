package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanClothExtendedPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("扩大布匹房主")
	guest.register("扩大布匹朋友")
	raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "六席布匹", "capacity": 6, "catanScenario": "cloth", "catanOptions": game.CatanOptions{FiveSix: true}}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	change := func(c *testClient, kind string, extra map[string]any, status int) {
		extra["type"] = kind
		extra["version"] = s.rooms[id].Version
		extra["nonce"] = randomID(12)
		c.post("/api/rooms/"+id, extra, status)
	}
	options := func(o game.CatanOptions, status int) {
		change(h, "catan_options", map[string]any{"catanOptions": o}, status)
	}
	before, _ := json.Marshal(s.rooms[id])
	change(guest, "catan_scenario", map[string]any{"catanScenario": ""}, 400)
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "cloth", Layout: "variable"}, 400)
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "fog"}, 400)
	selectCatanFriendlyRobber(h, true, 400)
	selectCatanHarbors(h, true, 400)
	change(h, "catan_fishing", map[string]any{"enabled": true}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid change mutated room")
	}
	choices := current(h)["catanSeafarersChoices"].([]any)
	if len(choices) != 1 || choices[0].(map[string]any)["id"] != "cloth" {
		t.Fatal("unsupported extended map advertised", choices)
	}
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "cloth"}, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same map cleared ready")
		}
	}
	options(game.CatanOptions{FiveSix: true, Helpers: true}, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready {
			t.Fatal("changed rules retained ready")
		}
	}
	selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 400)
	options(game.CatanOptions{FiveSix: true}, 200)
	selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 200)
	options(game.CatanOptions{}, 200)
	if r := s.rooms[id]; r.Capacity != 4 || r.CatanSeafarers.Layout != "fixed" || r.CatanCitiesKnights.Rules != game.CatanCitiesKnightsRules {
		t.Fatal("incorrect downgrade")
	}
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "cloth", Layout: "variable"}, 200)
	options(game.CatanOptions{FiveSix: true}, 200)
	if r := s.rooms[id]; r.Capacity != 5 || r.CatanSeafarers.Layout != "fixed" || r.CatanCitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules {
		t.Fatal("incorrect upgrade")
	}
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	h.command(current(h), "start", nil, 400)
	for range 3 {
		h.command(current(h), "add_bot", nil, 200)
	}
	options(game.CatanOptions{}, 400)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 5 || len(g.Seafarers.Cloth.Villages) != 12 || g.Seafarers.Cloth.Issued != 0 || g.CitiesKnights == nil || g.Paired == nil || g.SetupLimit() != 15 {
		t.Fatal("wrong actual-player opening")
	}
	assertPublicClothSupply(t, g)
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	if r := s.rooms[id]; r.CatanScenario != "cloth" || r.CatanCitiesKnights == nil || !r.CatanOptions.FiveSix {
		t.Fatal("rematch lost cloth recipe")
	}
	change(h, "catan_scenario", map[string]any{"catanScenario": ""}, 200)
	if r := s.rooms[id]; r.CatanSeafarers != nil || r.CatanCitiesKnights != nil || !r.CatanOptions.FiveSix {
		t.Fatal("base retained expansion state")
	}
}
