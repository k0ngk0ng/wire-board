package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanAttackShoresHTTP(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, 4)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("蛮族航海%d", i))
	}
	h := clients[0]
	r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "蛮族新海岸", "capacity": 4, "catanScenario": "attack-shores", "catanEvents": game.CatanEventCatalogue}, 201)
	id := r["id"].(string)
	for _, c := range clients[1:] {
		c.command(current(h), "join", nil, 200)
	}
	for _, c := range clients {
		c.command(current(c), "ready", nil, 200)
	}
	h.command(current(h), "start", nil, 200)
	ordered := make([]*testClient, 4)
	for _, c := range clients {
		ordered[int(current(c)["you"].(float64))] = c
	}
	clients = ordered
	for step := 0; step < 14000 && !s.rooms[id].Game.Finished; step++ {
		g := s.rooms[id].Game
		p := twoHTTPActor(g)
		a, e := g.BotAction(p)
		if e != nil {
			t.Fatal(e)
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
	if record["catanScenario"] != "attack-shores" || record["catanRules"] != game.CatanAttackSeafarersRules {
		t.Fatal("history")
	}
}
