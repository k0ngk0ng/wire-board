package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanExplorerKnightsPublicConfiguration(t *testing.T) {
	for _, capacity := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			host, guest, third := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
			host.register("组合房主")
			guest.register("组合朋友")
			third.register("第三位")
			setup := &game.CatanCitiesKnightsSetup{Layout: "variable"}
			raw := host.post("/api/rooms", map[string]any{"kind": "catan", "capacity": capacity, "name": "公开探索骑士", "catanScenario": "pirate-lairs", "catanCitiesKnights": setup}, 201)
			id := raw["id"].(string)
			guest.command(current(host), "join", nil, 200)
			selectKnights := func(c *testClient, v *game.CatanCitiesKnightsSetup, status int) {
				c.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": v, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			scenario := func(v string, status int) {
				host.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": v, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			ready := func() {
				for _, c := range []*testClient{host, guest} {
					c.command(current(c), "ready", nil, 200)
				}
			}
			ready()
			selectKnights(host, setup, 200)
			if !s.rooms[id].Seats[0].Ready || !s.rooms[id].Seats[1].Ready {
				t.Fatal("same option reset readiness")
			}
			before, _ := json.Marshal(s.rooms[id])
			selectKnights(guest, nil, 400)
			selectKnights(host, &game.CatanCitiesKnightsSetup{Layout: "fixed"}, 400)
			host.command(current(host), "start", nil, 400) // Capacity is not the actual count.
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid action changed room")
			}
			selectKnights(host, nil, 200)
			if s.rooms[id].Seats[0].Ready || s.rooms[id].Seats[1].Ready || s.rooms[id].CatanCitiesKnights != nil {
				t.Fatal("toggle did not reset")
			}
			selectKnights(host, setup, 200)
			for _, v := range []string{"fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
				scenario(v, 200)
				if s.rooms[id].CatanCitiesKnights == nil {
					t.Fatal("mission switch lost combination")
				}
			}
			if capacity <= 4 {
				scenario("land-ho", 200)
				if s.rooms[id].CatanCitiesKnights == nil {
					t.Fatal("initial scenario lost knights")
				}
				selectKnights(host, setup, 200)
				scenario("", 200)
				if s.rooms[id].CatanCitiesKnights != nil || s.rooms[id].CatanOptions != (game.CatanOptions{}) {
					t.Fatal("basic recipe polluted")
				}
				scenario("explorers-and-pirates", 200)
				selectKnights(host, setup, 200)
			}
			third.command(current(host), "join", nil, 200)
			clients := []*testClient{host, guest, third}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			for _, c := range clients {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if len(g.Players) != 3 || g.Paired != nil || g.CitiesKnights == nil || g.Explorer.Board.Target != 22 || g.CitiesKnightsSetup().Rules != game.CatanCitiesKnightsRules {
				t.Fatal("actual count/recipe mismatch")
			}
			selectKnights(host, nil, 400)
			host.command(current(host), "close", nil, 200)
			host.command(current(host), "rematch", nil, 200)
			if s.rooms[id].CatanCitiesKnights == nil || s.rooms[id].CatanScenario != "explorers-and-pirates" {
				t.Fatal("rematch lost recipe")
			}
			selectKnights(host, nil, 200)
			for _, c := range clients {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			g = s.rooms[id].Game.Catan
			if g.CitiesKnights != nil || g.Explorer.Board.CitiesKnights || g.Explorer.Board.Target != 17 {
				t.Fatal("disabled combination survived new game")
			}
		})
	}
}

func TestCatanExplorerKnightsPublicCreationRejectsMixes(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("组合限制")
	for _, change := range []map[string]any{
		{"capacity": 2}, {"capacity": 7}, {"catanScenario": "land-ho", "capacity": 2},
		{"catanOptions": game.CatanOptions{Helpers: true}}, {"catanOptions": game.CatanOptions{FiveSix: true}},
		{"catanFishingLakes": true},
	} {
		request := map[string]any{"kind": "catan", "capacity": 6, "name": "组合限制", "catanScenario": "explorers-and-pirates", "catanCitiesKnights": game.CatanCitiesKnightsSetup{Layout: "variable"}}
		for k, v := range change {
			request[k] = v
		}
		host.post("/api/rooms", request, 400)
	}
	if len(s.rooms) != 0 {
		t.Fatal("invalid creation left room")
	}
}
