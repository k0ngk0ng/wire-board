package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func newCatanTable(t *testing.T) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	players := []*testClient{}
	for _, name := range []string{"岛主", "港口商人", "探险家", "观众"} {
		c := newClient(t, ts.URL)
		c.register(name)
		players = append(players, c)
	}
	a := players[0]
	// Two seats now select the supported two-player variant; one seat is still invalid.
	a.post("/api/rooms", map[string]any{"name": "invalid", "kind": "catan", "capacity": 1}, 400)
	r := a.post("/api/rooms", map[string]any{"name": "卡坦岛验证", "kind": "catan", "capacity": 3}, 201)
	for i := 1; i < 3; i++ {
		players[i].command(current(a), "join", nil, 200)
	}
	for i := 0; i < 3; i++ {
		players[i].command(current(a), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	id := r["id"].(string)
	for s.rooms[id].Game.Catan.SetupStep < 6 {
		room := s.rooms[id]
		a, e := room.Game.BotAction(room.Game.Turn)
		if e != nil {
			t.Fatal(e)
		}
		players[room.Game.Turn].command(current(players[0]), "action", a, 200)
	}
	return s, ts, players, id
}
func TestCatanMultiplayerDiscardTradePrivacyAndRestart(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	a, b, c, v := clients[0], clients[1], clients[2], clients[3]
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	room := current(v)
	view := room["game"].(map[string]any)["catan"].(map[string]any)
	for _, raw := range view["players"].([]any) {
		p := raw.(map[string]any)
		if _, ok := p["resources"]; ok {
			t.Fatal("spectator hand exposed")
		}
		if _, ok := p["dev"]; ok {
			t.Fatal("spectator development exposed")
		}
	}
	v.command(current(v), "action", map[string]any{"type": "catan_roll"}, 400)
	s.mu.Lock()
	r := s.rooms[id]
	g := r.Game.Catan
	for i := range g.Players {
		for color, n := range g.Players[i].Resources {
			g.Bank[color] += n
			g.Players[i].Resources[color] = 0
		}
		g.Players[i].Resources[i] = 10
		g.Bank[i] -= 10
	}
	r.Game.Phase = "catan_discard"
	g.ResumePhase = "catan_turn"
	g.DiscardDue = []int{5, 5, 5}
	r.CatanPendingVersion = r.Version
	r.startTurnClock(time.Now())
	if e := s.save(r); e != nil {
		t.Fatal(e)
	}
	s.mu.Unlock()
	initial := current(a)
	deadline := initial["turnDeadline"]
	a.command(initial, "action", map[string]any{"type": "catan_discard", "tokens": []int{5, 0, 0, 0, 0}}, 200)
	b.command(initial, "action", map[string]any{"type": "catan_discard", "tokens": []int{0, 5, 0, 0, 0}}, 200)
	if current(b)["turnDeadline"] != deadline {
		t.Fatal("partial discard reset clock")
	}
	c.command(initial, "action", map[string]any{"type": "catan_discard", "tokens": []int{0, 0, 5, 0, 0}}, 200)
	if s.rooms[id].Game.Phase != "catan_robber" {
		t.Fatal("discard phase")
	}
	for s.rooms[id].Game.Phase != "catan_turn" {
		action, e := s.rooms[id].Game.BotAction(0)
		if e != nil {
			t.Fatal(e)
		}
		a.command(current(a), "action", action, 200)
	}
	// Both responders have sheep; two responses to the same snapshot must survive.
	s.mu.Lock()
	r = s.rooms[id]
	g = r.Game.Catan
	for _, i := range []int{1, 2} {
		g.Players[i].Resources[2]++
		g.Bank[2]--
	}
	_ = s.save(r)
	s.mu.Unlock()
	a.command(current(a), "action", map[string]any{"type": "catan_trade_offer", "give": []int{1, 0, 0, 0, 0}, "take": []int{0, 0, 1, 0, 0}}, 200)
	offer := current(a)
	offerID := s.rooms[id].Game.Catan.Trade.ID
	b.command(offer, "action", map[string]any{"type": "catan_trade_accept", "offer": offerID}, 200)
	c.command(offer, "action", map[string]any{"type": "catan_trade_accept", "offer": offerID}, 200)
	if s.rooms[id].Game.Catan.Trade.Responses[1] != 1 || s.rooms[id].Game.Catan.Trade.Responses[2] != 1 {
		t.Fatal("response overwritten")
	}
	saved, _ := json.Marshal(s.rooms[id].Game)
	savedDeadline := s.rooms[id].TurnDeadline
	ts.Close()
	s.Close()
	next, e := New(s.cfg, s.files)
	if e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	stopBotTicker(next)
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, client := range clients {
		client.base = ts2.URL
	}
	restored, _ := json.Marshal(next.rooms[id].Game)
	if string(saved) != string(restored) || next.rooms[id].TurnDeadline != savedDeadline {
		t.Fatal("restart changed game or clock")
	}
	a.command(current(a), "action", map[string]any{"type": "catan_trade_complete", "offer": offerID, "target": 1}, 200)
	if next.rooms[id].Game.Catan.Trade != nil {
		t.Fatal("trade not completed")
	}
	c.command(current(c), "action", map[string]any{"type": "catan_trade_accept", "offer": offerID}, 400)
	a.command(current(a), "close", nil, 200)
	aid := a.state()["user"].(map[string]any)["id"].(string)
	code, profile := a.request("GET", "/api/players/"+aid, nil)
	if code != 200 || profile["total"] != float64(1) {
		t.Fatal(profile)
	}
}
func TestCatanHTTPFullGameAndHistory(t *testing.T) {
	s, _, clients, id := newCatanTable(t)
	steps := 0
	for !s.rooms[id].Game.Finished && steps < 1800 {
		state := s.rooms[id].Game
		player := state.Turn
		if state.Phase == "catan_discard" {
			for i, n := range state.Catan.DiscardDue {
				if n > 0 {
					player = i
					break
				}
			}
		}
		a, e := state.BotAction(player)
		if e != nil {
			t.Fatal(e)
		}
		clients[player].command(current(clients[player]), "action", a, 200)
		steps++
	}
	r := s.rooms[id]
	if r.Status != "finished" {
		t.Fatal("game did not finish")
	}
	winner := r.Game.Winners[0]
	uid := clients[winner].state()["user"].(map[string]any)["id"].(string)
	_, profile := clients[3].request("GET", "/api/players/"+uid, nil)
	stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
	if stats["wins"] != float64(1) || stats["played"] != float64(1) {
		t.Fatal("history stats", profile)
	}
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
}
func TestCatanAutomaticPendingAndBotScheduling(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	a := newClient(t, ts.URL)
	a.register("Alice")
	r := a.post("/api/rooms", map[string]any{"name": "Solo Catan", "kind": "catan", "capacity": 3}, 201)
	a.command(r, "add_bot", nil, 200)
	a.command(current(a), "add_bot", nil, 200)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	id := r["id"].(string)
	s.mu.Lock()
	s.expireSetups(time.UnixMilli(s.rooms[id].TurnDeadline).Add(time.Second))
	s.mu.Unlock()
	if s.rooms[id].Game.Catan.SetupStep != 0 || s.rooms[id].Game.Phase != "catan_setup_road" || !s.rooms[id].Seats[0].AutoPlay {
		t.Fatal("timeout did not start persistent setup control")
	}
	botTick(s, id)
	if s.rooms[id].Game.Catan.SetupStep != 1 {
		t.Fatal("automatic human setup")
	}
	for s.rooms[id].Game.Turn != 0 {
		botTick(s, id)
	}
	if s.rooms[id].Game.Catan.SetupStep != 5 {
		t.Fatal("snake bot placement")
	}
	// Seat zero remains controlled when the snake setup returns to them.
	botTick(s, id)
	botTick(s, id)
	if s.rooms[id].Game.Phase != "catan_roll" {
		t.Fatal("auto setup completion")
	}
}
