package server

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanKnightsExtendedPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("六人骑士房主")
	guest.register("六人骑士朋友")
	raw := h.post("/api/rooms", map[string]any{
		"kind": "catan", "name": "城市骑士六席", "capacity": 6, "catanScenario": "cities-knights",
		"catanOptions": game.CatanOptions{FiveSix: true}, "catanHarbors": game.CatanHarborsSetup{Enabled: true},
	}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	change := func(c *testClient, scene string, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": scene, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	options := func(o game.CatanOptions, status int) {
		h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": o, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	before, _ := json.Marshal(s.rooms[id])
	change(guest, "", 400)
	change(h, "unknown", 400)
	options(game.CatanOptions{FiveSix: true, Helpers: true}, 400)
	selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{Rules: game.CatanCitiesKnightsRules}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejected configuration changed room")
	}
	change(h, "cities-knights", 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same scenario cleared ready")
		}
	}
	change(h, "", 200)
	if r := s.rooms[id]; r.CatanCitiesKnights != nil || !r.CatanHarbors.Enabled || r.Capacity != 6 {
		t.Fatal("returning to base retained knights or lost variant/count")
	}
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready {
			t.Fatal("changed scenario kept ready")
		}
	}
	selectCatanBase(h, &game.CatanBaseConfiguration{Layout: "fixed"}, 200)
	change(h, "cities-knights", 200)
	if r := s.rooms[id]; r.CatanBaseConfiguration != nil || r.CatanCitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules {
		t.Fatal("base layout leaked into city recipe")
	}
	options(game.CatanOptions{}, 200)
	if r := s.rooms[id]; r.Capacity != 4 || r.CatanCitiesKnights.Rules != game.CatanCitiesKnightsRules || !r.CatanHarbors.Enabled {
		t.Fatal("incorrect four-player downgrade")
	}
	options(game.CatanOptions{FiveSix: true}, 200)
	if r := s.rooms[id]; r.Capacity != 5 || r.CatanCitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules {
		t.Fatal("incorrect five-player upgrade")
	}
	for range 3 {
		h.command(current(h), "add_bot", nil, 200)
	}
	options(game.CatanOptions{}, 400)
	h.command(current(h), "start", nil, 400)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 5 || len(g.Tiles) != 30 || len(g.Bank) != 8 || g.Bank[5] != 18 || len(g.DevDeck) != 0 || g.Paired == nil || g.Harbors == nil || g.CitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules {
		t.Fatal("incorrect actual five-player city opening")
	}
	change(h, "", 400)
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	if r := s.rooms[id]; r.CatanScenario != "cities-knights" || r.CatanCitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules || !r.CatanHarbors.Enabled {
		t.Fatal("rematch lost recipe")
	}
}
