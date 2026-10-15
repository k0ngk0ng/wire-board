package server

// Failure scenarios written before the async search implementation:
// search holds the global lock; workers receive hidden state; stale or duplicate
// results overwrite a manual move/new match; cancelling control leaves search
// alive; errors/panics/invalid moves/deadlines stall a bot; ignored cancellation
// spawns unbounded workers; timeout takeover cannot be reclaimed; shutdown or
// restore applies an old task; idle actors starve another actor or write logs.
// Real HTTP lifecycle and the production timer run these tests. No game rule
// suites are required for this platform-only contract.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func asyncFixture(think func(context.Context, json.RawMessage) (json.RawMessage, error)) gameapi.Module {
	m := acceptanceModule()
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Schema: 1, Data: json.RawMessage(`{"count":1,"secret":"private-test-card"}`), Actors: []int{1}, TimeoutActors: []int{1}, Deadline: c.Now.Add(120 * time.Second).UnixMilli()}, nil
	}
	m.BotSearch = &gameapi.BotSearch{
		Budget: 2 * time.Second,
		Input: func(c gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
			view, err := m.View(match, c.Actor)
			if err != nil {
				return nil, err
			}
			return json.Marshal(view)
		},
		Think: think,
	}
	return m
}

type asyncTable struct {
	s      *Server
	ts     *httptest.Server
	a      *testClient
	cfg    Config
	closed bool
}

func newAsyncTable(t *testing.T, m gameapi.Module) *asyncTable {
	t.Helper()
	f := &asyncTable{cfg: Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}}
	var err error
	f.s, err = New(f.cfg, fstest.MapFS{"index.html": {Data: []byte("async platform acceptance")}})
	if err != nil {
		t.Fatal(err)
	}
	f.ts = httptest.NewServer(f.s.Handler())
	t.Cleanup(func() {
		if !f.closed {
			f.ts.Close()
			f.s.Close()
		}
	})
	publishModuleForTest(t, f.ts.URL, m.ID)
	f.a = newClient(t, f.ts.URL)
	f.a.register("异步电脑验收")
	return f
}

func (f *asyncTable) start(t *testing.T) map[string]any {
	t.Helper()
	f.a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "后台选步", "capacity": 2}, 201)
	f.a.command(current(f.a), "add_bot", nil, 200)
	f.a.command(current(f.a), "ready", nil, 200)
	f.a.command(current(f.a), "start", nil, 200)
	return current(f.a)
}

func asyncAwait[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("async scheduler did not reach expected event")
	}
	var zero T
	return zero
}

func asyncCount(r map[string]any) float64 {
	return r["game"].(map[string]any)["data"].(map[string]any)["count"].(float64)
}
func awaitAsyncCount(t *testing.T, c *testClient, want float64) map[string]any {
	t.Helper()
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until); {
		r := current(c)
		if asyncCount(r) == want {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("want count %v; got %v", want, current(c))
	return nil
}
func asyncEvidence(t *testing.T, detail any) {
	t.Helper()
	dir := filepath.Join("..", "..", ".local", "chess-family", "async-bot")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "detail": detail}, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestModuleAsyncBotResponsiveHTTP(t *testing.T) {
	started := make(chan json.RawMessage, 1)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	m := asyncFixture(func(ctx context.Context, in json.RawMessage) (json.RawMessage, error) {
		started <- append(json.RawMessage(nil), in...)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return json.RawMessage(`{"increment":1}`), nil
	})
	f := newAsyncTable(t, m)
	b := newClient(t, f.ts.URL)
	b.register("另一张桌")
	before := f.start(t)
	input := asyncAwait(t, started)
	if strings.Contains(string(input), "private-test-card") {
		t.Fatal("hidden state reached worker", string(input))
	}
	f.a.client.Timeout = 500 * time.Millisecond
	start := time.Now()
	unchanged := current(f.a)
	elapsed := time.Since(start)
	if asyncCount(unchanged) != 1 || unchanged["version"] != before["version"] {
		t.Fatal("search changed state before a result")
	}
	// An independent room must also remain writable during search.
	b.client.Timeout = 500 * time.Millisecond
	b.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "不被搜索阻塞", "capacity": 2}, 201)
	once.Do(func() { close(release) })
	after := awaitAsyncCount(t, f.a, 2)
	if after["version"].(float64) != before["version"].(float64)+1 {
		t.Fatal("duplicate result or missing action version")
	}
	asyncEvidence(t, map[string]any{"before": before, "after": after, "input": json.RawMessage(input), "stateRequestMs": elapsed.Milliseconds()})
}

func TestModuleAsyncBotManualTakeoverHTTP(t *testing.T) {
	started := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 1)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	m := asyncFixture(func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		started <- struct{}{}
		<-ctx.Done()
		cancelled <- struct{}{}
		<-release
		return json.RawMessage(`{"increment":1}`), nil // Deliberately late, ignores cancellation result.
	})
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Data: json.RawMessage(`{"count":0}`), Actors: []int{0}, TimeoutActors: []int{0}, Deadline: c.Now.Add(120 * time.Second).UnixMilli()}, nil
	}
	f := newAsyncTable(t, m)
	r := f.start(t)
	control := func(enabled bool) {
		r = current(f.a)
		f.a.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "autoplay", "version": r["version"], "nonce": randomID(12), "enabled": enabled}, 200)
	}
	control(true)
	asyncAwait(t, started)
	control(false)
	asyncAwait(t, cancelled)
	r = current(f.a)
	f.a.command(r, "action", map[string]int{"increment": 1}, 200)
	once.Do(func() { close(release) })
	// The bot in seat 1 may now search. Close before it can make another action;
	// an old result must neither overwrite the human move nor survive rematch.
	f.a.command(current(f.a), "close", nil, 200)
	f.a.command(current(f.a), "rematch", nil, 200)
	after := current(f.a)
	if after["status"] != "waiting" {
		t.Fatal("late search altered rematch", after)
	}
	asyncEvidence(t, after)
}

func TestModuleAsyncBotFallbackHTTP(t *testing.T) {
	for _, mode := range []string{"error", "panic", "invalid_json", "illegal_move", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			m := asyncFixture(func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
				switch mode {
				case "error":
					return nil, errors.New("engine unavailable")
				case "panic":
					panic("engine panic")
				case "invalid_json":
					return json.RawMessage(`bad json`), nil
				case "illegal_move":
					return json.RawMessage(`{"increment":99}`), nil
				default:
					<-ctx.Done()
					return nil, ctx.Err()
				}
			})
			m.BotSearch.Budget = 30 * time.Millisecond
			f := newAsyncTable(t, m)
			before := f.start(t)
			after := awaitAsyncCount(t, f.a, 2)
			if after["version"].(float64) != before["version"].(float64)+1 {
				t.Fatal("failed search created extra versions")
			}
			asyncEvidence(t, after)
		})
	}
}

func TestModuleAsyncBotBoundedWorkersHTTP(t *testing.T) {
	var active, peak atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	m := asyncFixture(func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		<-release // Broken plugin; a timeout must not create unlimited workers.
		return json.RawMessage(`{"increment":1}`), nil
	})
	m.BotSearch.Budget = 30 * time.Millisecond
	f := newAsyncTable(t, m)
	clients := []*testClient{f.a}
	f.start(t)
	for i := 1; i < 6; i++ {
		c := newClient(t, f.ts.URL)
		c.register(fmt.Sprintf("并发搜索%d", i))
		clients = append(clients, c)
		c.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "并发", "capacity": 2}, 201)
		for _, cmd := range []string{"add_bot", "ready", "start"} {
			c.command(current(c), cmd, nil, 200)
		}
	}
	for _, c := range clients {
		awaitAsyncCount(t, c, 2)
	}
	if n := peak.Load(); n < 1 || n > 4 {
		t.Fatal("worker bound broken", n)
	}
	once.Do(func() { close(release) })
	asyncEvidence(t, map[string]any{"rooms": len(clients), "peakWorkers": peak.Load(), "allAdvancedWithLegalFallback": true})
}

func TestModuleAsyncBotExpiredReclaimHTTP(t *testing.T) {
	started := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 1)
	m := asyncFixture(func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		started <- struct{}{}
		<-ctx.Done()
		cancelled <- struct{}{}
		return nil, ctx.Err()
	})
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Data: json.RawMessage(`{"count":0}`), Actors: []int{0}, TimeoutActors: []int{0}, Deadline: c.Now.Add(150 * time.Millisecond).UnixMilli()}, nil
	}
	f := newAsyncTable(t, m)
	f.start(t)
	asyncAwait(t, started)
	r := current(f.a)
	if !r["seats"].([]any)[0].(map[string]any)["autoPlay"].(bool) {
		t.Fatal("timeout takeover not visible during search")
	}
	f.a.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "autoplay", "version": r["version"], "nonce": randomID(12), "enabled": false}, 200)
	asyncAwait(t, cancelled)
	after := current(f.a)
	if asyncCount(after) != 0 || after["turnDeadline"].(float64) <= float64(time.Now().UnixMilli()) {
		t.Fatal("reclaim lost board or expired immediately", after)
	}
	asyncEvidence(t, map[string]any{"duringSearch": r, "afterReclaim": after})
}

func TestModuleAsyncBotShutdownRestoreHTTP(t *testing.T) {
	started := make(chan struct{}, 2)
	cancelled := make(chan struct{}, 2)
	var calls atomic.Int32
	m := asyncFixture(func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		if calls.Add(1) == 1 {
			started <- struct{}{}
			<-ctx.Done()
			cancelled <- struct{}{}
			return nil, ctx.Err()
		}
		return json.RawMessage(`{"increment":1}`), nil
	})
	f := newAsyncTable(t, m)
	before := f.start(t)
	asyncAwait(t, started)
	f.ts.Close()
	if err := f.s.Close(); err != nil {
		t.Fatal(err)
	}
	f.closed = true
	asyncAwait(t, cancelled)
	s, err := New(f.cfg, fstest.MapFS{"index.html": {Data: []byte("restored async")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	f.a.base = ts.URL
	f.a.post("/api/login", map[string]string{"name": "异步电脑验收", "password": "test-password-123"}, 200)
	after := awaitAsyncCount(t, f.a, 2)
	if after["id"] != before["id"] || after["version"].(float64) != before["version"].(float64)+1 {
		t.Fatal("restore replayed/restarted match", before, after)
	}
	asyncEvidence(t, map[string]any{"before": before, "after": after, "calls": calls.Load()})
}

func TestModuleAsyncBotIdleActorsHTTP(t *testing.T) {
	m := asyncFixture(func(_ context.Context, raw json.RawMessage) (json.RawMessage, error) {
		var in struct{ Actor, Count int }
		_ = json.Unmarshal(raw, &in)
		if in.Actor == 2 && in.Count == 0 {
			return json.RawMessage(`{"increment":1}`), nil
		}
		return nil, nil
	})
	m.MinPlayers, m.MaxPlayers = 3, 3
	m.Configure = func(int, json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
	m.Start = func(gameapi.Context, json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Data: json.RawMessage(`{"count":0}`), Actors: []int{1, 2}}, nil
	}
	m.BotSearch.Input = func(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
		var v map[string]any
		_ = json.Unmarshal(m.Data, &v)
		v["actor"] = c.Actor
		return json.Marshal(v)
	}
	m.Apply = func(c gameapi.Context, m gameapi.Match, a json.RawMessage) (gameapi.Match, error) {
		if c.Actor != 2 || string(a) != `{"increment":1}` {
			return m, errors.New("wrong actor")
		}
		m.Data = json.RawMessage(`{"count":1}`)
		return m, nil
	}
	f := newAsyncTable(t, m)
	f.a.post("/api/rooms", map[string]any{"kind": "acceptance", "capacity": 3, "name": "空闲异步席位"}, 201)
	for _, cmd := range []string{"add_bot", "add_bot", "ready", "start"} {
		f.a.command(current(f.a), cmd, nil, 200)
	}
	r := awaitAsyncCount(t, f.a, 1)
	time.Sleep(2200 * time.Millisecond)
	after := current(f.a)
	if r["version"] != after["version"] {
		t.Fatal("idle workers rewrote the room", r, after)
	}
	asyncEvidence(t, map[string]any{"before": r, "afterIdle": after})
}

// A timeout control commit followed by a rejected move transaction must not
// publish an aliased candidate before its own transaction succeeds.
func TestModuleAsyncBotTransactionHTTP(t *testing.T) {
	m := asyncFixture(func(context.Context, json.RawMessage) (json.RawMessage, error) { return nil, nil })
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Data: json.RawMessage(`{"count":1}`), Actors: []int{0}, TimeoutActors: []int{0}, Deadline: c.Now.Add(150 * time.Millisecond).UnixMilli()}, nil
	}
	m.BotSearch.Input = func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { return nil, errors.New("use fallback") }
	f := newAsyncTable(t, m)
	_, err := f.s.db.Exec(`CREATE TRIGGER fail_async_move BEFORE UPDATE ON rooms WHEN json_extract(NEW.snapshot,'$.module.data.count') = 2 BEGIN SELECT RAISE(ABORT,'simulated move commit failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	f.start(t)
	var r map[string]any
	for until := time.Now().Add(4 * time.Second); time.Now().Before(until); {
		r = current(f.a)
		if r["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var persisted int
	if err = f.s.db.QueryRow(`SELECT json_extract(snapshot,'$.module.data.count') FROM rooms WHERE id=?`, r["id"]).Scan(&persisted); err != nil {
		t.Fatal(err)
	}
	if asyncCount(r) != 1 || persisted != 1 {
		t.Fatal("failed move leaked out of transaction", asyncCount(r), persisted)
	}
	if _, err = f.s.db.Exec(`DROP TRIGGER fail_async_move`); err != nil {
		t.Fatal(err)
	}
	after := awaitAsyncCount(t, f.a, 2)
	asyncEvidence(t, map[string]any{"duringFailure": r, "persisted": persisted, "afterRecovery": after})
}
