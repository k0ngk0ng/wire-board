package server

// Failure scenarios before no-action bot support: nil is passed to Apply and
// logged as an invalid action; the first idle bot starves a later active bot;
// all-idle rounds keep writing actions, broadcasting versions and refreshing
// activity, so a paused room never expires. All requests use the real HTTP
// lifecycle; the normal clock runs the actual bot scheduler.
import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"
)

func TestModuleIdleBotSchedulingHTTP(t *testing.T) {
	m := acceptanceModule()
	m.MinPlayers, m.MaxPlayers = 3, 3
	m.Configure = func(_ int, _ json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Data: json.RawMessage(`{"count":0}`), Actors: []int{0, 1, 2}, Attention: []int{}}, nil
	}
	m.View = func(match gameapi.Match, _ int) (any, error) {
		v := map[string]any{}
		e := json.Unmarshal(match.Data, &v)
		return v, e
	}
	m.BotAction = func(c gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
		if c.Actor == 2 && string(match.Data) == `{"count":0}` {
			return json.RawMessage(`{"increment":1}`), nil
		}
		return nil, nil
	}
	m.Apply = func(c gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		if c.Actor != 2 || string(raw) != `{"increment":1}` {
			return match, errors.New("idle bot submitted a decision")
		}
		match.Data = json.RawMessage(`{"count":1}`)
		return match, nil
	}
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}
	s, err := New(cfg, fstest.MapFS{"index.html": {Data: []byte("idle bot HTTP")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	publishModuleForTest(t, ts.URL, "acceptance")
	a := newClient(t, ts.URL)
	a.register("暂停调度验证")
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "空闲托管验证", "capacity": 3}, 201)
	id := r["id"].(string)
	a.command(current(a), "add_bot", nil, 200)
	a.command(current(a), "add_bot", nil, 200)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	reached := false
	for until := time.Now().Add(4 * time.Second); time.Now().Before(until); {
		r = current(a)
		if r["game"].(map[string]any)["data"].(map[string]any)["count"] == float64(1) {
			reached = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !reached {
		t.Fatal("idle first bot starved the later actionable bot")
	}
	before := current(a)
	var actionsBefore, actionsAfter int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=?", id).Scan(&actionsBefore); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	activeBefore := s.rooms[id].LastActive
	s.mu.Unlock()
	time.Sleep(2300 * time.Millisecond)
	after := current(a)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("idle bot rewrote/broadcast room", before["version"], after["version"])
	}
	if err = s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=?", id).Scan(&actionsAfter); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	activeAfter := s.rooms[id].LastActive
	s.mu.Unlock()
	if actionsBefore != actionsAfter || activeBefore != activeAfter {
		t.Fatal("idle bot mutated action journal or renewed room activity", actionsBefore, actionsAfter, activeBefore, activeAfter)
	}
	out := filepath.Join("..", "..", ".local", "starfort", "http")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "before": before, "after": after, "actionsBefore": actionsBefore, "actionsAfter": actionsAfter, "lastActiveBefore": activeBefore, "lastActiveAfter": activeAfter}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "idle-bot-scheduler.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
