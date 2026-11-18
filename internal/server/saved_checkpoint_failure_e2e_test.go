package server

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func failedCheckpointFixture(t *testing.T) gameapi.Module {
	t.Helper()
	m := checkpointFixture(t)
	advance := m.Apply
	m.Apply = func(c gameapi.Context, g gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		next, err := advance(c, g, raw)
		if err != nil {
			return next, err
		}
		var d checkpointFixtureState
		if err = json.Unmarshal(next.Data, &d); err != nil {
			return next, err
		}
		if d.GameOver {
			if err = json.Unmarshal([]byte(`{"preserveSavedCheckpoint":true,"restartable":true}`), &next); err != nil {
				return next, err
			}
			next.Actors, next.TimeoutActors = []int{}, []int{}
			next.Deadline, next.SystemDeadline = 0, 0
		}
		return next, nil
	}
	return m
}

func TestSavedCheckpointFailureExitKeepsManualBaselineHTTP(t *testing.T) {
	if os.Getenv("PAL_CHECKPOINT_FAILURE_E2E") != "1" {
		t.Skip("explicit failure save-exit and retained manual baseline")
	}
	m := failedCheckpointFixture(t)
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("failure checkpoint")}})
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(s)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	defer s.Close()
	owner := newClient(t, ts.URL)
	owner.register("失败状态保存本人")
	r := owner.post("/api/rooms", map[string]any{"kind": m.ID, "name": "保留健康手动基准", "capacity": 1}, 201)
	id := r["id"].(string)
	owner.command(current(owner), "ready", nil, 200)
	owner.command(current(owner), "start", nil, 200)
	owner.command(current(owner), "action", map[string]any{}, 200)
	owner.command(current(owner), "save", nil, 200)
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	owner.command(current(owner), "action", map[string]any{"fail": true}, 200)
	owner.command(current(owner), "save", nil, 200)
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	d := current(owner)["game"].(map[string]any)["data"].(map[string]any)
	if d["gameOver"] != true || d["count"] != float64(2) {
		t.Fatal("failure save-exit did not retain the actual failed current state", d)
	}
	owner.command(current(owner), "load_checkpoint", nil, 200)
	d = current(owner)["game"].(map[string]any)["data"].(map[string]any)
	if d["gameOver"] != false || d["count"] != float64(1) || d["cash"] != float64(1224) {
		t.Fatal("failure save-exit replaced the recoverable manual baseline", d)
	}
	out := filepath.Join("..", "..", ".local/saved-games")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "real authenticated HTTP and SQLite, save failed current state without replacing prior healthy manual baseline; generic contract, not PAL defeat", "loaded": d}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "checkpoint-failure-exit.json"), append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSavedCheckpointFailedJourneyRestartHTTP(t *testing.T) {
	if os.Getenv("PAL_CHECKPOINT_FAILURE_E2E") != "1" {
		t.Skip("explicit failed journey restart identity and transaction")
	}
	m := failedCheckpointFixture(t)
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("restart checkpoint")}})
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(s)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	defer s.Close()
	owner, stranger := newClient(t, ts.URL), newClient(t, ts.URL)
	owner.register("失败重新开始本人")
	stranger.register("失败重新开始陌生人")
	r := owner.post("/api/rooms", map[string]any{"kind": m.ID, "name": "新旅程边界", "capacity": 1}, 201)
	id := r["id"].(string)
	owner.command(current(owner), "ready", nil, 200)
	owner.command(current(owner), "start", nil, 200)
	owner.command(current(owner), "restart", nil, 400)
	owner.command(current(owner), "action", map[string]any{}, 200)
	owner.command(current(owner), "save", nil, 200)
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	owner.command(current(owner), "action", map[string]any{"fail": true}, 200)
	before := current(owner)
	stranger.post("/api/rooms/"+id, map[string]any{"type": "restart", "version": before["version"], "nonce": "foreign-restart"}, 400)
	s.mu.Lock()
	broken := s.modules[m.ID]
	start := broken.Start
	broken.Start = func(gameapi.Context, json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{}, errors.New("original new-game initialization unavailable")
	}
	s.modules[m.ID] = broken
	s.mu.Unlock()
	owner.command(current(owner), "restart", nil, 400)
	if current(owner)["matchId"] != before["matchId"] || current(owner)["checkpointSavedAt"] != before["checkpointSavedAt"] {
		t.Fatal("failed initialization discarded the original journey or manual baseline")
	}
	s.mu.Lock()
	broken.Start = start
	s.modules[m.ID] = broken
	s.mu.Unlock()
	if _, err = s.db.Exec("CREATE TRIGGER fail_journey_restart BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'restart rollback'); END"); err != nil {
		t.Fatal(err)
	}
	owner.command(before, "restart", nil, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_journey_restart"); err != nil {
		t.Fatal(err)
	}
	if current(owner)["matchId"] != before["matchId"] {
		t.Fatal("failed restart discarded the current journey")
	}
	request := map[string]any{"type": "restart", "version": current(owner)["version"], "nonce": "restart-new-journey-once"}
	owner.post("/api/rooms/"+id, request, 200)
	after := current(owner)
	d := after["game"].(map[string]any)["data"].(map[string]any)
	if after["matchId"] == before["matchId"] || d["count"] != float64(0) || d["cash"] != float64(1234) || d["gameOver"] != false || after["checkpointSavedAt"] != nil {
		t.Fatal("restart did not create a fresh journey without the old manual baseline", after)
	}
	owner.command(before, "action", map[string]any{}, 409)
	owner.command(after, "action", map[string]any{}, 200)
	owner.post("/api/rooms/"+id, request, 200)
	if d = current(owner)["game"].(map[string]any)["data"].(map[string]any); d["count"] != float64(1) {
		t.Fatal("retry restarted a second time", d)
	}
	owner.command(current(owner), "load_checkpoint", nil, 400)
	out := filepath.Join("..", "..", ".local/saved-games")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "real authenticated HTTP/SQLite; only failed single-player noncompetitive journey restarts, commit rollback, new match identity, stale action rejection and nonce retry preserving new progress; no native PAL or browser acceptance", "newMatchID": after["matchId"] != before["matchId"]}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "checkpoint-failure-restart.json"), append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
