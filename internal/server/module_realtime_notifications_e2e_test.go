package server

// Failure scenarios written before notification scoping: continuous frames or
// human controls wake the lobby and unrelated games; same-table viewers or a
// player's second tab stop receiving changes; leaving spectators retain frame
// subscriptions; finished races, closure, and autoplay stop refreshing the
// global room list; ordinary games lose their existing global notifications.
// Every connection below is an authenticated real WebSocket, not a host channel.
import (
	"context"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type realtimeSocketNotice struct {
	name string
	raw  string
	err  error
}

// Fresh sockets consume their initial change before the stimulus. Keeping one
// pending read per socket lets us prove silence without a Read timeout closing
// an otherwise reusable socket. The silence observation is always 200 ms after
// all required notifications arrive, with the production clock stopped.
func assertRealtimeNotices(t *testing.T, clients map[string]*testClient, expected []string, mutate func()) map[string]string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	notices := make(chan realtimeSocketNotice, len(clients))
	for name, client := range clients {
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(client.base, "http")+"/api/ws", &websocket.DialOptions{HTTPClient: client.client})
		if err != nil {
			t.Fatal(err)
		}
		defer conn.CloseNow()
		_, raw, err := conn.Read(ctx)
		if err != nil || string(raw) != `{"type":"changed"}` {
			t.Fatal("initial WS event changed", name, string(raw), err)
		}
		go func(name string, conn *websocket.Conn) {
			_, raw, err := conn.Read(ctx)
			notices <- realtimeSocketNotice{name: string(name), raw: string(raw), err: err}
		}(name, conn)
	}
	mutate()
	want := map[string]bool{}
	for _, name := range expected {
		want[name] = true
	}
	got := map[string]string{}
	until := time.NewTimer(2 * time.Second)
	defer until.Stop()
	for len(got) < len(want) {
		select {
		case notice := <-notices:
			if notice.err != nil || notice.raw != `{"type":"changed"}` {
				t.Fatal("invalid notification", notice.name, notice.raw, notice.err)
			}
			if !want[notice.name] {
				t.Fatal("unrelated socket received realtime wakeup", notice.name)
			}
			got[notice.name] = notice.raw
		case <-until.C:
			t.Fatal("missing scoped notification", want, got)
		}
	}
	silence := time.NewTimer(200 * time.Millisecond)
	defer silence.Stop()
	select {
	case notice := <-notices:
		t.Fatal("unrelated socket received delayed wakeup", notice.name, notice.raw, notice.err)
	case <-silence.C:
	}
	return got
}

func realtimeNotificationClients(t *testing.T, f *realtimeTable) map[string]*testClient {
	t.Helper()
	viewer, lobby, other := newClient(t, f.ts.URL), newClient(t, f.ts.URL), newClient(t, f.ts.URL)
	viewer.register("同桌观战通知")
	lobby.register("大厅通知隔离")
	other.register("其他游戏通知")
	viewer.post("/api/rooms/"+f.id+"/watch", map[string]any{}, 200)
	other.post("/api/rooms", map[string]any{"kind": "splendor", "name": "不应收到赛车输入", "capacity": 2}, 201)
	return map[string]*testClient{"seat": f.a, "seat_second_tab": f.a, "spectator": viewer, "lobby": lobby, "other_room": other}
}

func TestModuleRealtimeWebSocketScopesHTTP(t *testing.T) {
	trace := []any{}
	realtimeReport(t, &trace)
	f := newRealtimeTable(t, realtimeFixture(true, time.Hour, 3, ""), false)
	clients := realtimeNotificationClients(t, f)
	same := []string{"seat", "seat_second_tab", "spectator"}
	all := []string{"seat", "seat_second_tab", "spectator", "lobby", "other_room"}
	due := f.s.rooms[f.id].Module.SystemDeadline
	trace = append(trace, map[string]any{"event": "frame", "notifications": assertRealtimeNotices(t, clients, same, func() { f.frame(time.UnixMilli(due)) })})
	trace = append(trace, map[string]any{"event": "human_action", "notifications": assertRealtimeNotices(t, clients, same, func() { f.a.command(current(f.a), "action", map[string]any{"drive": 1}, 200) })})
	trace = append(trace, map[string]any{"event": "autoplay", "notifications": assertRealtimeNotices(t, clients, all, func() { setAutoPlay(f.a, current(f.a), true, 200) })})
	trace = append(trace, map[string]any{"event": "cancel_autoplay", "notifications": assertRealtimeNotices(t, clients, all, func() { setAutoPlay(f.a, current(f.a), false, 200) })})
	trace = append(trace, map[string]any{"event": "spectator_leave", "notifications": assertRealtimeNotices(t, clients, all, func() { clients["spectator"].post("/api/rooms/"+f.id+"/watch", map[string]any{"leave": true}, 200) })})
	trace = append(trace, map[string]any{"event": "frame_after_spectator_left", "notifications": assertRealtimeNotices(t, clients, []string{"seat", "seat_second_tab"}, func() { f.frame(time.UnixMilli(due + 100)) })})
	trace = append(trace, map[string]any{"event": "finished_frame", "notifications": assertRealtimeNotices(t, clients, all, func() { f.frame(time.UnixMilli(due + 200)) })})
	if current(f.a)["status"] != "finished" {
		t.Fatal("notification fixture did not finish")
	}
}

func TestModuleRealtimeCloseAndOrdinaryWebSocketScopesHTTP(t *testing.T) {
	trace := []any{}
	realtimeReport(t, &trace)
	f := newRealtimeTable(t, realtimeFixture(true, time.Hour, 0, ""), false)
	clients := realtimeNotificationClients(t, f)
	all := []string{"seat", "seat_second_tab", "spectator", "lobby", "other_room"}
	trace = append(trace, map[string]any{"event": "close", "notifications": assertRealtimeNotices(t, clients, all, func() { f.a.command(current(f.a), "close", nil, 200) })})
	closed := current(f.a)
	trace = append(trace, map[string]any{"event": "closed_frame", "notifications": assertRealtimeNotices(t, clients, nil, func() { f.frame(time.UnixMilli(f.s.rooms[f.id].Module.SystemDeadline)) })})
	if !reflect.DeepEqual(closed, current(f.a)) {
		t.Fatal("closed frame mutated room")
	}
	// Register the identical host fixture without Realtime: ordinary action and
	// timer publication continue to wake every logged-in client.
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{realtimeFixture(false, time.Hour, 0, "")}}
	s, err := New(cfg, fstest.MapFS{"index.html": {Data: []byte("ordinary notifications")}})
	if err != nil {
		t.Fatal(err)
	}
	s.cancel()
	<-s.done
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	publishModuleForTest(t, ts.URL, "realtime_fixture")
	a := newClient(t, ts.URL)
	a.register("普通模块通知")
	r := a.post("/api/rooms", map[string]any{"kind": "realtime_fixture", "name": "普通模块", "capacity": 4}, 201)
	for i := 0; i < 3; i++ {
		a.command(current(a), "add_bot", nil, 200)
	}
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	g := &realtimeTable{s: s, ts: ts, a: a, id: r["id"].(string)}
	ordinary := realtimeNotificationClients(t, g)
	trace = append(trace, map[string]any{"event": "ordinary_action", "notifications": assertRealtimeNotices(t, ordinary, all, func() { a.command(current(a), "action", map[string]any{"drive": 1}, 200) })})
	trace = append(trace, map[string]any{"event": "ordinary_timer", "notifications": assertRealtimeNotices(t, ordinary, all, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.expireModuleTimers(time.UnixMilli(s.rooms[g.id].Module.SystemDeadline))
	})})
}
