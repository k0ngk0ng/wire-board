package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanProgressScienceHTTPDiscountMetropolisAndRestart(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	state, err := game.NewCatanCitiesKnights(3, game.CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	g.SetupStep = 6
	g.TurnSerial = 1
	state.Turn, state.Phase = 0, "catan_turn"
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.CitiesKnights.Players[0].Improvements[0] = 3
	g.Players[0].Resources[5], g.Bank[5] = 3, 9
	cityProgressGive(t, g, 0, 1, 4)
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = state
	now := time.Now()
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	before, _ := json.Marshal(r)
	for _, p := range []int{1, 2, 3} {
		clients[p].command(current(clients[p]), "action", map[string]any{"type": "catan_progress", "card": 1, "color": 0}, 400)
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_improvement", "card": 1, "color": 0, "skill": "progress"}, 400)
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_progress", "card": 1, "color": 3}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid progress use mutated card, goods or clock")
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_progress", "card": 1, "color": 0}, 200)
	r = s.rooms[id]
	if r.Game.Phase != "catan_metropolis" || r.Game.Catan.Players[0].Resources[5] != 0 || len(r.Game.Catan.CitiesKnights.Players[0].Progress) != 1 || r.CatanTimeLeft < 44000 || r.CatanTimeLeft > 45000 {
		t.Fatal("Crane did not enter paid metropolis response")
	}
	for viewer, c := range clients {
		view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		if len(view["progressPlayable"].([]any)) != 0 {
			t.Fatal("can play a second card during metropolis response")
		}
		k := view["citiesKnights"].(map[string]any)
		if _, ok := k["progressDecks"]; ok {
			t.Fatal("progress deck exposed")
		}
		for p, raw := range k["players"].([]any) {
			_, ok := raw.(map[string]any)["progress"]
			if ok != (viewer == p) {
				t.Fatal("progress hand exposed")
			}
		}
	}
	before, _ = json.Marshal(r)
	savedTime := r.CatanTimeLeft
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	after, _ = json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("card payment/metropolis choice lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	next.mu.Lock()
	r = next.rooms[id]
	r.Seats[0].AutoPlay = true
	r.BotAt = 0
	botAt := time.Now()
	next.runBots(botAt)
	r = next.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.CitiesKnights.Metropolises[0] != 0 || r.Game.Catan.Players[0].Score != 4 || r.TurnDeadline != botAt.UnixMilli()+savedTime {
		t.Fatal("metropolis autoplay did not restore original clock")
	}
	next.mu.Unlock()
}

func TestCatanProgressScienceHTTPFreeRoadsPersistAndAutoplay(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	state, err := game.NewCatanCitiesKnights(3, game.CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g := state.Catan
	g.SetupStep = 6
	g.TurnSerial = 1
	state.Turn, state.Phase = 0, "catan_turn"
	g.Vertices = []game.CatanVertex{{ID: 0, Owner: 0, Level: 1}, {ID: 1, Owner: -1}, {ID: 2, Owner: -1}, {ID: 3, Owner: -1}}
	g.Edges = []game.CatanEdge{{ID: 0, A: 0, B: 1, Owner: -1}, {ID: 1, A: 1, B: 2, Owner: -1}, {ID: 2, A: 2, B: 3, Owner: -1}}
	g.Tiles = []game.CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0, 1, 2, 3}}}
	g.Ports = nil
	cityProgressGive(t, g, 0, 7)
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = state
	deadline := time.Now().Add(45 * time.Second).UnixMilli()
	r.TurnDeadline = deadline
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_progress", "card": 7}, 200)
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_road", "edge": 0}, 200)
	r = s.rooms[id]
	if r.Game.Phase != "catan_roads" || r.Game.Catan.FreeRoads != 1 || r.TurnDeadline != deadline {
		t.Fatal("free-road sequence reset player's ordinary clock")
	}
	before, _ := json.Marshal(r)
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	after, _ := json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("free-road continuation lost on restart")
	}
	next.mu.Lock()
	r = next.rooms[id]
	r.Seats[0].AutoPlay = true
	r.BotAt = 0
	next.runBots(time.Now())
	r = next.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.FreeRoads != 0 || r.Game.Catan.Edges[1].Owner != 0 || r.TurnDeadline != deadline {
		t.Fatal("autoplay failed to finish second free road on same clock")
	}
	next.mu.Unlock()
}
