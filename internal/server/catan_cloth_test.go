package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func clothServerFixture(r *Room) {
	g := r.Game.Catan
	for i := range g.Edges {
		g.Edges[i].Owner = -1
		g.Edges[i].Ship = false
	}
	for i := range g.Vertices {
		g.Vertices[i].Owner = -1
		g.Vertices[i].Level = 0
	}
	g.Tiles[0].Resource = game.CatanSea
	g.Tiles[0].Number = 0
	g.Edges[0].Owner = 1
	g.Edges[0].Ship = true
	g.Edges[0].Tiles = []int{0}
	g.Seafarers = &game.CatanSeafarers{Pirate: -1, Scenario: "cloth", VictoryPoints: 14, Cloth: &game.CatanClothState{Stock: 10, Held: []int{1, 2, 0}, EmptyLimit: 5, Villages: []game.CatanClothVillage{{Vertex: g.Edges[0].A, Number: 6, Stock: 5, Traders: []int{0}}}}}
	g.SetupStep = 9
	g.ResumePhase = "catan_turn"
	r.Game.Turn = 0
	r.Game.Phase = "catan_robber"
}
func TestCatanClothHTTPChoicePrivacyClockAndRestart(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	s.mu.Lock()
	r := s.rooms[id]
	clothServerFixture(r)
	now := time.Now()
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if err := r.applyGameAction(0, game.Action{Type: "catan_pirate", Tile: 0}, now); err != nil {
		t.Fatal(err)
	}
	if r.Game.Phase != "catan_cloth_steal" || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("theft did not pause clock")
	}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_cloth_steal", "target": 1, "choice": "cloth"}, 400)
	clients[3].command(current(clients[3]), "action", map[string]any{"type": "catan_cloth_steal", "target": 1, "choice": "cloth"}, 400)
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_cloth_steal", "target": 1, "choice": "both"}, 400)
	if s.rooms[id].TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("invalid action reset deadline")
	}
	for i, c := range clients {
		view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		cloth := view["seafarers"].(map[string]any)["cloth"].(map[string]any)
		if cloth["held"].([]any)[1] != float64(2) {
			t.Fatal("cloth count not public")
		}
		for j, raw := range view["players"].([]any) {
			p := raw.(map[string]any)
			_, resources := p["resources"]
			_, dev := p["dev"]
			if resources != (i == j) || dev != (i == j) {
				t.Fatal("private hand exposed", i, j)
			}
		}
	}
	before, _ := json.Marshal(s.rooms[id])
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
		t.Fatal("theft state/deadline lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_cloth_steal", "target": 1, "choice": "cloth"}, 200)
	r = next.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.Seafarers.Cloth.Held[0] != 2 || r.Game.CatanPendingActor() != -1 {
		t.Fatal("HTTP choice failed")
	}
	left := r.TurnDeadline - time.Now().UnixMilli()
	if left < 44000 || left > 45000 {
		t.Fatal("choice did not restore 45 seconds", left)
	}
}
func TestCatanClothTheftTimeoutAndAutoplay(t *testing.T) {
	for _, autoplay := range []bool{false, true} {
		s, _, _, id := newCatanTable(t)
		s.mu.Lock()
		r := s.rooms[id]
		clothServerFixture(r)
		now := time.Now()
		r.TurnDeadline = now.Add(23 * time.Second).UnixMilli()
		if err := r.applyGameAction(0, game.Action{Type: "catan_pirate", Tile: 0}, now); err != nil {
			t.Fatal(err)
		}
		when := now.Add(turnLimit)
		if autoplay {
			r.Seats[0].AutoPlay = true
			r.BotAt = 0
			when = now.Add(time.Second)
			s.runBots(when)
		} else {
			s.expireSetups(when)
		}
		r = s.rooms[id]
		if r.Game.Phase != "catan_turn" || r.Game.Catan.Seafarers.Cloth.Held[0] != 2 || r.TurnDeadline != when.Add(23*time.Second).UnixMilli() {
			t.Fatal("automatic theft/clock failed", autoplay, r.Game.Phase)
		}
		s.mu.Unlock()
	}
}
func TestCatanClothThirdSetupPassTimeout(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	g := r.Game.Catan
	homes := []int{}
	for _, tile := range g.Tiles {
		homes = append(homes, tile.ID)
	}
	g.Seafarers = &game.CatanSeafarers{Pirate: -1, VictoryPoints: 14, Cloth: &game.CatanClothState{HomeTiles: homes, Held: make([]int, 3), EmptyLimit: 5}}
	g.StartPlayer = 0
	g.SetupStep = 6
	r.Game.Turn = 0
	r.Game.Phase = "catan_setup_settlement"
	now := time.Now()
	for step := 6; step < 9; step++ {
		r.TurnDeadline = now.Add(-time.Second).UnixMilli()
		s.expireSetups(now)
		r = s.rooms[id]
		if r.Game.Catan.SetupStep != step+1 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
			t.Fatal("third setup pass not automated", step, r.Game.Catan.SetupStep)
		}
		now = now.Add(time.Second)
	}
	if r.Game.Phase != "catan_roll" || r.Game.Turn != 0 {
		t.Fatal("third pass did not start first production")
	}
}

func TestCatanClothInitialRobberHTTPRestartAndFreshSetupClock(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	s.mu.Lock()
	r := s.rooms[id]
	clothServerFixture(r)
	g := r.Game.Catan
	g.Seafarers.Variable = true
	g.Seafarers.Cloth.HomeTiles = []int{1, 2}
	g.Tiles[1].Number = 12
	g.Tiles[2].Number = 12
	g.Robber = 1
	g.SetupStep = 0
	g.StartPlayer = 0
	r.Game.Phase = "catan_cloth_start"
	now := time.Now()
	r.TurnDeadline = now.Add(40 * time.Second).UnixMilli()
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_cloth_start", "tile": 2}, 400)
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_cloth_start", "tile": 0}, 400)
	before, _ := json.Marshal(s.rooms[id])
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
		t.Fatal("initial choice lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_cloth_start", "tile": 2}, 200)
	r = next.rooms[id]
	left := r.TurnDeadline - time.Now().UnixMilli()
	if r.Game.Phase != "catan_setup_settlement" || r.Game.Catan.Robber != 2 || r.Game.Catan.SetupStep != 0 || left < 119000 || left > 120000 {
		t.Fatal("first settlement did not get full clock", left)
	}
	// The same transition runs automatically when the initial choice expires.
	next.mu.Lock()
	defer next.mu.Unlock()
	r.Game.Phase = "catan_cloth_start"
	r.TurnDeadline = now.Add(-time.Second).UnixMilli()
	next.expireSetups(now)
	r = next.rooms[id]
	if r.Game.Phase != "catan_setup_settlement" || r.Game.Catan.SetupStep != 0 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("initial timeout skipped/reset wrong step")
	}
}
