package server

// Before save-control implementation: realtime frames invalidate every vote;
// one vote saves unilaterally; failed pause persists; refusing cannot release
// the temporary pause; save changes gameplay; resume retains save-only pause.

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestSavedGameRealtimeConsentHTTP(t *testing.T) {
	m := savedGameFixture()
	m.ID = "save-control-fixture"
	m.Realtime = true
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Schema: 1, Unrated: true, Data: json.RawMessage(`{"count":0}`), Actors: []int{0, 1}, Attention: []int{}, SystemDeadline: c.Now.Add(50 * time.Millisecond).UnixMilli()}, nil
	}
	m.OnTimer = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
		var data map[string]int
		json.Unmarshal(g.Data, &data)
		data["count"]++
		g.Data, _ = json.Marshal(data)
		g.SystemDeadline = c.Now.Add(50 * time.Millisecond).UnixMilli()
		return g, nil
	}
	m.SaveControl = func(c gameapi.Context, g gameapi.Match, saving bool) (gameapi.Match, error) {
		g.Paused = saving
		g.SystemDeadline = 0
		if !saving {
			g.SystemDeadline = c.Now.Add(50 * time.Millisecond).UnixMilli()
		}
		g.Attention = []int{}
		return g, nil
	}
	m.ResumeSaved = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) { return m.SaveControl(c, g, false) }
	m.Restore = func(_ gameapi.Context, g gameapi.Match) (gameapi.Match, error) { return g, nil }
	m.Reclaim = func(_ gameapi.Context, g gameapi.Match) (gameapi.Match, error) { return g, nil }
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}
	s, err := New(cfg, fstest.MapFS{"index.html": {Data: []byte("save control")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() {
		ts.Close()
		if s != nil {
			s.Close()
		}
	})
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("保存发起者")
	b.register("保存队友")
	r := a.post("/api/rooms", map[string]any{"kind": m.ID, "name": "实时保存确认", "capacity": 2}, 201)
	b.command(r, "join", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	id := r["id"].(string)
	stale := current(a)
	time.Sleep(180 * time.Millisecond)
	if current(a)["version"] == stale["version"] {
		t.Fatal("fixture did not run the realtime ticker")
	}
	if _, err = s.db.Exec("CREATE TRIGGER fail_save_control BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'save pause rollback'); END"); err != nil {
		t.Fatal(err)
	}
	a.command(stale, "save", nil, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_save_control"); err != nil {
		t.Fatal(err)
	}
	if current(a)["game"].(map[string]any)["paused"] == true {
		t.Fatal("failed consent retained pause")
	}
	a.command(current(a), "save", nil, 200)
	paused := current(a)
	if paused["status"] != "playing" || paused["game"].(map[string]any)["paused"] != true || len(paused["saveVotes"].([]any)) != 1 {
		t.Fatal("first consent did not freeze game", paused)
	}
	a.command(paused, "action", map[string]any{"increment": 1}, 400)
	a.post("/api/rooms/"+id, map[string]any{"type": "autoplay", "enabled": true, "version": paused["version"], "nonce": "save-consent-autoplay"}, 400)
	before := paused["game"].(map[string]any)["data"].(map[string]any)["count"]
	time.Sleep(180 * time.Millisecond)
	if current(a)["game"].(map[string]any)["data"].(map[string]any)["count"] != before {
		t.Fatal("frame advanced while waiting for consent")
	}
	ts.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(cfg, fstest.MapFS{"index.html": {Data: []byte("save control")}})
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	a.base, b.base = ts.URL, ts.URL
	if current(a)["game"].(map[string]any)["paused"] != true || len(current(a)["saveVotes"].([]any)) != 1 {
		t.Fatal("restart lost the paused save consent")
	}
	// A teammate who did not approve may decline and keep the journey playing.
	b.command(current(b), "save_cancel", nil, 200)
	if current(a)["game"].(map[string]any)["paused"] == true || len(current(a)["saveVotes"].([]any)) != 0 {
		t.Fatal("decline failed to release save-only pause")
	}
	a.command(current(a), "save", nil, 200)
	if _, err = s.db.Exec("CREATE TRIGGER fail_cancel_control BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'save cancel rollback'); END"); err != nil {
		t.Fatal(err)
	}
	b.command(current(b), "save_cancel", nil, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_cancel_control"); err != nil {
		t.Fatal(err)
	}
	if current(a)["game"].(map[string]any)["paused"] != true || len(current(a)["saveVotes"].([]any)) != 1 {
		t.Fatal("failed cancellation discarded the pause or consent")
	}
	b.command(current(b), "save", nil, 200)
	if a.state()["room"] != nil || b.state()["room"] != nil {
		t.Fatal("consented save did not release seats")
	}
	a.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	b.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	if current(a)["game"].(map[string]any)["paused"] == true {
		t.Fatal("resume retained save-only pause")
	}
	a.command(paused, "save", nil, 409)
	out := filepath.Join("..", "..", ".local", "saved-games")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "scope": "real HTTP, realtime ticker and SQLite rollback; cooperative save consent"}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "realtime-consent.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
