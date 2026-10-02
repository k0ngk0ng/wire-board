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

func TestTimeoutAuthorizationHostTransferAndContinuation(t *testing.T) {
	s, ts := setupServer(t)
	a, b, c, outsider := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	for i, client := range []*testClient{a, b, c, outsider} {
		client.register([]string{"Alice", "Bobby", "Carol", "David"}[i])
	}
	r := a.post("/api/rooms", map[string]any{"name": "Timed game", "kind": "splendor", "capacity": 3}, 201)
	b.command(r, "join", nil, 200)
	c.command(current(a), "join", nil, 200)
	for _, client := range []*testClient{a, b, c} {
		client.command(current(client), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	r = current(a)
	id := r["id"].(string)
	aID := a.state()["user"].(map[string]any)["id"].(string)
	bID := b.state()["user"].(map[string]any)["id"].(string)
	deadline := int64(r["turnDeadline"].(float64))
	if remaining := time.Until(time.UnixMilli(deadline)); remaining > 120*time.Second || remaining < 118*time.Second {
		t.Fatal("incorrect turn limit", remaining)
	}
	kick(b, r, aID, 400) // Before timeout.
	if current(a)["version"] != r["version"] {
		t.Fatal("rejected kick changed state")
	}
	expireTurn(t, s, id)
	kick(outsider, r, aID, 400)
	kick(a, r, aID, 400)
	kick(b, r, bID, 400)
	kick(b, current(b), aID, 200)
	if _, ok := a.state()["room"]; ok {
		t.Fatal("kicked user retained room access")
	}
	r = current(b)
	if r["status"] != "playing" || r["host"] != bID || r["you"] != float64(1) {
		t.Fatal("host or seat changed incorrectly", r)
	}
	if r["game"].(map[string]any)["turn"] != float64(1) || int64(r["turnDeadline"].(float64)) <= time.Now().UnixMilli() {
		t.Fatal("next turn did not start")
	}
	a.command(r, "action", map[string]any{"type": "reserve", "tier": 1}, 400)
	a.command(r, "join", nil, 400)
	b.command(r, "action", map[string]any{"type": "reserve", "tier": 1}, 200)
	c.command(current(c), "action", map[string]any{"type": "reserve", "tier": 1}, 200)
	if current(b)["game"].(map[string]any)["turn"] != float64(1) {
		t.Fatal("rotation did not skip departed seat")
	}
	// A non-host cannot end the game; preserve the existing host-only behavior.
	c.command(current(c), "close", nil, 400)
	b.command(current(b), "close", nil, 200)
	if current(b)["status"] != "closed" || current(b)["turnDeadline"] != float64(0) {
		t.Fatal("host could not close game")
	}
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
	aID := a.state()["user"].(map[string]any)["id"].(string)
	kick(b, current(b), aID, 200)
	r = current(b)
	if r["status"] != "finished" || r["turnDeadline"] != float64(0) {
		t.Fatal("two-player timeout did not finish game")
	}
	if winners := r["game"].(map[string]any)["winners"].([]any); len(winners) != 1 || winners[0] != float64(1) {
		t.Fatal("incorrect survivor winner")
	}
	b.command(r, "rematch", nil, 200)
	if current(b)["turnDeadline"] != float64(0) || len(current(b)["seats"].([]any)) != 1 {
		t.Fatal("rematch retained deadline or eliminated seat")
	}
}

func TestLateActionWinsRaceAgainstStaleKick(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := startRoom(t, a, b, "splendor")
	expireTurn(t, s, r["id"].(string))
	aID := a.state()["user"].(map[string]any)["id"].(string)
	a.command(r, "action", map[string]any{"type": "reserve", "tier": 1}, 200)
	kick(b, r, aID, 409)
	if current(a)["game"].(map[string]any)["turn"] != float64(1) {
		t.Fatal("late action did not safely complete")
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
	kick(b, current(b), a.state()["user"].(map[string]any)["id"].(string), 200)
	if current(b)["status"] != "finished" {
		t.Fatal("timeout did not award survivor")
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
