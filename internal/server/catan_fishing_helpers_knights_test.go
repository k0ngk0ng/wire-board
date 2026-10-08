package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanFishingHelpersKnightsPublicConfiguration(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, scene := range []string{"fishing", "shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
			t.Run(fmt.Sprintf("%s/%d", scene, n), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
				h.register("渔夫骑士助手房主")
				guest.register("渔夫骑士助手朋友")
				body := map[string]any{"kind": "catan", "name": "渔夫骑士助手", "capacity": n, "catanScenario": scene, "catanFishing": scene != "fishing", "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: true}, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "catanEvents": game.CatanEventCatalogue, "catanHarbors": game.CatanHarborsSetup{Enabled: true}, "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}}
				raw := h.post("/api/rooms", body, 201)
				id := raw["id"].(string)
				guest.command(raw, "join", nil, 200)
				change := func(c *testClient, typ, key string, value any, status int) {
					c.post("/api/rooms/"+id, map[string]any{"type": typ, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
				}
				before, _ := json.Marshal(s.rooms[id])
				change(guest, "catan_options", "catanOptions", game.CatanOptions{FiveSix: n > 4}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("guest changed recipe")
				}
				for _, on := range []bool{false, true} {
					change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: n > 4, Helpers: on, AllHelpers: on}, 200)
					if on {
						change(h, "catan_cities_knights", "catanCitiesKnights", nil, 200)
						change(h, "catan_cities_knights", "catanCitiesKnights", game.CatanCitiesKnightsSetup{}, 200)
					}
				}
				s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
				for len(s.rooms[id].Seats) < n {
					h.command(current(h), "add_bot", nil, 200)
				}
				for _, c := range []*testClient{h, guest} {
					c.command(current(c), "ready", nil, 200)
				}
				h.command(current(h), "start", nil, 200)
				g := s.rooms[id].Game.Catan
				if g.Fishing.Helpers != game.CatanFishingHelpersRules || g.CitiesKnights.Helpers.Rules != game.CatanHelpersKnightsRules || g.Harbors == nil || g.FriendlyRobber == nil {
					t.Fatal("missing combination")
				}
				h.command(current(h), "close", nil, 200)
				h.command(current(h), "rematch", nil, 200)
				change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: n > 4}, 200)
				for _, c := range []*testClient{h, guest} {
					c.command(current(c), "ready", nil, 200)
				}
				h.command(current(h), "start", nil, 200)
				g = s.rooms[id].Game.Catan
				if g.Fishing.Helpers != "" || g.CitiesKnights.Helpers != nil || g.Options.Helpers {
					t.Fatal("disabled helpers persisted")
				}
			})
		}
	}
}
func TestCatanFishingHelpersKnightsNaturalHTTPGames(t *testing.T) {
	for _, tc := range []struct {
		n      int
		scene  string
		events bool
	}{{3, "fishing", false}, {6, "fishing", true}, {3, "shores", true}, {4, "islands", false}, {5, "fog", true}, {6, "desert", false}, {3, "tribe", true}, {5, "cloth", false}, {3, "wonders", true}, {4, "new_world", false}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			s, ts, clients, id := newHelpersKnightsHTTP(t, tc.n, tc.scene, tc.events, true)
			restored, responses := false, 0
			for step := 0; step < 16000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				p := twoHTTPActor(r.Game)
				a, e := r.Game.BotAction(p)
				if e != nil {
					snapshot, _ := json.Marshal(r.Game)
					var trial game.State
					json.Unmarshal(snapshot, &trial)
					endErr := trial.Apply(p, game.Action{Type: "catan_end"})
					t.Fatalf("step %d phase %s bot %v end %v state %s", step, r.Game.Phase, e, endErr, snapshot)
				}
				if a.Type == "catan_helper_choice" {
					responses++
				}
				clients[p].command(current(clients[p]), "action", a, 200)
				if !restored && responses > 0 {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored = true
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || !restored {
				t.Fatal("incomplete combined match", r.Game.Round, responses)
			}
			_, profile := clients[tc.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			rules := record["catanExpansionRules"].(map[string]any)
			if rules["helpers_knights"] != game.CatanHelpersKnightsRules || rules["fishing_helpers"] != game.CatanFishingHelpersRules {
				t.Fatal("history lost combination")
			}
			t.Log("completed", r.Game.Round, "rounds", responses, "helper responses")
		})
	}
}
