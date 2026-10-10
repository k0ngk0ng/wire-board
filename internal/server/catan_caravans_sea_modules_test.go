package server

import (
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanCaravansSeaModulesAdmission(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, scenario := range []string{"caravans-shores", "caravans-islands", "caravans-desert", "caravans-tribe", "caravans-new-world"} {
			for _, knights := range []bool{false, true} {
				for _, fishing := range []bool{false, true} {
					room := &Room{Kind: "catan", Status: "waiting", Capacity: n}
					if err := room.setCatanScenario(scenario); err != nil {
						t.Fatal(n, scenario, err)
					}
					if fishing {
						if err := room.setCatanFishing(true); err != nil {
							t.Fatal(n, scenario, err)
						}
					}
					if knights {
						setup, err := room.normalizeCatanCombinationKnights(game.CatanCitiesKnightsSetup{})
						if err != nil {
							t.Fatal(n, scenario, err)
						}
						if err = room.setPublicCatanCombinationKnights(&setup); err != nil {
							t.Fatal(n, scenario, err)
						}
					}
					if err := room.validateCatanScenario(); err != nil {
						t.Fatal(n, scenario, knights, fishing, err)
					}
				}
			}
		}
	}
}

func TestCatanCaravansSeaModulesNaturalHTTP(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		n        int
		knights  bool
		fishing  bool
		events   bool
	}{
		{"caravans-shores", 3, true, false, false},
		{"caravans-shores", 3, false, true, true},
		{"caravans-desert", 5, true, true, true},
		{"caravans-tribe", 2, true, false, false},
		{"caravans-new-world", 6, false, true, false},
	} {
		t.Run(fmt.Sprintf("%s/%d/knights%t/fishing%t", tc.scenario, tc.n, tc.knights, tc.fishing), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, tc.n+1)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("商队海图%d", p))
			}
			recipe := map[string]any{"kind": "catan", "name": "商队海图叠加", "capacity": tc.n, "catanScenario": tc.scenario}
			if tc.knights {
				recipe["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
			}
			if tc.fishing {
				recipe["catanFishing"] = true
			}
			if tc.events {
				recipe["catanEvents"] = game.CatanEventCatalogue
			}
			raw := clients[0].post("/api/rooms", recipe, 201)
			id := raw["id"].(string)
			for p := 1; p < tc.n; p++ {
				clients[p].command(current(clients[0]), "join", nil, 200)
			}
			for p := 0; p < tc.n; p++ {
				clients[p].command(current(clients[p]), "ready", nil, 200)
			}
			clients[0].command(current(clients[0]), "start", nil, 200)
			ordered := make([]*testClient, tc.n+1)
			for _, c := range clients[:tc.n] {
				ordered[int(current(c)["you"].(float64))] = c
			}
			ordered[tc.n] = clients[tc.n]
			clients = ordered
			clients[tc.n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			restored := false
			for step := 0; step < 24000 && !s.rooms[id].Game.Finished; step++ {
				g := s.rooms[id].Game
				p := twoHTTPActor(g)
				a, err := g.BotAction(p)
				if err != nil {
					t.Fatal(step, g.Phase, err)
				}
				clients[p].command(current(clients[p]), "action", a, 200)
				if step == 83 {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored = true
				}
				if step%131 == 0 {
					view := current(clients[tc.n])["game"].(map[string]any)["catan"].(map[string]any)
					if view["seafarers"] == nil || view["caravans"] == nil {
						t.Fatal("missing public components")
					}
					if tc.knights && view["citiesKnights"] == nil {
						t.Fatal("missing knight view")
					}
					if tc.fishing && view["fishing"] == nil {
						t.Fatal("missing fishing view")
					}
				}
			}
			if !s.rooms[id].Game.Finished || !restored {
				t.Fatal("incomplete", s.rooms[id].Game.Round)
			}
			t.Log("round", s.rooms[id].Game.Round, "winners", s.rooms[id].Game.Winners)
		})
	}
}
