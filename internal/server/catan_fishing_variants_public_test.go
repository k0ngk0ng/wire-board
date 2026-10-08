package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanFishingVariantsPublicConfigurationRestoreAndBaseReset(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, scene := range []string{"fishing", "knights", "fog", "new_world"} {
			t.Run(fmt.Sprintf("%d/%s", n, scene), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				h, c := newClient(t, ts.URL), newClient(t, ts.URL)
				h.register("渔夫组合房主")
				c.register("渔夫组合朋友")
				scenario := scene
				if scene == "knights" {
					scenario = "fishing"
				}
				body := map[string]any{"kind": "catan", "name": "渔夫变体组合", "capacity": n, "catanScenario": scenario, "catanOptions": game.CatanOptions{FiveSix: n > 4}, "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}, "catanHarbors": game.CatanHarborsSetup{Enabled: true}}
				if publicCatanSeaScenario(scene) {
					body["catanFishing"] = true
				}
				if scene == "knights" {
					body["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
				}
				raw := h.post("/api/rooms", body, 201)
				id := raw["id"].(string)
				c.command(raw, "join", nil, 200)
				ready := func() {
					for i, client := range []*testClient{h, c} {
						if !s.rooms[id].Seats[i].Ready {
							client.command(current(client), "ready", nil, 200)
						}
					}
				}
				ready()
				before, _ := json.Marshal(s.rooms[id])
				selectCatanFriendlyRobber(c, false, 400)
				selectCatanHarbors(c, false, 400)
				h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: n > 4, AllHelpers: true}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("rejection changed configuration")
				}
				selectCatanFriendlyRobber(h, true, 200)
				selectCatanHarbors(h, true, 200)
				for _, seat := range s.rooms[id].Seats {
					if !seat.Ready {
						t.Fatal("same variant reset readiness")
					}
				}
				for _, friendly := range []bool{true, false} {
					if friendly {
						selectCatanFriendlyRobber(h, false, 200)
					} else {
						selectCatanHarbors(h, false, 200)
					}
					for _, seat := range s.rooms[id].Seats {
						if seat.Ready {
							t.Fatal("changed variant retained readiness")
						}
					}
					if friendly {
						selectCatanFriendlyRobber(h, true, 200)
					} else {
						selectCatanHarbors(h, true, 200)
					}
					ready()
				}
				if publicCatanSeaScenario(scene) {
					for _, enabled := range []bool{false, true} {
						h.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": enabled, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
						if !s.rooms[id].friendlyRobberEnabled() || !s.rooms[id].CatanHarbors.Enabled {
							t.Fatal("fish toggle lost variants")
						}
					}
				}
				for len(s.rooms[id].Seats) < n {
					h.command(current(h), "add_bot", nil, 200)
				}
				ready()
				s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
				h.command(current(h), "start", nil, 200)
				g := s.rooms[id].Game.Catan
				if g.Fishing == nil || g.FriendlyRobber == nil || g.FriendlyRobber.Fallback != game.CatanFriendlyFishingFallbackRules || g.Harbors == nil || len(g.Players) != n {
					t.Fatal("incorrect actual recipe")
				}
				h.command(current(h), "close", nil, 200)
				h.command(current(h), "rematch", nil, 200)
				if !s.rooms[id].friendlyRobberEnabled() || !s.rooms[id].CatanHarbors.Enabled || s.rooms[id].CatanScenario != scenario {
					t.Fatal("rematch lost combination")
				}
				selectCatanFriendlyRobber(h, false, 200)
				selectCatanHarbors(h, false, 200)
				h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
				ready()
				h.command(current(h), "start", nil, 200)
				g = s.rooms[id].Game.Catan
				if g.Fishing != nil || g.FriendlyRobber != nil || g.Harbors != nil || g.CitiesKnights != nil || g.Seafarers != nil || len(g.Bank) != 5 {
					t.Fatal("base retained variant")
				}
			})
		}
	}
}
