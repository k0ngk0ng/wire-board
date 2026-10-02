package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHistorySurvivesRematchAndRoomDeletion(t *testing.T) {
	s, ts := setupServer(t)
	a, b, v := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	v.register("Viewer")
	aid := a.state()["user"].(map[string]any)["id"].(string)
	r := a.post("/api/rooms", map[string]any{"name": "Archive", "kind": "splendor", "capacity": 2}, 201)
	b.command(r, "join", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	id := r["id"].(string)
	s.mu.Lock()
	room := s.rooms[id]
	room.Status = "finished"
	room.Game.Finished = true
	room.Game.Winners = []int{0}
	room.Game.Splendor.Players[0].Score = 17
	room.Game.Splendor.Players[1].Score = 14
	if err := s.save(room); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	if len(v.state()["rooms"].([]any)) != 0 {
		t.Fatal("finished room listed")
	}
	v.post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
	code, p := v.request("GET", "/api/players/"+aid, nil)
	if code != 200 || p["total"] != float64(1) {
		t.Fatal(code, p)
	}
	stats := p["stats"].(map[string]any)["splendor"].(map[string]any)
	if stats["played"] != float64(1) || stats["wins"] != float64(1) {
		t.Fatal(stats)
	}
	raw, _ := json.Marshal(p)
	for _, secret := range []string{"reserved", "tokens", "password", "ticketDeck", "chat"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("private snapshot exposed", secret)
		}
	}
	a.command(current(a), "rematch", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	a.command(current(a), "close", nil, 200)
	_, mine := a.request("GET", "/api/players/"+aid, nil)
	if mine["total"] != float64(2) {
		t.Fatal("rematch overwrote history", mine)
	}
	_, public := v.request("GET", "/api/players/"+aid, nil)
	if public["total"] != float64(1) {
		t.Fatal("aborted private history exposed")
	}
	a.command(current(a), "leave", nil, 200)
	b.command(current(b), "leave", nil, 200)
	_, mine = a.request("GET", "/api/players/"+aid, nil)
	if mine["total"] != float64(2) {
		t.Fatal("deleted room deleted history")
	}
	s.mu.Lock()
	if s.rooms[id] != nil {
		t.Fatal("room not deleted")
	}
	s.mu.Unlock()
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	restart := httptest.NewServer(next.Handler())
	defer restart.Close()
	a.base = restart.URL
	_, mine = a.request("GET", "/api/players/"+aid, nil)
	if mine["total"] != float64(2) {
		t.Fatal("history lost on restart")
	}
}

func TestProfilePaginationMigrationAndActiveLobby(t *testing.T) {
	s, ts := setupServer(t)
	a := newClient(t, ts.URL)
	a.register("Alice")
	u := a.state()["user"].(map[string]any)
	id := u["id"].(string)
	s.mu.Lock()
	for i := 0; i < 25; i++ {
		g, _ := game.New("rail", 2)
		g.Finished = true
		g.Winners = []int{0}
		r := &Room{ID: fmt.Sprintf("arch-%d", i), Name: "History", Kind: "rail", Seats: []Seat{{User: User{ID: id, Name: "Alice"}}, {User: User{ID: "bot-1", Name: "电脑"}, Bot: true}}, Status: "finished", Game: g, Updated: int64(i + 1), SetupVersion: 1}
		if err := s.save(r); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 6; i++ {
		r := &Room{ID: fmt.Sprintf("live-%d", i), Name: "Live", Status: "waiting"}
		s.rooms[r.ID] = r
	}
	s.mu.Unlock()
	if len(a.state()["rooms"].([]any)) != 6 {
		t.Fatal("lobby limited room count")
	}
	_, page := a.request("GET", "/api/players/"+id, nil)
	if len(page["history"].([]any)) != 20 || page["hasMore"] != true || page["total"] != float64(25) {
		t.Fatal(page)
	}
	_, page = a.request("GET", "/api/players/"+id+"?offset=20", nil)
	if len(page["history"].([]any)) != 5 || page["hasMore"] != false {
		t.Fatal(page)
	}
	_, page = a.request("GET", "/api/players/"+id+"?offset=999", nil)
	if len(page["history"].([]any)) != 0 {
		t.Fatal(page)
	}
	anon := newClient(t, ts.URL)
	if code, _ := anon.request("GET", "/api/players/"+id, nil); code != 401 {
		t.Fatal(code)
	}
	if code, _ := a.request("GET", "/api/players/bot-1", nil); code != 404 {
		t.Fatal("bot profile exposed")
	}
}

func TestFriendRequestsConsentAndIsolation(t *testing.T) {
	_, ts := setupServer(t)
	a, b, c := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	c.register("Carol")
	uid := func(c *testClient) string { return c.state()["user"].(map[string]any)["id"].(string) }
	aid, bid := uid(a), uid(b)
	a.post("/api/players/"+aid+"/friend", map[string]string{"action": "request"}, 400)
	a.post("/api/players/bot-1/friend", map[string]string{"action": "request"}, 404)
	path := "/api/players/" + bid + "/friend"
	a.post(path, map[string]string{"action": "request"}, 200)
	a.post(path, map[string]string{"action": "request"}, 409)
	a.post(path, map[string]string{"action": "accept"}, 403)
	c.post(path, map[string]string{"action": "accept"}, 403)
	_, profile := b.request("GET", "/api/players/"+aid, nil)
	if profile["relationship"] != "incoming" {
		t.Fatal(profile)
	}
	_, friends := c.request("GET", "/api/friends", nil)
	if len(friends["friends"].([]any)) != 0 {
		t.Fatal("other friendships exposed")
	}
	b.post("/api/players/"+aid+"/friend", map[string]string{"action": "accept"}, 200)
	for _, client := range []*testClient{a, b} {
		_, friends = client.request("GET", "/api/friends", nil)
		if len(friends["friends"].([]any)) != 1 || friends["friends"].([]any)[0].(map[string]any)["relationship"] != "friends" {
			t.Fatal(friends)
		}
	}
	c.post(path, map[string]string{"action": "remove"}, 200)
	_, profile = a.request("GET", "/api/players/"+bid, nil)
	if profile["relationship"] != "friends" {
		t.Fatal("third party removed friendship")
	}
	b.post("/api/players/"+aid+"/friend", map[string]string{"action": "remove"}, 200)
	_, profile = a.request("GET", "/api/players/"+bid, nil)
	if profile["relationship"] != "none" {
		t.Fatal(profile)
	}
	_, results := a.request("GET", "/api/players?q=Bob", nil)
	if len(results["players"].([]any)) != 1 {
		t.Fatal(results)
	}
}
