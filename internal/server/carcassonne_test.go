package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCarcassonneHTTPPersistenceBotsAndHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	a, v := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("城主")
	v.register("田野观众")
	a.post("/api/rooms", map[string]any{"name": "bad", "kind": "carcassonne", "capacity": 6}, 400)
	r := soloRoom(t, a, "carcassonne")
	id := r["id"].(string)
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	view := current(v)["game"].(map[string]any)["carcassonne"].(map[string]any)
	if _, ok := view["deck"]; ok {
		t.Fatal("deck exposed")
	}
	if len(view["legal"].([]any)) != 0 {
		t.Fatal("spectator actions")
	}
	v.command(current(v), "action", map[string]any{"type": "car_place"}, 400)
	deadline := s.rooms[id].TurnDeadline
	a1, e := s.rooms[id].Game.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	a.command(r, "action", a1, 200)
	if s.rooms[id].TurnDeadline != deadline {
		t.Fatal("tile placement reset clock")
	}
	a.command(r, "action", a1, 409)
	saved, _ := json.Marshal(s.rooms[id].Game)
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
	a.base = ts2.URL
	v.base = ts2.URL
	restored, _ := json.Marshal(next.rooms[id].Game)
	if string(saved) != string(restored) || next.rooms[id].TurnDeadline != deadline {
		t.Fatal("restart changed state")
	}
	action, e := next.rooms[id].Game.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	a.command(current(a), "action", action, 200)
	if next.rooms[id].Game.Turn != 1 || next.rooms[id].TurnDeadline == deadline {
		t.Fatal("turn clock")
	}
	// Exercise actual bot scheduling and human HTTP actions through the whole deck.
	for steps := 0; steps < 150 && !next.rooms[id].Game.Finished; steps++ {
		r := next.rooms[id]
		if r.Game.Turn == 1 {
			botTick(next, id)
		} else {
			action, e := r.Game.BotAction(0)
			if e != nil {
				t.Fatal(e)
			}
			a.command(current(a), "action", action, 200)
		}
	}
	if next.rooms[id].Status != "finished" {
		t.Fatal("game did not finish")
	}
	var n int
	if e := next.db.QueryRow("SELECT count(*) FROM match_history WHERE id=?", next.rooms[id].MatchID).Scan(&n); e != nil || n != 1 {
		t.Fatal("history", e, n)
	}
	v.post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
	code, profile := a.request("GET", "/api/players/"+next.rooms[id].Seats[0].ID, nil)
	if code != 200 {
		t.Fatal(code, profile)
	}
	if profile["stats"].(map[string]any)["carcassonne"].(map[string]any)["played"] != float64(1) {
		t.Fatal("profile stats")
	}
}
func TestCarcassonneTimeoutContinues(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := []*testClient{newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)}
	for i, name := range []string{"城堡骑士", "道路旅人", "田野农民"} {
		clients[i].register(name)
	}
	a := clients[0]
	r := a.post("/api/rooms", map[string]any{"name": "超时测试", "kind": "carcassonne", "capacity": 3}, 201)
	id := r["id"].(string)
	for _, c := range clients[1:] {
		c.command(current(a), "join", nil, 200)
	}
	for _, c := range clients {
		c.command(current(a), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	action, _ := s.rooms[id].Game.BotAction(0)
	a.command(current(a), "action", action, 200)
	s.mu.Lock()
	s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	_ = s.save(s.rooms[id])
	s.mu.Unlock()
	s.mu.Lock()
	s.expireSetups(time.Now())
	s.mu.Unlock()
	r = current(clients[1])
	target := s.rooms[id].Seats[0].ID
	kick(clients[1], r, target, 400)
	g := s.rooms[id].Game
	if g.Carcassonne.Players[0].Eliminated || !s.rooms[id].Seats[0].AutoPlay || g.Turn != 1 || g.Finished || len(g.Carcassonne.Tiles) != 2 {
		t.Fatal("timeout takeover did not finish the follower choice and preserve the player")
	}
	setAutoPlay(clients[0], current(clients[0]), false, 200)
}
