package server

// Failure scenarios precede implementation in docs/research/game-loading/PLAN.md.
// These tests use authenticated HTTP and persisted SQLite. Time acceleration
// calls the actual platform scheduler; no game outcome or ready bit is injected.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"
)

func loadingTestModule() gameapi.Module {
	m := eventModule()
	m.Realtime = true
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		b, _ := json.Marshal(map[string]any{"ticks": 0, "at": c.Now.UnixMilli()})
		return gameapi.Match{Schema: 1, Data: b, Actors: []int{0, 1, 2}, Attention: []int{}, TimeoutActors: []int{0}, Deadline: c.Now.Add(30 * time.Second).UnixMilli(), SystemDeadline: c.Now.Add(100 * time.Millisecond).UnixMilli()}, nil
	}
	m.OnTimer = func(c gameapi.Context, s gameapi.Match) (gameapi.Match, error) {
		var d map[string]int64
		_ = json.Unmarshal(s.Data, &d)
		d["ticks"]++
		d["at"] = c.Now.UnixMilli()
		s.Data, _ = json.Marshal(d)
		s.SystemDeadline = c.Now.Add(100 * time.Millisecond).UnixMilli()
		return s, nil
	}
	m.BotAction = func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, nil }
	m.Reclaim = func(_ gameapi.Context, s gameapi.Match) (gameapi.Match, error) { return s, nil }
	m.View = func(s gameapi.Match, _ int) (any, error) {
		var d map[string]any
		e := json.Unmarshal(s.Data, &d)
		return d, e
	}
	return m
}
func loadingCommand(c *testClient, room map[string]any, kind, token, build, client string) (int, map[string]any) {
	return c.request("POST", "/api/rooms/"+room["id"].(string), map[string]any{"type": kind, "version": room["version"], "nonce": randomID(12), "loadToken": token, "loadBuild": build, "loadClient": client})
}
func loadInfo(t *testing.T, c *testClient) map[string]any {
	t.Helper()
	r := current(c)
	v, ok := r["loading"].(map[string]any)
	if !ok {
		t.Fatalf("missing load gate: %+v", r)
	}
	return v
}
func TestClientLoadingAllHumansBeforeRealtimeClock(t *testing.T) {
	m := loadingTestModule()
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}, ClientLoading: true}
	files := fstest.MapFS{"index.html": {Data: []byte("loading-build-one")}}
	s, e := New(cfg, files)
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	defer func() { ts.Close(); s.Close() }()
	publishModuleForTest(t, ts.URL, m.ID)
	a, b, watch := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("加载甲")
	b.register("加载乙")
	watch.register("加载观众")
	r := a.post("/api/rooms", map[string]any{"kind": m.ID, "name": "等待资源", "capacity": 3}, 201)
	b.command(r, "join", nil, 200)
	a.command(current(a), "add_bot", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	gate := loadInfo(t, a)
	token, build := gate["token"].(string), gate["build"].(string)
	ready := gate["ready"].([]any)
	if ready[0] != false || ready[1] != false || ready[2] != true {
		t.Fatal("wrong initial participants", ready)
	}
	before := current(a)["game"].(map[string]any)["data"].(map[string]any)
	at := before["at"]
	time.Sleep(250 * time.Millisecond)
	s.mu.Lock()
	s.expireRealtimeModuleTimers(time.Now().Add(time.Minute))
	s.runBots(time.Now().Add(time.Minute))
	s.mu.Unlock()
	now := current(a)
	if now["game"].(map[string]any)["data"].(map[string]any)["ticks"] != float64(0) {
		t.Fatal("loading advanced simulation")
	}
	if now["turnDeadline"] != float64(0) {
		t.Fatal("loading consumes action time", now["turnDeadline"])
	}
	for _, c := range []*testClient{a, b} {
		code, _ := loadingCommand(c, current(c), "load_begin", token, build, c.state()["user"].(map[string]any)["id"].(string))
		if code != 200 {
			t.Fatal(code)
		}
	}
	ca := a.state()["user"].(map[string]any)["id"].(string)
	cb := b.state()["user"].(map[string]any)["id"].(string)
	if code, _ := loadingCommand(a, current(a), "load_ready", token, build, ca); code != 200 {
		t.Fatal(code)
	}
	if loadInfo(t, a)["ready"].([]any)[1] != false {
		t.Fatal("first human started everyone")
	}
	watch.post("/api/rooms/"+current(a)["id"].(string)+"/watch", map[string]any{}, 200)
	if code, _ := loadingCommand(watch, current(a), "load_ready", token, build, "watch"); code == 200 {
		t.Fatal("observer acknowledged player")
	}
	if code, _ := loadingCommand(b, current(b), "load_ready", token, "old-build", cb); code == 200 {
		t.Fatal("wrong build acknowledged")
	}
	frozen := current(a)
	if frozen["game"].(map[string]any)["data"].(map[string]any)["at"] != at {
		t.Fatal("waiting mutated game time")
	}

	if _, err := s.db.Exec("CREATE TRIGGER reject_loading_save BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT, 'loading save failure'); END"); err != nil {
		t.Fatal(err)
	}
	if code, _ := loadingCommand(b, current(b), "load_ready", token, build, cb); code != 500 {
		t.Fatal("failed save acknowledged readiness", code)
	}
	if loadInfo(t, a)["ready"].([]any)[1] != false {
		t.Fatal("failed commit released gate")
	}
	if _, err := s.db.Exec("DROP TRIGGER reject_loading_save"); err != nil {
		t.Fatal(err)
	}
	if code, _ := loadingCommand(b, current(b), "load_ready", token, build, cb); code != 200 {
		t.Fatal(code)
	}
	resumed := current(a)
	if resumed["loading"] != nil {
		t.Fatal("last human did not release gate")
	}
	if resumed["clockOffsetMs"].(float64) < 200 {
		t.Fatal("load wait not removed from game clock")
	}
	deadline := resumed["turnDeadline"].(float64)
	if deadline != 0 {
		t.Fatal("realtime games must not acquire a turn deadline")
	}
	if code, _ := loadingCommand(a, resumed, "load_ready", token, build, ca); code != 200 {
		t.Fatal("duplicate confirmation not idempotent")
	}
	if current(a)["turnDeadline"] != deadline {
		t.Fatal("duplicate acknowledgement reset deadline")
	}
	end := time.Now().Add(time.Second)
	for time.Now().Before(end) {
		d := current(a)["game"].(map[string]any)["data"].(map[string]any)
		if d["ticks"].(float64) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("clock did not resume")
}
func TestClientLoadingReconnectRestoreAndOldClient(t *testing.T) {
	m := loadingTestModule()
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}, ClientLoading: true}
	files := fstest.MapFS{"index.html": {Data: []byte("load-v1")}}
	s, e := New(cfg, files)
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	defer func() { ts.Close(); s.Close() }()
	publishModuleForTest(t, ts.URL, m.ID)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("资源重连甲")
	b.register("资源重连乙")
	r := a.post("/api/rooms", map[string]any{"kind": m.ID, "name": "资源重连", "capacity": 3}, 201)
	b.command(r, "join", nil, 200)
	a.command(current(a), "add_bot", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	g := loadInfo(t, a)
	token, build := g["token"].(string), g["build"].(string)
	for _, op := range []struct{ kind, client string }{{"load_begin", "old"}, {"load_ready", "old"}, {"load_begin", "new"}} {
		if code, _ := loadingCommand(a, current(a), op.kind, token, build, op.client); code != 200 {
			t.Fatal(code)
		}
	}
	if loadInfo(t, a)["ready"].([]any)[0] != false {
		t.Fatal("new client inherited obsolete acknowledgement")
	}
	if code, _ := loadingCommand(a, current(a), "load_ready", token, build, "old"); code == 200 {
		t.Fatal("old client won after reconnect")
	}
	ts.Close()
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	files["index.html"].Data = []byte("load-v2")
	s, e = New(cfg, files)
	if e != nil {
		t.Fatal(e)
	}
	ts = httptest.NewServer(s.Handler())
	a.base = ts.URL
	b.base = ts.URL
	next := loadInfo(t, a)
	if next["token"] == token || next["build"] == build || next["ready"].([]any)[0] != false || next["ready"].([]any)[2] != true {
		t.Fatal("deployment did not rebind gate", next)
	}
	if next["frozenAt"] != g["frozenAt"] {
		t.Fatal("restore advanced frozen game time")
	}
	a.command(current(a), "close", nil, 200)
	if current(a)["loading"] != nil {
		t.Fatal("closed room retained loading barrier")
	}
}

func TestClientLoadingTurnAndLegacyClocks(t *testing.T) {
	for _, kind := range []string{"acceptance", "splendor"} {
		t.Run(kind, func(t *testing.T) {
			m := loadingTestModule()
			m.Realtime = false
			cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}, ClientLoading: true}
			s, e := New(cfg, fstest.MapFS{"index.html": {Data: []byte("turn-load")}})
			if e != nil {
				t.Fatal(e)
			}
			ts := httptest.NewServer(s.Handler())
			defer ts.Close()
			defer s.Close()
			publishModuleForTest(t, ts.URL, m.ID)
			a := newClient(t, ts.URL)
			a.register("回合加载" + kind)
			n := 2
			if kind == m.ID {
				n = 3
			}
			a.post("/api/rooms", map[string]any{"kind": kind, "name": "回合等待", "capacity": n}, 201)
			for i := 1; i < n; i++ {
				a.command(current(a), "add_bot", nil, 200)
			}
			a.command(current(a), "ready", nil, 200)
			a.command(current(a), "start", nil, 200)
			g := loadInfo(t, a)
			r := current(a)
			if code, _ := a.request("POST", "/api/rooms/"+r["id"].(string), map[string]any{"type": "action", "version": r["version"], "nonce": randomID(12), "action": map[string]any{"type": "pass"}}); code == 200 {
				t.Fatal("loading accepted action")
			}
			s.mu.Lock()
			s.expireSetups(time.Now().Add(time.Hour))
			s.expireModuleTimersAtCadence(time.Now().Add(time.Hour), false)
			s.runBots(time.Now().Add(time.Hour))
			s.mu.Unlock()
			if current(a)["version"] != r["version"] {
				t.Fatal("loading scheduler mutated room")
			}
			time.Sleep(150 * time.Millisecond)
			for _, op := range []string{"load_begin", "load_ready"} {
				if code, body := loadingCommand(a, current(a), op, g["token"].(string), g["build"].(string), "turn-client"); code != 200 {
					t.Fatal(code, body)
				}
			}
			next := current(a)
			if next["loading"] != nil || next["turnDeadline"].(float64) < float64(time.Now().Add(29*time.Second).UnixMilli()) {
				t.Fatal("lost action time", next)
			}
		})
	}
}
