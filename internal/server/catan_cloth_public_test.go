package server

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// A declared end-game fixture after real public setup. Equal points and cloth
// must record both winners once, rather than force a single arbitrary winner.
func TestCatanClothPublicSharedExhaustionHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, 3)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("布匹共同胜者%d", p))
	}
	raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "布匹平局结算", "capacity": 3, "catanScenario": "cloth"}, 201)
	id := raw["id"].(string)
	for p := 1; p < 3; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for _, c := range clients {
		c.command(current(c), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	for step := 0; s.rooms[id].Game.Phase != "catan_turn" && step < 100; step++ {
		state := s.rooms[id].Game
		p := twoHTTPActor(state)
		a, err := state.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
	r := s.rooms[id]
	if r.Game.Phase != "catan_turn" {
		t.Fatal("setup did not complete")
	}
	c := r.Game.Catan.Seafarers.Cloth
	c.Held, c.Stock = []int{9, 9, 8}, 9
	for i := range c.Villages {
		c.Villages[i].Stock = 5
		if i < 5 {
			c.Villages[i].Stock = 0
		}
	}
	assertPublicClothSupply(t, r.Game.Catan)
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	p := s.rooms[id].Game.Turn
	request := map[string]any{"type": "action", "action": game.Action{Type: "catan_end"}, "version": current(clients[p])["version"], "nonce": randomID(12)}
	clients[p].post("/api/rooms/"+id, request, 200)
	if !reflect.DeepEqual(s.rooms[id].Game.Winners, []int{0, 1}) || s.rooms[id].Status != "finished" {
		t.Fatal("equal points and cloth did not share exhaustion result", s.rooms[id].Game.Winners)
	}
	assertPublicSeaVictory(t, s.rooms[id].Game, 0)
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	clients[p].post("/api/rooms/"+id, request, 200)
	for seat, c := range clients {
		code, profile := c.request("GET", "/api/players/"+s.rooms[id].Seats[seat].ID, nil)
		if code != 200 {
			t.Fatal(code)
		}
		stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
		wins := float64(1)
		if seat == 2 {
			wins = 0
		}
		if stats["played"] != float64(1) || stats["wins"] != wins || len(profile["history"].([]any)) != 1 {
			t.Fatal("shared result was lost or counted twice", seat, stats)
		}
	}
}
