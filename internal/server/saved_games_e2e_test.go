package server

// Written before implementation. Failure cases: unsupported/anonymous/spectator
// save; unilateral approval; stale approvals after gameplay; private snapshot
// disclosure; stranger resume; occupied original player; duplicate save/resume;
// incomplete roster ticking; save/restore transaction rollback; clock reset;
// crash/restart; saved-room idle expiration; duplicate scoring or abandoned loss.
// All lifecycle changes use authenticated HTTP and the real SQLite transaction.

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func savedGameFixture() gameapi.Module {
	m := acceptanceModule()
	m.ID, m.Name, m.MinPlayers, m.MaxPlayers = "saved-fixture", "存档契约验收", 1, 3
	m.SupportsSave = true
	m.Configure = func(n int, _ json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Schema: 1, Unrated: true, Data: json.RawMessage(`{"count":0,"secret":"private-test-card"}`), Actors: []int{0}, TimeoutActors: []int{0}, Deadline: c.Now.Add(90 * time.Second).UnixMilli()}, nil
	}
	return m
}

func TestSavedGamePermissionsRollbackHTTP(t *testing.T) {
	m := savedGameFixture()
	plain := acceptanceModule()
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m, plain}}
	files := fstest.MapFS{"index.html": {Data: []byte("save permissions")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(s)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	a, b, stranger := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("原队友甲")
	b.register("原队友乙")
	stranger.register("陌生玩家")
	r := a.post("/api/rooms", map[string]any{"kind": m.ID, "name": "权限和回滚", "capacity": 3}, 201)
	id := r["id"].(string)
	b.command(current(a), "join", nil, 200)
	a.command(current(a), "add_bot", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	stranger.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	stranger.command(current(stranger), "save", nil, 400)
	a.command(current(a), "save", nil, 200)
	a.command(current(a), "save_cancel", nil, 200)
	if len(current(a)["saveVotes"].([]any)) != 0 {
		t.Fatal("cannot revoke approval")
	}
	a.command(current(a), "save", nil, 200)
	before := current(a)
	if _, err = s.db.Exec("CREATE TRIGGER fail_final_save BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'final save rollback'); END"); err != nil {
		t.Fatal(err)
	}
	b.command(current(b), "save", nil, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_final_save"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, current(a)) {
		t.Fatal("failed final approval changed snapshot or consent")
	}
	b.command(current(b), "save", nil, 200)
	if stranger.state()["room"] != nil {
		t.Fatal("saved room retained spectator")
	}
	a.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	if b.state()["room"] != nil {
		t.Fatal("resume occupied absent teammate")
	}
	a.command(current(a), "action", map[string]any{"increment": 1}, 400)
	a.command(current(a), "leave", nil, 200)
	if a.state()["room"] != nil {
		t.Fatal("pending resume cannot be cancelled")
	}
	a.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	before = current(a)
	if _, err = s.db.Exec("CREATE TRIGGER fail_resume BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'resume rollback'); END"); err != nil {
		t.Fatal(err)
	}
	b.post("/api/saves/"+id+"/resume", map[string]any{}, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_resume"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, current(a)) || b.state()["room"] != nil {
		t.Fatal("failed resume changed roster or progress")
	}
	// A pending confirmation survives a restart without occupying absent players.
	ts.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(s)
	ts = httptest.NewServer(s.Handler())
	a.base, b.base, stranger.base = ts.URL, ts.URL, ts.URL
	if current(a)["status"] != "restoring" || b.state()["room"] != nil {
		t.Fatal("pending resume lost on restart")
	}
	b.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	stranger.post("/api/rooms/"+id+"/watch", map[string]any{}, 403)
	for _, value := range stranger.state()["rooms"].([]any) {
		if value.(map[string]any)["id"] == id {
			t.Fatal("resumed private journey leaked into public directory")
		}
	}
	// No opt-in means no independent saves, even if a client forges the button.
	other := stranger.post("/api/rooms", map[string]any{"kind": plain.ID, "name": "不支持存档", "capacity": 2}, 201)
	stranger.command(other, "add_bot", nil, 200)
	stranger.command(current(stranger), "ready", nil, 200)
	stranger.command(current(stranger), "start", nil, 200)
	stranger.command(current(stranger), "save", nil, 400)
	out := filepath.Join("..", "..", ".local", "saved-games")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "real HTTP and SQLite, private original-roster resume and rollback", "restored": current(a)}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "permissions-rollback.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSavedGameLifecycleHTTP(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		t.Run(string(rune('0'+n))+"players", func(t *testing.T) {
			m := savedGameFixture()
			plain := acceptanceModule()
			cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m, plain}}
			files := fstest.MapFS{"index.html": {Data: []byte("saved game HTTP fixture")}}
			s, err := New(cfg, files)
			if err != nil {
				t.Fatal(err)
			}
			stopBotTicker(s)
			ts := httptest.NewServer(s.Handler())
			t.Cleanup(func() { ts.Close(); s.Close() })
			publishModuleForTest(t, ts.URL, m.ID)
			players := make([]*testClient, n)
			for i := range players {
				players[i] = newClient(t, ts.URL)
				players[i].register("存档玩家" + string(rune('甲'+i)))
			}
			owner := players[0]
			outsider := newClient(t, ts.URL)
			outsider.register("无权读档")
			anonymous := newClient(t, ts.URL)
			room := owner.post("/api/rooms", map[string]any{"kind": m.ID, "name": "多人旅程", "capacity": n}, 201)
			id := room["id"].(string)
			for _, p := range players[1:] {
				p.command(current(owner), "join", nil, 200)
			}
			for _, p := range players {
				p.command(current(p), "ready", nil, 200)
			}
			owner.command(current(owner), "start", nil, 200)
			if current(owner)["supportsSave"] != true {
				t.Fatal("save capability missing")
			}
			outsider.command(current(owner), "save", nil, 400)
			anonymous.command(current(owner), "save", nil, 401)
			trace := []any{}
			t.Cleanup(func() {
				out := filepath.Join("..", "..", ".local", "saved-games")
				if err := os.MkdirAll(out, 0755); err != nil {
					t.Error(err)
					return
				}
				raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "players": n, "scope": "authenticated HTTP/SQLite fixture, not PAL gameplay", "trace": trace}, "", "  ")
				if err := os.WriteFile(filepath.Join(out, "lifecycle-"+string(rune('0'+n))+".json"), raw, 0600); err != nil {
					t.Error(err)
				}
			})
			if n > 1 {
				owner.command(current(owner), "save", nil, 200)
				if current(owner)["status"] != "playing" {
					t.Fatal("unilateral save")
				}
				owner.command(current(owner), "action", map[string]any{"increment": 1}, 200)
				if len(current(owner)["saveVotes"].([]any)) != 0 {
					t.Fatal("gameplay retained outdated save approval")
				}
			}
			before := current(owner)
			s.mu.Lock()
			clockBefore := s.rooms[id].gameNow(time.Now()).UnixMilli()
			deadline := s.rooms[id].Module.Deadline
			matchID := s.rooms[id].MatchID
			s.mu.Unlock()
			// A rejected transaction must preserve both gameplay and approvals.
			if _, err := s.db.Exec("CREATE TRIGGER fail_save BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'saved-game rollback'); END"); err != nil {
				t.Fatal(err)
			}
			owner.command(before, "save", nil, 500)
			if _, err := s.db.Exec("DROP TRIGGER fail_save"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, current(owner)) {
				t.Fatal("failed save mutated room")
			}
			for _, p := range players[:n-1] {
				p.command(current(p), "save", nil, 200)
			}
			last := players[n-1]
			request := map[string]any{"type": "save", "version": current(last)["version"], "nonce": randomID(12)}
			last.post("/api/rooms/"+id, request, 200)
			last.post("/api/rooms/"+id, request, 200)
			for _, p := range players {
				if p.state()["room"] != nil {
					t.Fatal("save did not release seat")
				}
			}
			code, saves := owner.request("GET", "/api/saves", nil)
			if code != 200 || len(saves["saves"].([]any)) != 1 {
				t.Fatal("missing saved game", code, saves)
			}
			summary := saves["saves"].([]any)[0].(map[string]any)
			if summary["id"] != id || summary["module"] != nil || summary["game"] != nil {
				t.Fatal("invalid or private save summary", summary)
			}
			_, denied := outsider.request("GET", "/api/saves", nil)
			if len(denied["saves"].([]any)) != 0 {
				t.Fatal("stranger discovered private save")
			}
			outsider.post("/api/saves/"+id+"/resume", map[string]any{}, 404)
			owner.post("/api/rooms/"+id, map[string]any{"type": "action", "action": map[string]any{"increment": 1}, "version": summary["version"], "nonce": randomID(12)}, 400)
			// Saving frees the account for another actual game room.
			other := owner.post("/api/rooms", map[string]any{"kind": plain.ID, "name": "切换游戏", "capacity": 2}, 201)
			owner.post("/api/saves/"+id+"/resume", map[string]any{}, 409)
			owner.command(other, "leave", nil, 200)
			// Retention applies to rooms, never to explicitly saved journeys.
			s.mu.Lock()
			s.rooms[id].LastActive = time.Now().Add(-60 * 24 * time.Hour).Unix()
			err = s.save(s.rooms[id])
			s.expireIdleRooms(time.Now())
			s.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			ts.Close()
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = New(cfg, files)
			if err != nil {
				t.Fatal(err)
			}
			stopBotTicker(s)
			ts = httptest.NewServer(s.Handler())
			for _, p := range append(players, outsider) {
				p.base = ts.URL
			}
			for i, p := range players {
				p.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
				if i < n-1 && current(p)["status"] != "restoring" {
					t.Fatal("resumed before roster confirmation")
				}
				// A retry is safe and never creates a second room.
				p.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
			}
			after := current(owner)
			if after["id"] != id || after["status"] != "playing" {
				t.Fatal("did not restore same journey", after)
			}
			if !reflect.DeepEqual(before["game"].(map[string]any)["data"], after["game"].(map[string]any)["data"]) {
				t.Fatal("saved progress changed")
			}
			s.mu.Lock()
			restored := s.rooms[id]
			remaining := restored.Module.Deadline - restored.gameNow(time.Now()).UnixMilli()
			sameID := restored.MatchID == matchID
			s.mu.Unlock()
			if !sameID || remaining < deadline-clockBefore-2000 || remaining > deadline-clockBefore+2000 {
				t.Fatal("restored identity or clock budget changed", remaining, deadline-clockBefore)
			}
			var history, rating int
			if err := s.db.QueryRow("SELECT COUNT(*) FROM match_history").Scan(&history); err != nil {
				t.Fatal(err)
			}
			if err := s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger").Scan(&rating); err != nil {
				t.Fatal(err)
			}
			if history != 0 || rating != 0 {
				t.Fatal("saving treated as match outcome", history, rating)
			}
			trace = append(trace, map[string]any{"saved": summary, "restored": after, "remainingMS": remaining, "matchIDPreserved": sameID})
		})
	}
}
