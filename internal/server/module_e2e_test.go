package server

// Failure cases fixed before implementation: unknown/duplicate registration,
// invalid options or capacity, unauthorized settings, stale/future requests,
// replayed nonces, rejected actions changing the snapshot, spectator secrets,
// restart losing state/deadlines, stalled bots, incorrect timeout takeover,
// duplicate history/rating, and rematch retaining a previous session.

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// Opt-in live fixture for scripts/accept_game_modules.py. It uses the same
// registered test module as the HTTP E2E, and serves the real production build.
func TestModuleBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_MODULE_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local", "module-refactor", "browser")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", AssetsBaseURL: os.Getenv("WIRE_BOARD_MODULE_ASSETS"), Modules: []gameapi.Module{acceptanceModule()}}, os.DirFS(filepath.Join(root, "web", "dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer httpServer.Close()
	go httpServer.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for start := time.Now(); time.Since(start) < 10*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}

// This game exists only in the test binary. It has an action field absent from
// the six legacy games and no shared State/Action or engine dependency.
func acceptanceModule() gameapi.Module {
	type position struct {
		Count  int    `json:"count"`
		Secret string `json:"secret"`
	}
	advance := func(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var action struct {
			Increment int `json:"increment"`
		}
		var data position
		if json.Unmarshal(raw, &action) != nil || action.Increment != 1 || len(match.Actors) != 1 || match.Actors[0] != ctx.Actor {
			return match, errors.New("invalid increment")
		}
		if err := json.Unmarshal(match.Data, &data); err != nil {
			return match, err
		}
		data.Count++
		match.Data, _ = json.Marshal(data)
		match.Actors = []int{data.Count % ctx.Players}
		match.TimeoutActors = append([]int(nil), match.Actors...)
		match.Deadline = ctx.Now.Add(120 * time.Second).UnixMilli()
		if data.Count == 4 {
			match.Result = &gameapi.Result{Ranks: []int{1, 2}, Winners: []int{0}, Scores: []int{4, 0}}
			match.Actors, match.TimeoutActors, match.Deadline = nil, nil, 0
		}
		return match, nil
	}
	return gameapi.Module{
		ID: "acceptance", Name: "接入验收", MinPlayers: 2, MaxPlayers: 2,
		Configure: func(players int, raw json.RawMessage) (json.RawMessage, error) {
			if players != 2 {
				return nil, errors.New("two players required")
			}
			var options struct {
				Mode string `json:"mode"`
			}
			if len(raw) > 0 && json.Unmarshal(raw, &options) != nil {
				return nil, errors.New("invalid options")
			}
			if options.Mode != "" && options.Mode != "normal" {
				return nil, errors.New("invalid mode")
			}
			return json.RawMessage(`{"mode":"normal"}`), nil
		},
		Start: func(ctx gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
			return gameapi.Match{Data: json.RawMessage(`{"count":0,"secret":"private-test-card"}`), Actors: []int{0}, TimeoutActors: []int{0}, Deadline: ctx.Now.Add(120 * time.Second).UnixMilli()}, nil
		},
		Apply: advance,
		View: func(match gameapi.Match, player int) (any, error) {
			var data position
			if err := json.Unmarshal(match.Data, &data); err != nil {
				return nil, err
			}
			if player != 0 {
				data.Secret = ""
			}
			return data, nil
		},
		BotAction: func(ctx gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
			return json.RawMessage(`{"increment":1}`), nil
		},
		Reclaim: func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
			match.Deadline = ctx.Now.Add(120 * time.Second).UnixMilli()
			return match, nil
		},
	}
}

func TestModuleRegistrationAndLifecycleE2E(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{acceptanceModule()}}
	files := fstest.MapFS{"index.html": {Data: []byte("Wire Board")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	publishModuleForTest(t, ts.URL, "acceptance")
	t.Cleanup(func() { ts.Close(); s.Close() })
	a, b, viewer := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("模块房主")
	b.register("模块玩家")
	viewer.register("模块观众")
	userID := a.state()["user"].(map[string]any)["id"].(string)
	code, profile := a.request("GET", "/api/players/"+userID, nil)
	if code != 200 || profile["stats"].(map[string]any)["acceptance"] == nil {
		t.Fatal("registered game missing from profile statistics", code, profile)
	}
	trace := []any{}
	t.Cleanup(func() {
		root := filepath.Join("..", "..", ".local", "module-refactor")
		if err := os.MkdirAll(root, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "test": t.Name(), "trace": trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "module-lifecycle.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	available := a.state()["availableGames"].([]any)
	found := false
	for _, value := range available {
		found = found || value == "acceptance"
	}
	if !found {
		t.Fatal("registered game missing from lobby")
	}
	a.post("/api/rooms", map[string]any{"kind": "unknown", "name": "bad", "capacity": 2}, 400)
	a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "bad", "capacity": 3}, 400)
	a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "bad", "capacity": 2, "options": map[string]any{"mode": "invalid"}}, 400)
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "独立模块", "capacity": 2}, 201)
	id := r["id"].(string)
	b.command(r, "join", nil, 200)
	r = current(a)
	b.post("/api/rooms/"+id, map[string]any{"type": "options", "options": map[string]any{"mode": "normal"}, "nonce": randomID(12), "version": r["version"]}, 400)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	r = current(a)
	viewer.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	data := func(c *testClient) map[string]any {
		return current(c)["game"].(map[string]any)["data"].(map[string]any)
	}
	if data(a)["secret"] != "private-test-card" || data(b)["secret"] != "" || data(viewer)["secret"] != "" {
		t.Fatal("private projection")
	}
	viewer.command(current(viewer), "action", map[string]any{"increment": 1}, 400)
	before := current(a)
	a.command(before, "action", map[string]any{"increment": 99}, 400)
	if !reflect.DeepEqual(before, current(a)) {
		t.Fatal("invalid action changed room")
	}
	future := map[string]any{"type": "action", "action": map[string]any{"increment": 1}, "nonce": randomID(12), "version": int(r["version"].(float64)) + 1}
	a.post("/api/rooms/"+id, future, 409)
	request := map[string]any{"type": "action", "action": map[string]any{"increment": 1}, "nonce": randomID(12), "version": r["version"]}
	a.post("/api/rooms/"+id, request, 200)
	a.post("/api/rooms/"+id, request, 200)
	if data(a)["count"] != float64(1) {
		t.Fatal("nonce replay applied twice")
	}
	b.command(r, "action", map[string]any{"increment": 1}, 409)
	saved := current(a)
	trace = append(trace, saved)
	ts.Close()
	s.Close()
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	a.base, b.base, viewer.base = ts.URL, ts.URL, ts.URL
	if !reflect.DeepEqual(saved, current(a)) {
		t.Fatal("restart changed room, deadline or projection")
	}
	for i, client := range []*testClient{b, a, b} {
		client.command(current(client), "action", map[string]any{"increment": 1}, 200)
		trace = append(trace, map[string]any{"step": i + 2, "room": current(a)})
	}
	r = current(a)
	if r["status"] != "finished" {
		t.Fatal("match did not finish", r)
	}
	result := r["result"].(map[string]any)
	code, profile = a.request("GET", "/api/players/"+userID, nil)
	if code != 200 || profile["stats"].(map[string]any)["acceptance"].(map[string]any)["played"] != float64(1) {
		t.Fatal("module history missing from profile", code, profile)
	}
	players := result["players"].([]any)
	if players[0].(map[string]any)["ratingDelta"] != float64(20) || players[1].(map[string]any)["ratingDelta"] != float64(-5) {
		t.Fatal("module ranking not used", result)
	}
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger").Scan(&count); err != nil || count != 2 {
		t.Fatal("ratings", count, err)
	}
	a.command(r, "rematch", nil, 200)
	r = current(a)
	if r["status"] != "waiting" || r["game"] != nil {
		t.Fatal("rematch retained session")
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger").Scan(&count); err != nil || count != 2 {
		t.Fatal("duplicate ratings", count, err)
	}
}

func TestModuleBotAndTimeoutE2E(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{acceptanceModule()}}
	s, err := New(cfg, fstest.MapFS{"index.html": {Data: []byte("Wire Board")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	publishModuleForTest(t, ts.URL, "acceptance")
	defer ts.Close()
	a := newClient(t, ts.URL)
	a.register("模块托管")
	a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "托管验收", "capacity": 2}, 201)
	a.command(current(a), "add_bot", nil, 200)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	r := current(a)
	id := r["id"].(string)
	// Only the clock is advanced; moves use the real scheduler and HTTP API.
	s.mu.Lock()
	s.rooms[id].Module.Deadline = time.Now().Add(-time.Second).UnixMilli()
	s.rooms[id].TurnDeadline = s.rooms[id].Module.Deadline
	lastActive := s.rooms[id].LastActive
	s.mu.Unlock()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		r = current(a)
		if r["status"] == "finished" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if r["status"] != "finished" {
		t.Fatal("natural bot/timeout scheduler stalled", r)
	}
	if r["result"].(map[string]any)["rated"] != false {
		t.Fatal("bot match rated")
	}
	if !r["seats"].([]any)[0].(map[string]any)["autoPlay"].(bool) {
		t.Fatal("timeout did not enable autoplay")
	}
	s.mu.Lock()
	retained := s.rooms[id].LastActive
	s.mu.Unlock()
	// A real bot can renew activity; the timeout-controlled human must not.
	if retained < lastActive {
		t.Fatal("activity moved backwards")
	}
}
