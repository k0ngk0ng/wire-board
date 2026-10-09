package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanCaravansSeaPublicHTTP(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, n)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("航海商队%d", p))
			}
			h := clients[0]
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "商队沙漠", "capacity": n, "catanScenario": "caravans-desert"}, 201)
			id := raw["id"].(string)
			for p := 1; p < n; p++ {
				clients[p].command(current(h), "join", nil, 200)
			}
			for _, c := range clients {
				c.command(current(c), "ready", nil, 200)
			}
			h.command(current(h), "start", nil, 200)
			ordered := make([]*testClient, n)
			for _, c := range clients {
				ordered[int(current(c)["you"].(float64))] = c
			}
			clients = ordered
			for step := 0; step < 18000 && !s.rooms[id].Game.Finished; step++ {
				g := s.rooms[id].Game
				p := twoHTTPActor(g)
				a, err := g.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				clients[p].command(current(clients[p]), "action", a, 200)
				if step == 83 {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
				}
			}
			if !s.rooms[id].Game.Finished {
				t.Fatal("unfinished")
			}
			_, profile := h.request("GET", "/api/players/"+s.rooms[id].Host, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			if record["catanScenario"] != "caravans-desert" || record["catanRules"] != game.CatanCaravansSeafarersRules {
				t.Fatal("history")
			}
			h.command(current(h), "rematch", nil, 200)
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			if s.rooms[id].CatanScenario != "" {
				t.Fatal("base switch")
			}
		})
	}
}

func TestCatanCaravansSeaRejectsUnsupported(t *testing.T) {
	for _, n := range []int{2, 5, 6} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
		if r.setCatanScenario("caravans-desert") == nil {
			t.Fatal("unsupported count", n)
		}
	}
	for _, mutate := range []func(*Room){func(r *Room) { r.CatanFishing = true }, func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} }, func(r *Room) { r.CatanOptions.Helpers = true }, func(r *Room) { r.CatanEvents = game.CatanEventCatalogue }} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: 3, CatanScenario: "caravans-desert"}
		mutate(r)
		if r.validateCatanScenario() == nil {
			t.Fatal("unsupported mix")
		}
	}
}
