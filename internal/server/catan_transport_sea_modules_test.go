package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanTransportSeaModulesHTTP(t *testing.T) {
	for _, scene := range []string{"shores", "desert"} {
		for _, c := range []struct{ n, bits int }{{2, 1}, {3, 2}, {2, 3}, {6, 3}} {
			t.Run(fmt.Sprintf("%s/%d/%d", scene, c.n, c.bits), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, c.n+1)
				for p := range clients {
					clients[p] = newClient(t, ts.URL)
					clients[p].register(fmt.Sprintf("运输海图%d", p))
				}
				body := map[string]any{"kind": "catan", "name": "运输海图完整组合", "capacity": c.n, "catanScenario": "transport-" + scene, "catanTransportSea": game.CatanTransportSeaSetup{Scenario: scene, Layout: "variable"}, "catanOptions": game.CatanOptions{Helpers: true, AllHelpers: true}, "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}, "catanHarbors": game.CatanHarborsSetup{Enabled: true}, "catanEvents": game.CatanEventCatalogue, "catanFishing": c.bits&2 != 0}
				if c.bits&1 != 0 {
					body["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
				}
				r := clients[0].post("/api/rooms", body, 201)
				id := r["id"].(string)
				for p := 1; p < c.n; p++ {
					clients[p].command(current(clients[0]), "join", nil, 200)
				}
				for p := 0; p < c.n; p++ {
					clients[p].command(current(clients[0]), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				clients[c.n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				reload, timeout := false, false
				for step := 0; step < 16000 && !s.rooms[id].Game.Finished; step++ {
					room := s.rooms[id]
					g := room.Game
					p := twoHTTPActor(g)
					if g.Phase == "catan_transport_move" && !reload {
						deadline := room.TurnDeadline
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						if s.rooms[id].TurnDeadline != deadline {
							t.Fatal("restart changed movement clock")
						}
						s.mu.Lock()
						s.expireSetups(time.UnixMilli(deadline + 1))
						s.mu.Unlock()
						if !s.rooms[id].Seats[p].AutoPlay {
							t.Fatal("timeout did not take over")
						}
						setAutoPlay(clients[p], current(clients[p]), false, 200)
						reload, timeout = true, true
						continue
					}
					a, e := g.BotAction(p)
					if e != nil {
						t.Fatal(step, g.Phase, e)
					}
					clients[p].command(current(clients[p]), "action", a, 200)
				}
				room := s.rooms[id]
				if !room.Game.Finished || !reload || !timeout {
					t.Fatal("incomplete combo", room.Game.Round, room.Game.Phase)
				}
				_, profile := clients[c.n].request("GET", "/api/players/"+room.Seats[room.Game.Winners[0]].ID, nil)
				record := profile["history"].([]any)[0].(map[string]any)
				rules := record["catanExpansionRules"].(map[string]any)
				if record["catanLayout"] != game.CatanTransportSeaVariableLayout || rules["transport_sea_layout"] != game.CatanTransportSeaVariableLayout {
					t.Fatal("layout history lost")
				}
				if c.bits&1 != 0 && rules["transport_sea_knights"] != game.CatanTransportSeaKnightsRules {
					t.Fatal("knight history")
				}
				if c.bits&2 != 0 && rules["transport_sea_fishing"] != game.CatanTransportSeaFishingRules {
					t.Fatal("fish history")
				}
				t.Logf("finished round %d", room.Game.Round)
			})
		}
	}
}

func TestCatanTransportSeaModulesWaitingSwitches(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h := newClient(t, ts.URL)
	h.register("运输布局房主")
	r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "运输布局", "capacity": 2, "catanScenario": "transport-shores", "catanTransportSea": game.CatanTransportSeaSetup{Scenario: "shores", Layout: "variable"}, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "catanFishing": true, "catanOptions": game.CatanOptions{Helpers: true}, "catanEvents": game.CatanEventCatalogue}, 201)
	id := r["id"].(string)
	change := func(kind string, more map[string]any) {
		more["type"] = kind
		more["version"] = current(h)["version"]
		more["nonce"] = randomID(12)
		h.post("/api/rooms/"+id, more, 200)
	}
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_transport_sea", "version": current(h)["version"], "nonce": randomID(12), "catanTransportSea": game.CatanTransportSeaSetup{Scenario: "desert", Layout: "fixed"}}, 400)
	change("catan_scenario", map[string]any{"catanScenario": "transport-desert"})
	room := s.rooms[id]
	if room.CatanTransportSea == nil || room.CatanTransportSea.Scenario != "desert" || room.CatanTransportSea.Layout != "variable" || room.CatanCitiesKnights == nil || !room.CatanFishing || !room.CatanOptions.Helpers {
		t.Fatal("map switch lost compatible modules")
	}
	change("catan_transport_sea", map[string]any{"catanTransportSea": game.CatanTransportSeaSetup{Scenario: "desert", Layout: "fixed"}})
	if s.rooms[id].CatanTransportSea.Layout != "fixed" {
		t.Fatal("layout not changed")
	}
	change("catan_two_scenario", map[string]any{"catanTwoScenario": ""})
	if s.rooms[id].CatanTransportSea != nil || s.rooms[id].CatanScenario != "" {
		t.Fatal("transport setup leaked into base")
	}
	change("catan_scenario", map[string]any{"catanScenario": "transport-shores"})
	if s.rooms[id].CatanTransportSea == nil || s.rooms[id].CatanTransportSea.Layout != "fixed" {
		t.Fatal("transport defaults missing")
	}
}
