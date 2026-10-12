package server

// Failure scenarios (written before the host extension): system deadlines
// incorrectly enable autoplay; events stall without a connected player or bot;
// restart loses a deadline; repeated ticks commit twice; failed event/DB writes
// leak state; eliminated users act or remain trapped in a room; leaving removes
// original seats/history; the last human leaving deletes an unfinished bot game;
// optional actors are incorrectly presented as owing a decision.

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func eventModule() gameapi.Module {
	m := acceptanceModule()
	m.MinPlayers, m.MaxPlayers = 3, 3
	m.Configure = func(n int, raw json.RawMessage) (json.RawMessage, error) {
		if n != 3 {
			return nil, errors.New("three players required")
		}
		return json.RawMessage(`{}`), nil
	}
	m.Start = func(ctx gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Data: json.RawMessage(`{"events":0}`), Actors: []int{0, 1, 2}, Attention: []int{}, SystemDeadline: ctx.Now.Add(time.Hour).UnixMilli()}, nil
	}
	m.OnTimer = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		if ctx.Actor != -1 || ctx.Now.UnixMilli() < match.SystemDeadline {
			return match, errors.New("invalid system context")
		}
		match.Data = json.RawMessage(`{"events":1}`)
		match.SystemDeadline = 0
		match.Eliminated = []int{0}
		match.Actors, match.Attention = []int{1, 2}, []int{1}
		return match, nil
	}
	m.Apply = func(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		if ctx.Actor != 1 || string(raw) != `{"finish":true}` || len(match.Eliminated) != 1 {
			return match, errors.New("invalid finish")
		}
		match.Actors, match.Attention = nil, nil
		match.Result = &gameapi.Result{Ranks: []int{3, 1, 2}, Winners: []int{1}}
		return match, nil
	}
	m.View = func(match gameapi.Match, _ int) (any, error) {
		var data map[string]any
		err := json.Unmarshal(match.Data, &data)
		return data, err
	}
	// System timers must work even for a module with no bot implementation.
	m.BotAction, m.Reclaim = nil, nil
	return m
}

func eventReport(t *testing.T, trace *[]any) {
	t.Helper()
	t.Cleanup(func() {
		out := filepath.Join("..", "..", ".local", "monopoly", "acceptance", "host")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, err := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "host contract fixture, not Monopoly gameplay", "trace": *trace}, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		if err = os.WriteFile(filepath.Join(out, t.Name()+".json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
}

func TestModuleSystemEventAndDepartureE2E(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{eventModule()}}
	files := fstest.MapFS{"index.html": {Data: []byte("Wire Board")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	publishModuleForTest(t, ts.URL, "acceptance")
	t.Cleanup(func() { ts.Close(); s.Close() })
	trace := []any{}
	eventReport(t, &trace)
	a, b, c, viewer := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	for i, client := range []*testClient{a, b, c, viewer} {
		client.register([]string{"事件房主", "事件玩家乙", "事件玩家丙", "事件观众"}[i])
	}
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "系统时钟验收", "capacity": 3}, 201)
	id := r["id"].(string)
	b.command(r, "join", nil, 200)
	c.command(current(a), "join", nil, 200)
	for _, client := range []*testClient{a, b, c} {
		client.command(current(client), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	viewer.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	r = current(a)
	if len(r["game"].(map[string]any)["attention"].([]any)) != 0 {
		t.Fatal("optional actors owe a decision", r)
	}
	a.command(r, "leave", nil, 400)
	saved := current(a)
	trace = append(trace, saved)
	ts.Close()
	s.Close()
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	for _, client := range []*testClient{a, b, c, viewer} {
		client.base = ts.URL
	}
	if !reflect.DeepEqual(saved, current(a)) {
		t.Fatal("restart changed event snapshot")
	}
	// Persisted deadline is authoritative; drive the actual scheduler at that time.
	s.mu.Lock()
	due := s.rooms[id].Module.SystemDeadline
	s.expireModuleTimers(time.UnixMilli(due))
	s.expireModuleTimers(time.UnixMilli(due + 1))
	s.mu.Unlock()
	r = current(a)
	trace = append(trace, r)
	if r["version"].(float64) != saved["version"].(float64)+1 {
		t.Fatal("event did not commit exactly once", r)
	}
	for _, seat := range r["seats"].([]any) {
		if seat.(map[string]any)["autoPlay"] == true {
			t.Fatal("timer enabled autoplay")
		}
	}
	if r["game"].(map[string]any)["data"].(map[string]any)["events"] != float64(1) {
		t.Fatal("event not applied")
	}
	a.command(r, "action", map[string]any{"finish": true}, 400)
	a.post("/api/rooms/"+id, map[string]any{"type": "autoplay", "enabled": true, "nonce": randomID(12), "version": r["version"]}, 400)
	a.command(r, "leave", nil, 200)
	if a.state()["room"] != nil {
		t.Fatal("eliminated user still bound to room")
	}
	other := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "出局后新牌桌", "capacity": 3}, 201)
	if other["id"] == id {
		t.Fatal("did not create independent room")
	}
	r = current(b)
	if len(r["seats"].([]any)) != 3 || r["seats"].([]any)[0].(map[string]any)["left"] != true {
		t.Fatal("lost original roster", r)
	}
	if r["host"] != b.state()["user"].(map[string]any)["id"] {
		t.Fatal("host not transferred")
	}
	b.command(r, "action", map[string]any{"finish": true}, 200)
	r = current(b)
	trace = append(trace, r)
	if r["status"] != "finished" {
		t.Fatal("game did not finish")
	}
	players := r["result"].(map[string]any)["players"].([]any)
	if len(players) != 3 || players[0].(map[string]any)["rank"] != float64(3) {
		t.Fatal("departed player missing from results", r)
	}
	var count int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger").Scan(&count); err != nil || count != 3 {
		t.Fatal("ratings lost/duplicated", count, err)
	}
	if current(viewer)["status"] != "finished" {
		t.Fatal("spectator did not see result")
	}
}

func TestModuleSystemEventAtomicityE2E(t *testing.T) {
	for _, mode := range []string{"callback_error", "database_error", "invalid_deadline", "success_without_bots"} {
		t.Run(mode, func(t *testing.T) {
			m := eventModule()
			original := m.OnTimer
			m.OnTimer = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
				match, err := original(ctx, match)
				if mode == "callback_error" {
					return match, errors.New("test callback failure")
				}
				if mode == "invalid_deadline" {
					match.SystemDeadline = ctx.Now.UnixMilli()
				}
				return match, err
			}
			s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("ok")}})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			ts := httptest.NewServer(s.Handler())
			publishModuleForTest(t, ts.URL, "acceptance")
			defer ts.Close()
			a, b, c := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
			a.register("原子事件甲")
			b.register("原子事件乙")
			c.register("原子事件丙")
			r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "原子事件", "capacity": 3}, 201)
			b.command(r, "join", nil, 200)
			c.command(current(a), "join", nil, 200)
			for _, client := range []*testClient{a, b, c} {
				client.command(current(client), "ready", nil, 200)
			}
			a.command(current(a), "start", nil, 200)
			id := r["id"].(string)
			before := current(a)
			if mode == "database_error" {
				if _, err = s.db.Exec("CREATE TRIGGER fail_event BEFORE INSERT ON actions BEGIN SELECT RAISE(ABORT, 'injected failure'); END"); err != nil {
					t.Fatal(err)
				}
			}
			s.mu.Lock()
			due := s.rooms[id].Module.SystemDeadline
			s.expireModuleTimers(time.UnixMilli(due))
			s.mu.Unlock()
			after := current(a)
			if mode != "success_without_bots" && !reflect.DeepEqual(before, after) {
				t.Fatal("failed event leaked state", after)
			}
			if mode == "success_without_bots" && after["version"].(float64) != before["version"].(float64)+1 {
				t.Fatal("timer depends on bots")
			}
			var raw []byte
			if err = s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", id).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var saved Room
			if err = json.Unmarshal(raw, &saved); err != nil {
				t.Fatal(err)
			}
			if float64(saved.Version) != after["version"] {
				t.Fatal("persisted state diverged")
			}
		})
	}
}

func TestModuleLastHumanDepartureE2E(t *testing.T) {
	m := eventModule()
	m.BotAction = func(_ gameapi.Context, _ gameapi.Match) (json.RawMessage, error) {
		return json.RawMessage(`{"finish":true}`), nil
	}
	m.Reclaim = acceptanceModule().Reclaim
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("ok")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	publishModuleForTest(t, ts.URL, "acceptance")
	defer ts.Close()
	a, viewer := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("最后真人")
	viewer.register("保留观众")
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "无人后继续", "capacity": 3}, 201)
	id := r["id"].(string)
	a.command(r, "add_bot", nil, 200)
	a.command(current(a), "add_bot", nil, 200)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	viewer.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	s.mu.Lock()
	due := s.rooms[id].Module.SystemDeadline
	s.expireModuleTimers(time.UnixMilli(due))
	s.mu.Unlock()
	a.command(current(a), "leave", nil, 200)
	r = current(viewer)
	if r["status"] != "playing" || len(r["seats"].([]any)) != 3 {
		t.Fatal("last departure deleted ongoing match", r)
	}
	s.mu.Lock()
	s.runBots(time.Now().Add(2 * time.Hour))
	s.mu.Unlock()
	r = current(viewer)
	if r["status"] != "finished" || len(r["result"].(map[string]any)["players"].([]any)) != 3 {
		t.Fatal("remaining bots lost history", r)
	}
	trace := []any{r}
	eventReport(t, &trace)
}

func TestModuleSystemEventLiveClockE2E(t *testing.T) {
	m := eventModule()
	start := m.Start
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		match, err := start(ctx, raw)
		match.SystemDeadline = ctx.Now.Add(50 * time.Millisecond).UnixMilli()
		return match, err
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("ok")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	publishModuleForTest(t, ts.URL, "acceptance")
	defer ts.Close()
	a, b, c := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("自然时钟甲")
	b.register("自然时钟乙")
	c.register("自然时钟丙")
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "自然时钟", "capacity": 3}, 201)
	b.command(r, "join", nil, 200)
	c.command(current(a), "join", nil, 200)
	for _, client := range []*testClient{a, b, c} {
		client.command(current(client), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	// No explicit scheduler calls, fake clock changes, bots, or open WebSocket.
	for limit := time.Now().Add(5 * time.Second); time.Now().Before(limit); {
		r = current(a)
		if r["game"].(map[string]any)["data"].(map[string]any)["events"] == float64(1) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if r["game"].(map[string]any)["data"].(map[string]any)["events"] != float64(1) {
		t.Fatal("live scheduler did not deliver event")
	}
	for _, seat := range r["seats"].([]any) {
		if seat.(map[string]any)["autoPlay"] == true {
			t.Fatal("system deadline enabled autoplay")
		}
	}
	trace := []any{r}
	eventReport(t, &trace)
}
