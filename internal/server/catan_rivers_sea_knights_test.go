package server

import (
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanRiversSeaKnightsAdmission(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, scenario := range []string{"rivers-shores", "rivers-fog", "rivers-desert", "rivers-desert-belt", "rivers-tribe", "rivers-new-world"} {
			room := &Room{Kind: "catan", Status: "waiting", Capacity: n}
			if err := room.setCatanScenario(scenario); err != nil {
				t.Fatal(n, scenario, err)
			}
			setup, err := room.normalizeCatanCombinationKnights(game.CatanCitiesKnightsSetup{})
			if err != nil {
				t.Fatal(n, scenario, err)
			}
			if err = room.setPublicCatanCombinationKnights(&setup); err != nil {
				t.Fatal(n, scenario, err)
			}
			if err = room.validateCatanScenario(); err != nil {
				t.Fatal(n, scenario, err)
			}
			// Fishing stays outside this combination.
			room.CatanFishing = true
			if room.validateCatanScenario() == nil {
				t.Fatal(n, scenario, "fishing accepted")
			}
			room.CatanFishing = false
		}
	}
}

func TestCatanRiversSeaKnightsNaturalHTTP(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		n        int
		events   bool
	}{
		{"rivers-shores", 3, false},
		{"rivers-shores", 2, true},
		{"rivers-fog", 4, true},
		{"rivers-desert", 5, false},
		{"rivers-desert-belt", 6, true},
		{"rivers-tribe", 3, true},
		{"rivers-new-world", 6, false},
	} {
		t.Run(fmt.Sprintf("%s/%d/events%t", tc.scenario, tc.n, tc.events), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, tc.n+1)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("河流骑士%d", p))
			}
			recipe := map[string]any{"kind": "catan", "name": "河流海图骑士", "capacity": tc.n, "catanScenario": tc.scenario, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}}
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
					if view["rivers"] == nil || view["seafarers"] == nil || view["citiesKnights"] == nil {
						t.Fatal("missing public components")
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
