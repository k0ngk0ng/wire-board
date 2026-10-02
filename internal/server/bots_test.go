package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func soloRoom(t *testing.T, a *testClient, kind string) map[string]any {
	t.Helper()
	r := a.post("/api/rooms", map[string]any{"name": "Solo", "kind": kind, "capacity": 2}, 201)
	a.command(r, "add_bot", nil, 200)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	return current(a)
}
func stopBotTicker(s *Server) { s.cancel(); <-s.done }
func botTick(s *Server, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runBots(time.UnixMilli(s.rooms[id].BotAt).Add(time.Second))
}
func removeBot(c *testClient, r map[string]any, id string, want int) {
	c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "remove_bot", "target": id, "version": r["version"], "nonce": randomID(12)}, want)
}
func TestBotManagementPermissionsAndCleanup(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := a.post("/api/rooms", map[string]any{"name": "Bots", "kind": "splendor", "capacity": 3}, 201)
	b.command(r, "add_bot", nil, 400)
	a.command(r, "add_bot", nil, 200)
	r = current(a)
	bot := r["seats"].([]any)[1].(map[string]any)
	if bot["bot"] != true || bot["ready"] != true {
		t.Fatal("bot not ready")
	}
	id := bot["id"].(string)
	removeBot(b, r, id, 400)
	removeBot(a, r, a.state()["user"].(map[string]any)["id"].(string), 400)
	removeBot(a, r, id, 200)
	a.command(current(a), "add_bot", nil, 200)
	b.command(current(a), "join", nil, 200)
	a.command(current(a), "add_bot", nil, 400)
	b.command(current(b), "add_bot", nil, 400)
	a.command(current(a), "leave", nil, 200)
	r = current(b)
	if r["host"] != b.state()["user"].(map[string]any)["id"] {
		t.Fatal("host transferred to computer")
	}
	if r["seats"].([]any)[0].(map[string]any)["ready"] != true {
		t.Fatal("bot readiness lost")
	}
	b.command(r, "leave", nil, 200)
	if len(s.rooms) != 0 {
		t.Fatal("orphan bot room retained")
	}
	var users int
	if err := s.db.QueryRow("SELECT count(*) FROM users").Scan(&users); err != nil || users != 2 {
		t.Fatal("bots created login accounts", users, err)
	}
}
func TestSoloGamesAIContinuationPersistenceAndRematch(t *testing.T) {
	for _, kind := range []string{"splendor", "rail"} {
		t.Run(kind, func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			a := newClient(t, ts.URL)
			a.register("Alice")
			r := soloRoom(t, a, kind)
			id := r["id"].(string)
			a.command(r, "add_bot", nil, 400)
			botID := r["seats"].([]any)[1].(map[string]any)["id"].(string)
			removeBot(a, r, botID, 400)
			if kind == "rail" {
				deadline := r["turnDeadline"]
				botTick(s, id)
				if current(a)["turnDeadline"] != deadline {
					t.Fatal("bot setup reset deadline")
				}
				s.mu.Lock()
				pending := append([]game.Ticket{}, s.rooms[id].Game.Rail.SetupPending[0]...)
				s.mu.Unlock()
				a.command(r, "action", map[string]any{"type": "keep", "keep": []int{pending[0].ID, pending[1].ID}}, 200)
				a.command(current(a), "action", map[string]any{"type": "draw", "slot": -1}, 200)
				a.command(current(a), "action", map[string]any{"type": "draw", "slot": -1}, 200)
			} else {
				a.command(current(a), "action", map[string]any{"type": "reserve", "tier": 1}, 200)
			}
			if current(a)["game"].(map[string]any)["turn"] != float64(1) {
				t.Fatal("not bot turn")
			}
			// Restart exactly on the computer's turn; no browser is required to resume.
			ts.Close()
			s.Close()
			resumed, err := New(s.cfg, s.files)
			if err != nil {
				t.Fatal(err)
			}
			defer resumed.Close()
			stopBotTicker(resumed)
			ts2 := httptest.NewServer(resumed.Handler())
			defer ts2.Close()
			a.base = ts2.URL
			for i := 0; i < 4; i++ {
				botTick(resumed, id)
				if current(a)["game"].(map[string]any)["turn"] == float64(0) {
					break
				}
			}
			if current(a)["game"].(map[string]any)["turn"] != float64(0) {
				t.Fatal("AI did not complete its turn")
			}
			var actions int
			if err := resumed.db.QueryRow("SELECT count(*) FROM actions WHERE user_id=?", botID).Scan(&actions); err != nil || actions < 1 {
				t.Fatal("AI action not journaled", err)
			}
			a.command(current(a), "close", nil, 200)
			before, _ := json.Marshal(current(a))
			botTick(resumed, id)
			after, _ := json.Marshal(current(a))
			if string(before) != string(after) {
				t.Fatal("bot acted in closed game")
			}
			a.command(current(a), "rematch", nil, 200)
			r = current(a)
			if len(r["seats"].([]any)) != 2 || r["seats"].([]any)[1].(map[string]any)["ready"] != true {
				t.Fatal("rematch lost ready bot")
			}
			a.command(r, "leave", nil, 200)
			if len(resumed.rooms) != 0 {
				t.Fatal("solo room retained after last human left")
			}
		})
	}
}
func TestBotTickerActsWithoutBrowser(t *testing.T) {
	s, ts := setupServer(t)
	a := newClient(t, ts.URL)
	a.register("Alice")
	r := soloRoom(t, a, "splendor")
	a.command(r, "action", map[string]any{"type": "reserve", "tier": 1}, 200)
	id := r["id"].(string)
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		s.mu.Lock()
		done := s.rooms[id].Game.Turn == 0
		s.mu.Unlock()
		if done {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("timer did not drive AI turn")
}

func TestBotFinalTurnFinishesRoom(t *testing.T) {
	for _, kind := range []string{"splendor", "rail"} {
		t.Run(kind, func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			a := newClient(t, ts.URL)
			a.register("Alice")
			r := soloRoom(t, a, kind)
			id := r["id"].(string)
			s.mu.Lock()
			g := s.rooms[id].Game
			if kind == "rail" {
				g.AutoChooseRailSetup()
				g.Rail.LastRemaining = 1
			} else {
				g.Splendor.LastRound = true
				g.Splendor.Players[1].Score = 15
			}
			g.Turn = 1
			s.mu.Unlock()
			for i := 0; i < 4; i++ {
				botTick(s, id)
			}
			r = current(a)
			if r["status"] != "finished" || r["turnDeadline"] != float64(0) || len(r["game"].(map[string]any)["winners"].([]any)) == 0 {
				t.Fatal("AI finish not reflected in room", r["status"])
			}
			a.command(r, "rematch", nil, 200)
			a.command(current(a), "ready", nil, 200)
			a.command(current(a), "start", nil, 200)
		})
	}
}
