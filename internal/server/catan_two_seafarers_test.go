package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanTwoSeafarersPublicConfiguration(t *testing.T) {
	runTwoSeafarersPublicConfiguration(t, false)
}

func TestCatanTwoFishingSeafarersPublicConfiguration(t *testing.T) {
	runTwoSeafarersPublicConfiguration(t, true)
}

func TestCatanTwoSeafarersKnightsPublicConfiguration(t *testing.T) {
	for _, fish := range []bool{false, true} {
		t.Run(fmt.Sprint(fish), func(t *testing.T) { runTwoSeafarersPublicConfiguration(t, fish, true) })
	}
}
func runTwoSeafarersPublicConfiguration(t *testing.T, fishing bool, knightOption ...bool) {
	knights := len(knightOption) > 0 && knightOption[0]
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, g := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("双人航海房主")
	g.register("双人航海朋友")
	recipe := map[string]any{"kind": "catan", "capacity": 2, "name": "双人航海", "catanFishing": fishing, "catanTwoScenario": "shores", "catanOptions": game.CatanOptions{Helpers: !knights}, "catanEvents": game.CatanEventCatalogue, "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}, "catanHarbors": game.CatanHarborsSetup{Enabled: true}}
	if knights {
		recipe["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
	}
	raw := h.post("/api/rooms", recipe, 201)
	id := raw["id"].(string)
	g.command(current(h), "join", nil, 200)
	change := func(c *testClient, scenario string, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": scenario, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	for _, scenario := range []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		before, _ := json.Marshal(s.rooms[id])
		change(g, scenario, 400)
		change(h, "pirate_islands", 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("invalid config mutation")
		}
		change(h, scenario, 200)
		r := s.rooms[id]
		if r.CatanSeafarers == nil || r.CatanSeafarers.Scenario != scenario || len(current(h)["catanSeafarersChoices"].([]any)) != 8 {
			t.Fatal("missing map choices")
		}
		if scenario != "new_world" {
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_seafarers", "catanSeafarers": game.CatanSeafarersSetup{Scenario: scenario, Layout: "variable"}, "version": r.Version, "nonce": randomID(12)}, func() int {
				if fishing && (scenario == "desert" || scenario == "tribe") {
					return 400
				}
				return 200
			}())
		}
		if knights {
			toggle := func(c *testClient, enabled bool, status int) {
				var setup *game.CatanCitiesKnightsSetup
				if enabled {
					setup = &game.CatanCitiesKnightsSetup{}
				}
				c.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": setup, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			toggle(g, false, 400)
			toggle(h, false, 200)
			if s.rooms[id].CatanCitiesKnights != nil {
				t.Fatal("toggle did not clear")
			}
			toggle(h, true, 200)
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{Helpers: true, FiveSix: true}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
		}
		h.command(current(h), "ready", nil, 200)
		g.command(current(g), "ready", nil, 200)
		change(h, scenario, 200)
		if !s.rooms[id].Seats[0].Ready || !s.rooms[id].Seats[1].Ready {
			t.Fatal("same config reset readiness")
		}
		s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, g}, id)
		h.command(current(h), "start", nil, 200)
		if knights {
			view := s.rooms[id].Game.View(0)["catan"].(map[string]any)["two"].(map[string]any)
			if view["seaKnights"] != game.CatanTwoSeafarersKnightsRules {
				t.Fatal("missing knights marker", view)
			}
		}
		if fishing {
			view := s.rooms[id].Game.View(0)["catan"].(map[string]any)["fishing"].(map[string]any)
			if view["twoSea"] != game.CatanTwoFishingSeafarersRules {
				t.Fatal("missing public rule marker", view)
			}
		}
		if s.rooms[id].Game.Catan.Two.Seafarers != game.CatanTwoSeafarersRules {
			t.Fatal("wrong constructor")
		}
		h.command(current(h), "close", nil, 200)
		h.command(current(h), "rematch", nil, 200)
		id = current(h)["id"].(string)
	}
	change(h, "", 200)
	if s.rooms[id].CatanCitiesKnights != nil || s.rooms[id].CatanFishing || s.rooms[id].CatanSeafarers != nil || s.rooms[id].CatanNewWorldMap != nil {
		t.Fatal("base contaminated")
	}
	h.command(current(h), "ready", nil, 200)
	g.command(current(g), "ready", nil, 200)
	h.command(current(h), "start", nil, 200)
	if s.rooms[id].Game.Catan.CitiesKnights != nil || s.rooms[id].Game.Catan.Fishing != nil || s.rooms[id].Game.Catan.Seafarers != nil || s.rooms[id].Game.Catan.Two.Seafarers != "" {
		t.Fatal("base constructor contaminated")
	}
}
