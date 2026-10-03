package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestDotaHTTPPrivacyParallelPersistenceAndTeamRatings(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, 4)
	for i := range clients {
		clients[i] = newClient(t, ts.URL)
		clients[i].register(fmt.Sprintf("遗迹玩家%d", i))
	}
	a := clients[0]
	a.post("/api/rooms", map[string]any{"name": "奇数席位", "kind": "dota", "capacity": 3}, 400)
	r := a.post("/api/rooms", map[string]any{"name": "遗迹之战", "kind": "dota", "capacity": 4}, 201)
	id := r["id"].(string)
	for _, c := range clients[1:] {
		c.command(current(a), "join", nil, 200)
	}
	if _, ok := current(a)["game"]; ok {
		t.Fatal("lobby started team setup before start")
	}
	for _, c := range clients {
		c.command(current(c), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	v := newClient(t, ts.URL)
	v.register("遗迹观众")
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	base := current(a)
	sequence := s.rooms[id].Game.Dota.Sequence
	deadline := s.rooms[id].TurnDeadline
	for i, c := range clients {
		c.command(base, "action", game.Action{Type: "dota_confirm", Prompt: sequence}, 200)
		if i < 3 && s.rooms[id].TurnDeadline != deadline {
			t.Fatal("parallel confirmation extended clock")
		}
	}
	v.command(current(v), "action", game.Action{Type: "dota_pick", Prompt: s.rooms[id].Game.Dota.Sequence, Choice: "axe"}, 400)
	for s.rooms[id].Game.Phase == "dota_draft" {
		p := s.rooms[id].Game.Turn
		action, e := s.rooms[id].Game.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		clients[p].command(current(clients[p]), "action", action, 200)
	}
	base = current(a)
	sequence = s.rooms[id].Game.Dota.Sequence
	deadline = s.rooms[id].TurnDeadline
	action, e := s.rooms[id].Game.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	a.command(base, "action", action, 200)
	if s.rooms[id].TurnDeadline != deadline {
		t.Fatal("locking plan reset shared deadline")
	}
	for i, c := range clients {
		d := current(c)["game"].(map[string]any)["dota"].(map[string]any)
		if _, ok := d["pending"]; ok {
			t.Fatal("pending leaked")
		}
		plans := d["plans"].(map[string]any)
		_, seen := plans["0"]
		if seen != (s.rooms[id].Game.Dota.Players[i].Team == s.rooms[id].Game.Dota.Players[0].Team) {
			t.Fatal("incorrect team privacy")
		}
	}
	if len(current(v)["game"].(map[string]any)["dota"].(map[string]any)["plans"].(map[string]any)) != 0 {
		t.Fatal("spectator read plan")
	}
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
	for _, c := range clients {
		c.base = ts2.URL
	}
	v.base = ts2.URL
	restored, _ := json.Marshal(next.rooms[id].Game)
	if string(saved) != string(restored) || next.rooms[id].TurnDeadline != deadline {
		t.Fatal("restart lost hidden plan or clock")
	}
	for i := 1; i < 4; i++ {
		action, e := next.rooms[id].Game.BotAction(i)
		if e != nil {
			t.Fatal(e)
		}
		clients[i].command(base, "action", action, 200)
	}
	if next.rooms[id].Game.Round != 2 {
		t.Fatal("simultaneous stale-version submissions did not resolve")
	}
	a.command(current(a), "action", game.Action{Type: "dota_plan", Prompt: sequence, Dota: action.Dota}, 400)
	for _, c := range clients {
		setAutoPlay(c, current(c), true, 200)
	}
	for step := 0; step < 400 && !next.rooms[id].Game.Finished; step++ {
		botTick(next, id)
	}
	room := next.rooms[id]
	if room.Status != "finished" || len(room.Game.Winners) != 2 {
		t.Fatal("team match failed to finish")
	}
	var record MatchRecord
	var raw []byte
	if e = next.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", room.MatchID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	_ = json.Unmarshal(raw, &record)
	if !record.Rated {
		t.Fatal("human autoplay match lost rating")
	}
	for _, p := range record.Players {
		want := -10
		if p.Won {
			want = 20
		}
		if p.RatingDelta != want || p.Score != nil {
			t.Fatalf("bad team result: %+v", p)
		}
	}
	var count int
	_ = next.db.QueryRow("SELECT count(*) FROM rating_ledger WHERE match_id=?", room.MatchID).Scan(&count)
	if count != 4 {
		t.Fatal("wrong rating records", count)
	}
	_, profile := a.request("GET", "/api/players/"+room.Seats[0].ID, nil)
	if profile["stats"].(map[string]any)["dota"].(map[string]any)["played"] != float64(1) {
		t.Fatal("missing Dota history")
	}
	v.post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
}

func TestDotaTimeoutAutoplayAndCancellation(t *testing.T) {
	s, _, clients, id := autoPlayTable(t, "dota")
	r := s.rooms[id]
	clients[0].command(current(clients[0]), "action", game.Action{Type: "dota_confirm", Prompt: r.Game.Dota.Sequence}, 200)
	s.mu.Lock()
	s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	s.expireSetups(time.Now())
	s.mu.Unlock()
	r = s.rooms[id]
	if r.Seats[0].AutoPlay || !r.Seats[1].AutoPlay || r.Seats[1].Left || r.Game.Phase != "dota_draft" {
		t.Fatal("timeout did not preserve team seat")
	}
	setAutoPlay(clients[1], current(clients[1]), false, 200)
	if s.rooms[id].Seats[1].AutoPlay {
		t.Fatal("cannot cancel timeout autoplay")
	}
	before := cloneServerDota(t, s.rooms[id])
	for i := 0; i < 3; i++ {
		botTick(s, id)
	}
	if !reflect.DeepEqual(before, cloneServerDota(t, s.rooms[id])) {
		t.Fatal("bot acted after cancellation")
	}
	kick(clients[0], current(clients[0]), r.Seats[1].ID, 400)
	// Shared deadline handles all remaining planning actors once, even without browsers.
	for s.rooms[id].Game.Phase == "dota_draft" {
		p := s.rooms[id].Game.Turn
		a, e := s.rooms[id].Game.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
	s.mu.Lock()
	s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	s.expireSetups(time.Now())
	s.mu.Unlock()
	if s.rooms[id].Game.Round != 2 {
		t.Fatal("timeout failed to resolve whole shared round")
	}
	for i, p := range s.rooms[id].Seats {
		if !p.AutoPlay || p.Left || p.Bot {
			t.Fatal("wrong takeover seat " + strconv.Itoa(i))
		}
	}
}
func cloneServerDota(t *testing.T, r *Room) string {
	t.Helper()
	b, e := json.Marshal(r.Game)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
