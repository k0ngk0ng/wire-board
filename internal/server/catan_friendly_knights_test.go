package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanFriendlyKnightsPublicConfigurationRestoreAndReset(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, c := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("友善骑士房主")
			c.register("友善骑士朋友")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "友善骑士", "capacity": n, "catanScenario": "cities-knights", "catanOptions": game.CatanOptions{FiveSix: n > 4}, "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}, "catanHarbors": game.CatanHarborsSetup{Enabled: true}}, 201)
			id := raw["id"].(string)
			c.command(raw, "join", nil, 200)
			for i := 2; i < n; i++ {
				h.command(current(h), "add_bot", nil, 200)
			}
			ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
			ready()
			before, _ := json.Marshal(s.rooms[id])
			selectCatanFriendlyRobber(c, false, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("guest changed configuration")
			}
			selectCatanFriendlyRobber(h, true, 200)
			for _, seat := range s.rooms[id].Seats {
				if !seat.Ready {
					t.Fatal("same value clears readiness")
				}
			}
			selectCatanFriendlyRobber(h, false, 200)
			for _, seat := range s.rooms[id].Seats {
				if seat.Ready != seat.Bot {
					t.Fatal("changed value keeps readiness")
				}
			}
			selectCatanFriendlyRobber(h, true, 200)
			change := func(scene string) {
				h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": scene, "version": current(h)["version"], "nonce": randomID(12)}, 200)
			}
			change("new_world")
			selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 200)
			selectCatanCitiesKnights(h, nil, 200)
			selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 200)
			if !s.rooms[id].CatanFriendlyRobber.Enabled || !s.rooms[id].CatanHarbors.Enabled {
				t.Fatal("toggle loses variants")
			}
			ready()
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if g.FriendlyRobber.Knights != game.CatanFriendlyKnightsRules || g.FriendlyRobber.Fallback != game.CatanFriendlySeaFallbackRules || g.Seafarers.Scenario != "new_world" || g.CitiesKnights == nil || g.Harbors == nil {
				t.Fatal("combined start")
			}
			selectCatanFriendlyRobber(h, false, 400)
			h.command(current(h), "close", nil, 200)
			_, profile := h.request("GET", "/api/players/"+s.rooms[id].Host, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			if record["catanExpansionRules"].(map[string]any)["friendly_knights"] != game.CatanFriendlyKnightsRules {
				t.Fatal("history recipe")
			}
			h.command(current(h), "rematch", nil, 200)
			selectCatanFriendlyRobber(h, false, 200)
			selectCatanHarbors(h, false, 200)
			change("")
			ready()
			h.command(current(h), "start", nil, 200)
			g = s.rooms[id].Game.Catan
			if g.CitiesKnights != nil || g.FriendlyRobber != nil || g.Seafarers != nil || len(g.Bank) != 5 || g.Harbors != nil {
				t.Fatal("base game contaminated")
			}
		})
	}
}
