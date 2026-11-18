package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type checkpointFixtureState struct {
	Count    int    `json:"count"`
	Cash     int    `json:"cash"`
	Seed     uint32 `json:"seed"`
	Epoch    int    `json:"epoch"`
	Held     bool   `json:"held"`
	GameOver bool   `json:"gameOver"`
	Secret   string `json:"secret"`
}

func checkpointFixture(t *testing.T) gameapi.Module {
	t.Helper()
	m := savedGameFixture()
	m.ID, m.Name, m.MinPlayers, m.MaxPlayers = "checkpoint-fixture", "手动读档契约验收", 1, 1
	if err := json.Unmarshal([]byte(`{"supportsCheckpoint":true}`), &m); err != nil {
		t.Fatal(err)
	}
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		d := checkpointFixtureState{Cash: 1234, Seed: 17, Epoch: 1, Secret: "checkpoint-private-random"}
		data, _ := json.Marshal(d)
		return gameapi.Match{Schema: 1, Unrated: true, Data: data, Actors: []int{0}, TimeoutActors: []int{0}, Deadline: c.Now.Add(90 * time.Second).UnixMilli()}, nil
	}
	m.Apply = func(c gameapi.Context, g gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var action struct {
			Fail bool `json:"fail"`
		}
		var d checkpointFixtureState
		if json.Unmarshal(raw, &action) != nil || json.Unmarshal(g.Data, &d) != nil || d.GameOver || c.Actor != 0 {
			return g, errors.New("invalid checkpoint fixture action")
		}
		d.Count++
		d.Cash -= 10
		d.Seed = d.Seed*1664525 + 1013904223
		d.Held, d.GameOver = true, action.Fail
		g.Data, _ = json.Marshal(d)
		return g, nil
	}
	m.ResumeSaved = func(c gameapi.Context, g gameapi.Match) (gameapi.Match, error) {
		var d checkpointFixtureState
		if err := json.Unmarshal(g.Data, &d); err != nil {
			return g, err
		}
		d.Epoch, d.Held = max(d.Epoch+1, c.Version+1), false
		g.Data, _ = json.Marshal(d)
		return g, nil
	}
	m.View = func(g gameapi.Match, _ int) (any, error) {
		var d checkpointFixtureState
		if err := json.Unmarshal(g.Data, &d); err != nil {
			return nil, err
		}
		return map[string]any{"count": d.Count, "cash": d.Cash, "epoch": d.Epoch, "held": d.Held, "gameOver": d.GameOver}, nil
	}
	resume := m.ResumeSaved
	m.RestoreCheckpoint = func(c gameapi.Context, saved, active gameapi.Match) (gameapi.Match, error) {
		var current, next checkpointFixtureState
		if err := json.Unmarshal(active.Data, &current); err != nil {
			return saved, err
		}
		restored, err := resume(c, saved)
		if err != nil {
			return saved, err
		}
		if err = json.Unmarshal(restored.Data, &next); err != nil {
			return saved, err
		}
		next.Epoch = max(next.Epoch, current.Epoch+1)
		restored.Data, _ = json.Marshal(next)
		return restored, nil
	}
	return m
}

func TestSavedCheckpointManualBaselineAndReloadHTTP(t *testing.T) {
	if os.Getenv("PAL_CHECKPOINT_E2E") != "1" {
		t.Skip("explicit platform checkpoint-save implementation acceptance")
	}
	m := checkpointFixture(t)
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}
	files := fstest.MapFS{"index.html": {Data: []byte("checkpoint acceptance")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(s)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	owner, stranger := newClient(t, ts.URL), newClient(t, ts.URL)
	owner.register("手动读档本人")
	stranger.register("手动读档陌生人")
	r := owner.post("/api/rooms", map[string]any{"kind": m.ID, "name": "独立手动基准", "capacity": 1}, 201)
	id := r["id"].(string)
	owner.command(current(owner), "ready", nil, 200)
	owner.command(current(owner), "start", nil, 200)
	owner.command(current(owner), "load_checkpoint", nil, 400)
	owner.command(current(owner), "action", map[string]any{}, 200)
	matchID := current(owner)["matchId"]
	snapshot := func() []byte {
		s.mu.Lock()
		defer s.mu.Unlock()
		b, err := json.Marshal(s.rooms[id])
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	state := func() checkpointFixtureState {
		s.mu.Lock()
		defer s.mu.Unlock()
		var d checkpointFixtureState
		if err := json.Unmarshal(s.rooms[id].Module.Data, &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	saved := state()
	owner.command(current(owner), "save", nil, 200)
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	owner.command(current(owner), "action", map[string]any{}, 200)
	owner.command(current(owner), "action", map[string]any{"fail": true}, 200)
	failed := state()
	if !failed.GameOver || failed.Count != 3 || failed.Seed == saved.Seed {
		t.Fatal("fixture did not advance beyond the manual save")
	}
	before := current(owner)
	stranger.post("/api/rooms/"+id, map[string]any{"type": "load_checkpoint", "version": before["version"], "nonce": "stranger-reload"}, 400)
	request := map[string]any{"type": "load_checkpoint", "version": before["version"], "nonce": "manual-reload-once"}
	owner.post("/api/rooms/"+id, request, 200)
	loaded := state()
	if loaded.Count != saved.Count || loaded.Cash != saved.Cash || loaded.Seed != saved.Seed || loaded.GameOver || loaded.Held || loaded.Epoch <= failed.Epoch || current(owner)["matchId"] != matchID {
		t.Fatal("manual checkpoint did not restore its original independent rules and input boundary", saved, failed, loaded)
	}
	owner.command(before, "action", map[string]any{}, 409)
	owner.command(current(owner), "action", map[string]any{}, 200)
	afterAdvance := snapshot()
	owner.post("/api/rooms/"+id, request, 200)
	if !reflect.DeepEqual(afterAdvance, snapshot()) {
		t.Fatal("duplicate reload nonce rewound already resumed progress")
	}
	owner.post("/api/rooms/"+id, map[string]any{"type": "load_checkpoint", "version": before["version"], "nonce": "old-page-new-nonce"}, 409)
	beforeFailure := snapshot()
	if _, err = s.db.Exec("CREATE TRIGGER fail_checkpoint_reload BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'checkpoint rollback'); END"); err != nil {
		t.Fatal(err)
	}
	owner.command(current(owner), "load_checkpoint", nil, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_checkpoint_reload"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeFailure, snapshot()) {
		t.Fatal("failed checkpoint commit changed the active rules or manual baseline")
	}
	s.mu.Lock()
	broken := s.modules[m.ID]
	restore := broken.RestoreCheckpoint
	broken.RestoreCheckpoint = func(_ gameapi.Context, g, active gameapi.Match) (gameapi.Match, error) {
		g.Data[0] = '!'
		active.Data[0] = '!'
		return g, errors.New("restore callback failed after mutation")
	}
	s.modules[m.ID] = broken
	s.mu.Unlock()
	owner.command(current(owner), "load_checkpoint", nil, 400)
	if !reflect.DeepEqual(beforeFailure, snapshot()) {
		t.Fatal("failed callback mutated the persisted manual checkpoint")
	}
	s.mu.Lock()
	broken.RestoreCheckpoint = restore
	s.modules[m.ID] = broken
	s.mu.Unlock()
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
	owner.base, stranger.base = ts.URL, ts.URL
	owner.command(current(owner), "load_checkpoint", nil, 200)
	if d := state(); d.Count != saved.Count || d.Seed != saved.Seed || d.Cash != saved.Cash || d.Held || d.GameOver {
		t.Fatal("process restart lost the independent manual checkpoint", d)
	}
	owner.command(current(owner), "action", map[string]any{}, 200)
	if _, err = s.db.Exec("CREATE TRIGGER fail_checkpoint_save BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'manual save rollback'); END"); err != nil {
		t.Fatal(err)
	}
	beforeFailure = snapshot()
	owner.command(current(owner), "save", nil, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_checkpoint_save"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeFailure, snapshot()) {
		t.Fatal("failed second manual save replaced the first checkpoint")
	}
	owner.command(current(owner), "load_checkpoint", nil, 200)
	if d := state(); d.Count != saved.Count || d.Seed != saved.Seed {
		t.Fatal("failed replacement destroyed the first manual baseline")
	}
	owner.command(current(owner), "action", map[string]any{}, 200)
	second := state()
	owner.command(current(owner), "save", nil, 200)
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	owner.command(current(owner), "action", map[string]any{"fail": true}, 200)
	owner.command(current(owner), "load_checkpoint", nil, 200)
	if d := state(); d.Count != second.Count || d.Cash != second.Cash || d.Seed != second.Seed || d.GameOver {
		t.Fatal("successful second save did not replace the manual baseline", d, second)
	}
	for _, table := range []string{"match_history", "rating_ledger"} {
		var count int
		if err = s.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal("manual reload created settlement records", table, count, err)
		}
	}
	public, _ := json.Marshal(stranger.state())
	if strings.Contains(string(public), "checkpoint-private-random") || strings.Contains(string(public), "\"seed\"") {
		t.Fatal("public response exposed checkpoint internals")
	}
	out := filepath.Join("..", "..", ".local/saved-games")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "real authenticated HTTP and SQLite; distinct manual baseline after ordinary resume and failure, exact resources/random restore, monotonically newer input boundary, stale actions/reloads rejected, nonce retry after new action, callback/commit rollback and process restart; generic contract, not PAL native death or browser acceptance", "manual": saved, "failed": failed, "loaded": loaded, "matchIDRetained": true}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "checkpoint-lifecycle.json"), append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSavedCheckpointCapabilityGatesHTTP(t *testing.T) {
	if os.Getenv("PAL_CHECKPOINT_E2E") != "1" {
		t.Skip("explicit platform checkpoint capability gates")
	}
	for _, patch := range []string{`{"minPlayers":1,"maxPlayers":3}`, `{"supportsSave":false}`, `{"ratingPolicy":"coop_campaign_v1"}`} {
		m := checkpointFixture(t)
		if err := json.Unmarshal([]byte(patch), &m); err != nil {
			t.Fatal(err)
		}
		s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("invalid checkpoint capability")}})
		if err == nil {
			s.Close()
			t.Fatal("invalid checkpoint registration was accepted", patch)
		}
	}
	for _, opted := range []bool{false, true} {
		t.Run(fmt.Sprint(opted), func(t *testing.T) {
			m := checkpointFixture(t)
			if !opted {
				if err := json.Unmarshal([]byte(`{"supportsCheckpoint":false}`), &m); err != nil {
					t.Fatal(err)
				}
				m.RestoreCheckpoint = nil
			} else {
				start := m.Start
				m.Start = func(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
					g, err := start(c, options)
					g.Unrated = false
					return g, err
				}
			}
			s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("checkpoint opt in")}})
			if err != nil {
				t.Fatal(err)
			}
			stopBotTicker(s)
			ts := httptest.NewServer(s.Handler())
			defer ts.Close()
			defer s.Close()
			owner := newClient(t, ts.URL)
			owner.register("能力验收玩家")
			owner.post("/api/rooms", map[string]any{"kind": m.ID, "name": "显式能力", "capacity": 1}, 201)
			owner.command(current(owner), "ready", nil, 200)
			if opted {
				owner.command(current(owner), "start", nil, 400)
				if current(owner)["status"] != "waiting" {
					t.Fatal("invalid competitive start changed the waiting room")
				}
				return
			}
			owner.command(current(owner), "start", nil, 200)
			owner.command(current(owner), "load_checkpoint", nil, 400)
			if current(owner)["supportsCheckpoint"] == true {
				t.Fatal("save-only game advertised checkpoint reload")
			}
		})
	}
}

func TestSavedCheckpointConcurrentReloadHTTP(t *testing.T) {
	if os.Getenv("PAL_CHECKPOINT_E2E") != "1" {
		t.Skip("explicit concurrent checkpoint restore contract")
	}
	m := checkpointFixture(t)
	var calls atomic.Int32
	restore := m.RestoreCheckpoint
	m.RestoreCheckpoint = func(c gameapi.Context, g, active gameapi.Match) (gameapi.Match, error) {
		calls.Add(1)
		return restore(c, g, active)
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("concurrent checkpoint")}})
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(s)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	defer s.Close()
	owner := newClient(t, ts.URL)
	owner.register("并发读档本人")
	room := owner.post("/api/rooms", map[string]any{"kind": m.ID, "name": "并发读取", "capacity": 1}, 201)
	id := room["id"].(string)
	owner.command(current(owner), "ready", nil, 200)
	owner.command(current(owner), "start", nil, 200)
	owner.command(current(owner), "action", map[string]any{}, 200)
	owner.command(current(owner), "save", nil, 200)
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	owner.command(current(owner), "action", map[string]any{}, 200)
	version := current(owner)["version"]
	calls.Store(0)
	start := make(chan struct{})
	statuses := make(chan int, 24)
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			body, _ := json.Marshal(map[string]any{"type": "load_checkpoint", "version": version, "nonce": fmt.Sprint("parallel-checkpoint-", i)})
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/rooms/"+id, strings.NewReader(string(body)))
			if err != nil {
				statuses <- 0
				return
			}
			req.Header.Set("Content-Type", "application/json")
			res, err := owner.client.Do(req)
			if err != nil {
				statuses <- 0
				return
			}
			defer res.Body.Close()
			statuses <- res.StatusCode
		}(i)
	}
	close(start)
	wg.Wait()
	close(statuses)
	success := 0
	for status := range statuses {
		if status == 200 {
			success++
		} else if status != 409 {
			t.Fatal("unexpected concurrent reload response", status)
		}
	}
	if success != 1 || calls.Load() != 1 || current(owner)["game"].(map[string]any)["data"].(map[string]any)["count"] != float64(1) {
		t.Fatal("concurrent checkpoint restore ran more than once", success, calls.Load())
	}
	out := filepath.Join("..", "..", ".local/saved-games")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "single server instance, real authenticated HTTP and SQLite concurrent manual checkpoint loads; not multi-process deployment or game acceptance", "requests": 24, "committed": success, "resumeCallbacks": calls.Load()}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "checkpoint-concurrent.json"), append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
