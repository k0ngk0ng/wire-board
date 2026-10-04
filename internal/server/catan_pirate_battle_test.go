package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestCatanPirateKnightBattleHTTPPersistenceAndNextTurn(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	s.mu.Lock()
	r := s.rooms[id]
	g := r.Game.Catan
	for i := range g.Vertices {
		g.Vertices[i].Owner = -1
		g.Vertices[i].Level = 0
	}
	for i := range g.Edges {
		g.Edges[i].Owner = -1
		g.Edges[i].Ship = false
		g.Edges[i].Warship = false
	}
	// A small, persisted naval line isolates HTTP/clock/restart behavior; game
	// tests separately exercise all printed map paths and battle outcomes.
	root := 0
	route, vertices := []int{}, []int{root}
	var walk func(int) bool
	walk = func(v int) bool {
		if len(route) == 8 {
			return true
		}
		for _, e := range g.Edges {
			next := -1
			if e.A == v {
				next = e.B
			} else if e.B == v {
				next = e.A
			}
			if next < 0 || slices.Contains(vertices, next) {
				continue
			}
			route = append(route, e.ID)
			vertices = append(vertices, next)
			if walk(next) {
				return true
			}
			route = route[:len(route)-1]
			vertices = vertices[:len(vertices)-1]
		}
		return false
	}
	if !walk(root) {
		t.Fatal("failed to create HTTP battle fixture")
	}
	for i, id := range route {
		g.Edges[id].Owner = 0
		g.Edges[id].Ship = true
		g.Edges[id].Warship = i < len(route)-1
	}
	g.Vertices[root].Owner = 0
	g.Vertices[root].Level = 1
	fort := vertices[len(vertices)-1]
	g.Robber = -1
	g.SetupStep = g.SetupLimit()
	g.Players[0].Dev[0] = 1
	g.Players[0].NewDev[0] = 0
	g.PlayedDev = false
	g.Seafarers = &game.CatanSeafarers{Pirate: -1, Scenario: "pirate_islands", VictoryPoints: 10, PirateIslands: &game.CatanPirateIslands{Fortresses: []game.CatanPirateFortress{{Root: root, Route: route, Vertex: fort, Beachhead: vertices[4], Strength: 1}, {Root: 0, Strength: 3}, {Root: 0, Strength: 3}}}}
	r.Game.Turn = 0
	r.Game.Phase = "catan_turn"
	now := time.Now()
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	for _, i := range []int{1, 3} {
		clients[i].command(current(clients[i]), "action", map[string]any{"type": "catan_dev", "card": 0}, 400)
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_dev", "card": 0}, 200)
	r = s.rooms[id]
	if !r.Game.Catan.Edges[route[len(route)-1]].Warship || r.Game.Catan.Players[0].Knights != 0 || r.Game.Phase != "catan_turn" || r.TurnDeadline != now.Add(45*time.Second).UnixMilli() {
		t.Fatal("knight upgrade phase / clock")
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_end"}, 200)
	r = s.rooms[id]
	g = r.Game.Catan
	if r.Game.Turn != 1 || r.Game.Phase != "catan_roll" || g.Seafarers.PirateIslands.Fortresses[0].Strength != 0 || g.Vertices[fort].Owner != 0 || g.Vertices[fort].Level != 1 || g.Seafarers.PirateIslands.Battle.ID != 1 {
		t.Fatal("battle did not capture and hand off")
	}
	remaining := r.TurnDeadline - time.Now().UnixMilli()
	if remaining < 119000 || remaining > 120000 {
		t.Fatal("next player missing fresh turn", remaining)
	}
	view := current(clients[3])["game"].(map[string]any)["catan"].(map[string]any)
	for _, raw := range view["players"].([]any) {
		p := raw.(map[string]any)
		if _, ok := p["dev"]; ok {
			t.Fatal("spectator private cards")
		}
	}
	battle := view["seafarers"].(map[string]any)["pirateIslands"].(map[string]any)["battle"].(map[string]any)
	if battle["warships"] != float64(8) || battle["remaining"] != float64(0) {
		t.Fatal("public battle missing")
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
		t.Fatal("naval path / battle / captured settlement lost after restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_end"}, 400)
	if next.rooms[id].Game.Catan.Seafarers.PirateIslands.Battle.ID != 1 {
		t.Fatal("replayed finished battle")
	}
}
