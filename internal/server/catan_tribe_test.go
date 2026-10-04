package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func tribeServerFixture(t *testing.T, r *Room) (int, int) {
	t.Helper()
	g := r.Game.Catan
	first, second := g.Ports[0].Edge, g.Ports[4].Edge
	for i := range g.Vertices {
		g.Vertices[i].Owner = -1
		g.Vertices[i].Level = 0
	}
	for i := range g.Edges {
		g.Edges[i].Owner = -1
		g.Edges[i].Ship = false
	}
	for _, edge := range []int{first, second} {
		v := g.Edges[edge].A
		g.Vertices[v].Owner = 0
		g.Vertices[v].Level = 1
	}
	g.Ports = nil
	g.Seafarers = &game.CatanSeafarers{Scenario: "tribe", VictoryPoints: 13, Pirate: -1, Tribe: &game.CatanTribeState{Points: make([]int, 3), HeldPorts: make([][]int, 3)}}
	g.Seafarers.Tribe.Ports = []game.CatanPort{{Edge: first, Resource: 0}}
	g.Seafarers.Tribe.Development = []game.CatanTribeDevelopment{{Edge: second, Card: g.DevDeck[len(g.DevDeck)-1]}}
	g.DevDeck = g.DevDeck[:len(g.DevDeck)-1]
	for _, color := range []int{0, 2} {
		g.Bank[color] -= 2 - g.Players[0].Resources[color]
		g.Players[0].Resources[color] = 2
	}
	r.Game.Turn = 0
	r.Game.Phase = "catan_turn"
	g.ResumePhase = "catan_turn"
	return first, second
}
func TestCatanTribeHTTPPrivacyClockAndRestart(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	s.mu.Lock()
	r := s.rooms[id]
	edge, _ := tribeServerFixture(t, r)
	now := time.Now()
	r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
	if err := r.applyGameAction(0, game.Action{Type: "catan_ship", Edge: edge}, now); err != nil {
		t.Fatal(err)
	}
	if r.Game.Phase != "catan_port" || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("port response did not pause clock")
	}
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_port", "edge": edge}, 400)
	clients[3].command(current(clients[3]), "action", map[string]any{"type": "catan_port", "edge": edge}, 400)
	for i, c := range clients {
		view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		tribe := view["seafarers"].(map[string]any)["tribe"].(map[string]any)
		for _, card := range tribe["development"].([]any) {
			if _, ok := card.(map[string]any)["card"]; ok {
				t.Fatal("hidden edge development exposed", i)
			}
		}
		ports := view["legal"].(map[string]any)["ports"].([]any)
		if (len(ports) > 0) != (i == 0) {
			t.Fatal("wrong port actor hints", i)
		}
		for j, raw := range view["players"].([]any) {
			_, ok := raw.(map[string]any)["dev"]
			if ok != (i == j) {
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
		t.Fatal("port response/continuation/clock lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_port", "edge": edge, "slot": 0}, 200)
	r = next.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.Seafarers.Tribe.Pending != nil || r.Game.CatanPendingActor() != -1 {
		t.Fatal("HTTP port failed to resume action")
	}
	left := r.TurnDeadline - time.Now().UnixMilli()
	if left < 44000 || left > 45000 {
		t.Fatal("response gave a fresh action clock", left)
	}
}
func TestCatanTribePortChainDeadlineTimeoutAndAutoplay(t *testing.T) {
	s, _, _, id := newCatanTable(t)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	edge, _ := tribeServerFixture(t, r)
	g := r.Game.Catan
	tr := g.Seafarers.Tribe
	tr.Ports = nil
	tr.HeldPorts[0] = []int{0, 1}
	tr.Pending = &game.CatanTribePortPending{Player: 0, Resume: "catan_turn"}
	r.Game.Phase = "catan_port"
	now := time.Now()
	r.TurnDeadline = now.Add(40 * time.Second).UnixMilli()
	r.CatanTimeLeft = 23000
	before, _ := json.Marshal(r)
	if err := r.applyGameAction(0, game.Action{Type: "catan_port", Edge: edge, Slot: 4}, now); err == nil {
		t.Fatal("invalid port accepted")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("invalid port changed game/clock")
	}
	if err := r.applyGameAction(0, game.Action{Type: "catan_port", Edge: edge, Slot: 0}, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if r.Game.Phase != "catan_port" || r.TurnDeadline != now.Add(40*time.Second).UnixMilli() {
		t.Fatal("same player's port chain refreshed clock")
	}
	expiry := now.Add(41 * time.Second)
	s.expireSetups(expiry)
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.Seafarers.Tribe.Pending != nil || len(r.Game.Catan.Seafarers.Tribe.HeldPorts[0]) != 0 || r.TurnDeadline != expiry.Add(23*time.Second).UnixMilli() {
		t.Fatal("timeout failed to place remaining port/restore clock")
	}
	// Re-enter a response with a fresh coastal position to verify the existing
	// autoplay dispatcher recognizes the required-choice actor.
	g = r.Game.Catan
	g.Ports = nil
	tr = g.Seafarers.Tribe
	tr.HeldPorts[0] = []int{-1}
	tr.Pending = &game.CatanTribePortPending{Player: 0, Resume: "catan_turn"}
	r.Game.Phase = "catan_port"
	r.Seats[0].AutoPlay = true
	r.BotAt = 0
	r.CatanTimeLeft = 17000
	r.TurnDeadline = expiry.Add(turnLimit).UnixMilli()
	s.runBots(expiry.Add(time.Second))
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || r.Game.Catan.Seafarers.Tribe.Pending != nil || r.TurnDeadline != expiry.Add(18*time.Second).UnixMilli() {
		t.Fatal("autoplay port failed / clock changed")
	}
}
func TestCatanTribePortTimeoutResumesHelperAndSetup(t *testing.T) {
	for _, setup := range []bool{false, true} {
		s, _, _, id := newCatanTable(t)
		s.mu.Lock()
		r := s.rooms[id]
		_, _ = tribeServerFixture(t, r)
		g := r.Game.Catan
		tr := g.Seafarers.Tribe
		tr.Ports = nil
		tr.HeldPorts[0] = []int{0}
		tr.Pending = &game.CatanTribePortPending{Player: 0, Resume: "catan_turn", Helper: true}
		g.Options.Helpers = true
		g.TurnSerial = 2
		g.Players[0].Helper = &game.CatanHelperSeat{ID: 8}
		g.HelperDisplay = []int{1, 2}
		if setup {
			g.Options.Helpers = false
			g.SetupStep = 0
			g.StartPlayer = 0
			tr.Pending.Helper = false
			tr.Pending.Resume = "catan_setup_road"
			tr.Pending.AfterRoute = &game.CatanRouteCompletion{Player: 0, Edge: 0, Setup: true}
		}
		r.Game.Phase = "catan_port"
		now := time.Now()
		r.CatanTimeLeft = 19000
		r.TurnDeadline = now.Add(-time.Second).UnixMilli()
		s.expireSetups(now)
		r = s.rooms[id]
		if setup {
			if r.Game.Phase != "catan_setup_settlement" || r.Game.Turn != 1 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
				t.Fatal("setup did not get new clock")
			}
		} else {
			if r.Game.Phase != "catan_helper" || r.TurnDeadline != now.Add(turnLimit).UnixMilli() || r.CatanTimeLeft != 19000 {
				t.Fatal("helper exchange did not get response window")
			}
			s.expireSetups(now.Add(turnLimit))
			r = s.rooms[id]
			if r.Game.Phase != "catan_turn" || r.TurnDeadline != now.Add(turnLimit+19*time.Second).UnixMilli() {
				t.Fatal("helper chain did not restore action time")
			}
		}
		s.mu.Unlock()
	}
}
