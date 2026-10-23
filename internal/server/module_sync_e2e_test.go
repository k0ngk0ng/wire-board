package server

// Before implementation: unauthenticated/foreign-room reads leak checkpoints;
// spectators receive player-private data; snapshot calls mutate match/version;
// a module callback shares backing storage with the active authoritative room.
import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestModuleSyncSnapshotE2E(t *testing.T) {
	m := acceptanceModule()
	view := m.View
	m.SyncSnapshot = func(match gameapi.Match, seat int) (any, error) {
		value, e := view(match, seat)
		// Host must pass a detached snapshot even to a buggy module callback.
		match.Data[0] = '!'
		match.Actors[0] = 31
		return map[string]any{"viewer": seat, "value": value}, e
	}
	s, e := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("Wire Board")}})
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	publishModuleForTest(t, ts.URL, "acceptance")
	a, b, watcher, outsider := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	for i, c := range []*testClient{a, b, watcher, outsider} {
		c.register([]string{"同步甲", "同步乙", "同步观众", "同步外部"}[i])
	}
	room := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "恢复入口", "capacity": 2}, 201)
	id := room["id"].(string)
	b.command(room, "join", nil, 200)
	for _, c := range []*testClient{a, b} {
		c.command(current(c), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	route := "/api/rooms/" + id + "/sync"
	unauth := newClient(t, ts.URL)
	if status, _ := unauth.request(http.MethodGet, route, nil); status != 401 {
		t.Fatal("anonymous checkpoint", status)
	}
	if status, _ := outsider.request(http.MethodGet, route, nil); status != 403 {
		t.Fatal("foreign checkpoint", status)
	}
	watcher.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	before := current(a)
	trace := []any{}
	for i, c := range []*testClient{a, b, watcher} {
		status, result := c.request(http.MethodGet, route, nil)
		if status != 200 {
			t.Fatal(status, result)
		}
		if result["roomId"] != id || result["version"] != before["version"] || result["serverInstance"] != s.instance {
			t.Fatal("snapshot identity", result)
		}
		data := result["data"].(map[string]any)
		secret := data["value"].(map[string]any)["secret"]
		if (i == 0) != (secret == "private-test-card") {
			t.Fatal("viewer permission lost", result)
		}
		if i == 2 && data["viewer"] != float64(-1) {
			t.Fatal("observer assigned a controller")
		}
		trace = append(trace, result)
	}
	after := current(a)
	if before["version"] != after["version"] || !reflect.DeepEqual(before["game"], after["game"]) {
		t.Fatal("read-only checkpoint changed match")
	}
	s.mu.Lock()
	disabled := s.modules["acceptance"]
	disabled.SyncSnapshot = nil
	s.modules["acceptance"] = disabled
	s.mu.Unlock()
	if status, _ := a.request(http.MethodGet, route, nil); status != 404 {
		t.Fatal("unsupported snapshot endpoint", status)
	}
	out := filepath.Join("..", "..", ".local", "acceptance", "module-sync")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	data, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "test": t.Name(), "trace": trace}, "", "  ")
	if e = os.WriteFile(filepath.Join(out, "result.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
}
