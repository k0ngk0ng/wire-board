package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func expireTurn(t *testing.T, s *Server, id string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
}

func kick(c *testClient, r map[string]any, target string, want int) {
	c.t.Helper()
	c.post("/api/rooms/"+r["id"].(string), map[string]any{
		"type": "kick_timeout", "target": target, "version": r["version"], "nonce": randomID(12),
	}, want)
}

func TestTimeoutAutoplayPreservesHostAndSeat(t *testing.T) {
	s, _, clients, id := autoPlayTable(t, "splendor")
	a, b := clients[0], clients[1]
	r := current(a)
	userID := s.rooms[id].Seats[0].ID
	kick(b, r, userID, 400)
	expireTurn(t, s, id)
	s.mu.Lock()
	s.expireSetups(time.Now())
	s.mu.Unlock()
	r = current(a)
	if r["host"] != userID || r["status"] != "playing" || s.rooms[id].Seats[0].Left || !s.rooms[id].Seats[0].AutoPlay || s.rooms[id].Game.Turn != 1 {
		t.Fatal("timeout removed human or failed to continue")
	}
	kick(b, current(b), userID, 400)
	a.command(current(a), "action", map[string]any{"type": "reserve", "tier": 1}, 400)
	setAutoPlay(a, current(a), false, 200)
	b.command(current(b), "action", map[string]any{"type": "reserve", "tier": 1}, 200)
	a.command(current(a), "action", map[string]any{"type": "reserve", "tier": 1}, 200)
	b.command(current(b), "close", nil, 400)
	a.command(current(a), "close", nil, 200)
}

func TestTimeoutClockDoesNotResetDuringSubstepsOrRestart(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := startRoom(t, a, b, "splendor")
	id := r["id"].(string)
	deadline := r["turnDeadline"]
	s.mu.Lock()
	g := s.rooms[id].Game.Splendor
	g.Players[0].Tokens = []int{2, 2, 2, 2, 2, 0}
	g.Players[0].Bonus = []int{5, 5, 5, 5, 5}
	s.mu.Unlock()
	a.command(current(a), "action", map[string]any{"type": "reserve", "tier": 1}, 200)
	if current(a)["game"].(map[string]any)["phase"] != "discard" || current(a)["turnDeadline"] != deadline {
		t.Fatal("discard reset clock")
	}
	a.command(current(a), "action", map[string]any{"type": "discard", "tokens": []int{0, 0, 0, 0, 0, 1}}, 200)
	if current(a)["game"].(map[string]any)["phase"] != "noble" || current(a)["turnDeadline"] != deadline {
		t.Fatal("noble selection reset clock")
	}
	expireTurn(t, s, id)
	before, _ := json.Marshal(current(a))
	ts.Close()
	s.Close()
	restarted, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	ts2 := httptest.NewServer(restarted.Handler())
	defer ts2.Close()
	a.base, b.base = ts2.URL, ts2.URL
	after, _ := json.Marshal(current(a))
	if string(before) != string(after) {
		t.Fatal("restart changed deadline or game")
	}
	restarted.mu.Lock()
	restarted.expireSetups(time.Now())
	restarted.mu.Unlock()
	r = current(a)
	if r["status"] != "playing" || !restarted.rooms[id].Seats[0].AutoPlay || restarted.rooms[id].Seats[0].Left || restarted.rooms[id].Game.Turn != 1 {
		t.Fatal("expired noble selection did not preserve the player")
	}
	setAutoPlay(a, r, false, 200)
	a.command(current(a), "close", nil, 200)
	a.command(current(a), "rematch", nil, 200)
	if restarted.rooms[id].Seats[0].AutoPlay || restarted.rooms[id].Seats[0].TimeoutAutoPlay || len(restarted.rooms[id].Seats) != 2 {
		t.Fatal("rematch retained control flag or lost a seat")
	}

}

func TestExpiredActionCannotRacePastAutomaticTakeover(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := startRoom(t, a, b, "splendor")
	expireTurn(t, s, r["id"].(string))
	aID := a.state()["user"].(map[string]any)["id"].(string)
	a.command(r, "action", map[string]any{"type": "reserve", "tier": 1}, 409)
	kick(b, r, aID, 409)
	if current(a)["game"].(map[string]any)["turn"] != float64(1) {
		t.Fatal("takeover did not safely complete")
	}
}

func TestLegacyPlayingRoomReceivesPersistentClock(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := startRoom(t, a, b, "splendor")
	id := r["id"].(string)
	s.mu.Lock()
	s.rooms[id].TurnDeadline = 0
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	ts.Close()
	s.Close()
	for i := 0; i < 2; i++ {
		restarted, err := New(s.cfg, s.files)
		if err != nil {
			t.Fatal(err)
		}
		got := restarted.rooms[id].TurnDeadline
		if got <= 0 || (i == 1 && got != s.rooms[id].TurnDeadline) {
			t.Fatal("migration failed or deadline reset")
		}
		s.rooms[id].TurnDeadline = got
		restarted.Close()
	}
}

func TestRailSharedSetupVersionAndTurnDeadline(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := startRoom(t, a, b, "rail")
	id := r["id"].(string)
	s.mu.Lock()
	pa, pb := s.rooms[id].Game.Rail.SetupPending[0], s.rooms[id].Game.Rail.SetupPending[1]
	s.mu.Unlock()
	deadline := r["turnDeadline"]
	b.command(r, "action", map[string]any{"type": "keep", "keep": []int{pb[0].ID, pb[1].ID, pb[2].ID}}, 200)
	if current(a)["turnDeadline"] != deadline {
		t.Fatal("one selection reset shared clock")
	}
	// Both submissions use the same snapshot, as simultaneous clients do.
	a.command(r, "action", map[string]any{"type": "keep", "keep": []int{pa[0].ID, pa[1].ID}}, 200)
	r = current(a)
	if r["game"].(map[string]any)["phase"] != "turn" {
		t.Fatal("setup not completed")
	}
	deadline = r["turnDeadline"]
	a.command(r, "action", map[string]any{"type": "draw", "slot": -1}, 200)
	if current(a)["turnDeadline"] != deadline {
		t.Fatal("first draw reset deadline")
	}
	expireTurn(t, s, id)
	s.mu.Lock()
	s.expireSetups(time.Now())
	s.mu.Unlock()
	if s.rooms[id].Status != "playing" || s.rooms[id].Game.Turn != 1 || !s.rooms[id].Seats[0].AutoPlay || s.rooms[id].Seats[1].AutoPlay {
		t.Fatal("second draw did not continue under the original player's control")
	}

}

func TestRailSetupExpiresWithoutConnectedClients(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := startRoom(t, a, b, "rail")
	id := r["id"].(string)
	s.mu.Lock()
	pa := s.rooms[id].Game.Rail.SetupPending[0]
	s.mu.Unlock()
	a.command(r, "action", map[string]any{"type": "keep", "keep": []int{pa[0].ID, pa[1].ID, pa[2].ID}}, 200)
	expireTurn(t, s, id)
	// No API calls or WebSockets drive expiration: the background ticker does.
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		s.mu.Lock()
		done := !s.rooms[id].Game.Rail.Setup
		s.mu.Unlock()
		if done {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	room := s.rooms[id]
	if room.Game.Rail.Setup || len(room.Game.Rail.Players[0].Tickets) != 3 || len(room.Game.Rail.Players[1].Tickets) != 2 || room.TurnDeadline <= time.Now().UnixMilli() {
		t.Fatal("automatic setup failed")
	}
}

func TestAssetURLValidationAndCSP(t *testing.T) {
	for _, raw := range []string{"http://assets.example.com", "https://user:pass@assets.example.com", "https://assets.example.com/a?token=secret", "https://assets.example.com/a#fragment", "https://assets.example.com/a;script-src *"} {
		if _, err := New(Config{AssetsBaseURL: raw}, nil); err == nil {
			t.Fatal("accepted invalid assets URL", raw)
		}
	}
	s, ts := setupServer(t)
	s.cfg.AssetsBaseURL = "https://assets.example.com/games/v1"
	response, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	csp := response.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "img-src 'self' data: https://assets.example.com;") || !strings.Contains(csp, "script-src 'self';") {
		t.Fatal("incorrect image-only CSP", csp)
	}
}
