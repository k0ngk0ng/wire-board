package server

// Failure inventory written before implementation:
// - Explicitly paused matches expire at the ordinary 24-hour limit or restart
//   resets the 30-day retention deadline; ordinary rooms accidentally inherit it.
// - Paused matches retain player/system deadlines or a final result; they keep
//   executing bot decisions (including asynchronous work) or change the journal.
// - A spectator/forged flag can pause/resume; a rejected/failed transaction or a
//   repeated nonce changes pause state, budget, version, history or retention.
// - Canceling autoplay resumes play or replenishes the remaining action budget.
// - Resume loses the saved budget, changes the board, grants resources, or does
//   not restore ordinary idle expiration and normal computer decisions.
// Real authenticated HTTP commands drive the fixture. The host clock and the
// stored activity timestamp are controlled only to exercise day/month boundaries.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type pauseFixtureState struct {
	Count     int   `json:"count"`
	Remaining int64 `json:"remaining"`
}

func pauseFixtureModule(calls *atomic.Int64) gameapi.Module {
	m := acceptanceModule()
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Data: json.RawMessage(`{"count":0,"remaining":0}`), Actors: []int{0, 1}, Attention: []int{0}, TimeoutActors: []int{0}, Deadline: c.Now.Add(90 * time.Second).UnixMilli()}, nil
	}
	m.View = func(v gameapi.Match, _ int) (any, error) {
		var data pauseFixtureState
		err := json.Unmarshal(v.Data, &data)
		return data, err
	}
	m.Apply = func(c gameapi.Context, v gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var a struct {
			Type string `json:"type"`
		}
		var data pauseFixtureState
		if json.Unmarshal(raw, &a) != nil || json.Unmarshal(v.Data, &data) != nil {
			return v, errors.New("invalid fixture action")
		}
		switch a.Type {
		case "pause", "invalid_pause":
			if c.Actor != 0 || v.Paused {
				return v, errors.New("cannot pause")
			}
			data.Remaining = max(0, v.Deadline-c.Now.UnixMilli())
			v.Paused = true
			if a.Type == "pause" {
				v.Deadline, v.SystemDeadline, v.TimeoutActors, v.Attention = 0, 0, nil, []int{}
			}
		case "resume":
			if c.Actor != 0 || !v.Paused {
				return v, errors.New("cannot resume")
			}
			v.Paused = false
			v.Deadline = c.Now.UnixMilli() + data.Remaining
			v.TimeoutActors, v.Attention = []int{0}, []int{0}
		case "advance":
			// Deliberately accepts automatic moves even while paused: the generic
			// host, not the fixture's rule guard, must stop computer scheduling.
			data.Count++
		default:
			return v, errors.New("unknown fixture action")
		}
		v.Data, _ = json.Marshal(data)
		return v, nil
	}
	m.BotAction = func(_ gameapi.Context, _ gameapi.Match) (json.RawMessage, error) {
		calls.Add(1)
		return json.RawMessage(`{"type":"advance"}`), nil
	}
	m.Reclaim = func(_ gameapi.Context, v gameapi.Match) (gameapi.Match, error) { return v, nil }
	return m
}

func TestModulePausedSaveHTTP(t *testing.T) {
	for _, asynchronous := range []bool{false, true} {
		name := "ordinary"
		if asynchronous {
			name = "background"
		}
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int64
			m := pauseFixtureModule(&calls)
			if asynchronous {
				m.BotSearch = &gameapi.BotSearch{
					Input: func(_ gameapi.Context, v gameapi.Match) (json.RawMessage, error) { calls.Add(1); return v.Data, nil },
					Think: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
						return json.RawMessage(`{"type":"advance"}`), nil
					},
				}
			}
			cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}
			files := fstest.MapFS{"index.html": {Data: []byte("pause HTTP")}}
			s, err := New(cfg, files)
			if err != nil {
				t.Fatal(err)
			}
			stopBotTicker(s)
			ts := httptest.NewServer(s.Handler())
			t.Cleanup(func() { ts.Close(); s.Close() })
			publishModuleForTest(t, ts.URL, "acceptance")
			a, b, spectator := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
			a.register("暂停房主")
			b.register("暂停队友")
			spectator.register("暂停观众")
			r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "跨天保存", "capacity": 2}, 201)
			id := r["id"].(string)
			b.command(r, "join", nil, 200)
			a.command(current(a), "ready", nil, 200)
			b.command(current(b), "ready", nil, 200)
			a.command(current(a), "start", nil, 200)
			spectator.post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			trace := []any{current(a)}
			t.Cleanup(func() {
				out := filepath.Join("..", "..", ".local", "module-pause")
				if err := os.MkdirAll(out, 0755); err != nil {
					t.Error(err)
					return
				}
				raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "trace": trace, "botCallbacks": calls.Load()}, "", "  ")
				if err := os.WriteFile(filepath.Join(out, name+".json"), raw, 0600); err != nil {
					t.Error(err)
				}
			})
			// Invalid metadata, spectators, and a failed commit cannot pause.
			before := current(a)
			a.command(before, "action", map[string]any{"type": "invalid_pause"}, 400)
			spectator.command(current(spectator), "action", map[string]any{"type": "pause"}, 400)
			b.command(current(b), "action", map[string]any{"type": "pause"}, 400)
			if !reflect.DeepEqual(before, current(a)) {
				t.Fatal("rejected pause mutated room")
			}
			if _, err = s.db.Exec("CREATE TRIGGER fail_pause BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'pause rollback'); END"); err != nil {
				t.Fatal(err)
			}
			a.command(current(a), "action", map[string]any{"type": "pause"}, 500)
			if _, err = s.db.Exec("DROP TRIGGER fail_pause"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, current(a)) {
				t.Fatal("failed pause transaction mutated room")
			}
			setAutoPlay(b, current(b), true, 200)
			request := map[string]any{"type": "action", "action": map[string]any{"type": "pause"}, "version": current(a)["version"], "nonce": randomID(12)}
			a.post("/api/rooms/"+id, request, 200)
			paused := current(a)
			a.post("/api/rooms/"+id, request, 200)
			if !reflect.DeepEqual(paused, current(a)) {
				t.Fatal("pause nonce replay changed budget or retention")
			}
			game := paused["game"].(map[string]any)
			if game["paused"] != true || paused["turnDeadline"] != float64(0) || len(game["attention"].([]any)) != 0 {
				t.Fatal("paused projection retains countdown or attention", paused)
			}
			budget := game["data"].(map[string]any)["remaining"].(float64)
			if budget < 80000 || budget > 90000 {
				t.Fatal("pause lost original action budget", budget)
			}
			// The host scheduler must remain silent even though the module's bot
			// always proposes a legal move and its Actors allow resume voting.
			var journalBefore, journalAfter int
			if err = s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=?", id).Scan(&journalBefore); err != nil {
				t.Fatal(err)
			}
			s.mu.Lock()
			s.runBots(time.Now().Add(48 * time.Hour))
			s.expireToAutoplay(s.rooms[id], time.Now().Add(48*time.Hour))
			s.expireModuleTimers(time.Now().Add(48 * time.Hour))
			s.collectModuleBotSearches(time.Now().Add(48 * time.Hour))
			s.mu.Unlock()
			if calls.Load() != 0 {
				t.Fatal("paused room invoked computer strategy", calls.Load())
			}
			if err = s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=?", id).Scan(&journalAfter); err != nil {
				t.Fatal(err)
			}
			if journalBefore != journalAfter || !reflect.DeepEqual(paused, current(a)) {
				t.Fatal("paused scheduler changed state or journal")
			}
			setAutoPlay(b, current(b), false, 200)
			if current(a)["game"].(map[string]any)["data"].(map[string]any)["remaining"] != budget || !s.rooms[id].Module.Paused {
				t.Fatal("reclaim changed paused budget")
			}
			// Persist an inactive paused table and restart the actual server.
			old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
			s.rooms[id].LastActive = old.Unix()
			if err = s.save(s.rooms[id]); err != nil {
				t.Fatal(err)
			}
			expires := old.Add(30 * 24 * time.Hour).UnixMilli()
			ts.Close()
			s.Close()
			s, err = New(cfg, files)
			if err != nil {
				t.Fatal(err)
			}
			stopBotTicker(s)
			ts = httptest.NewServer(s.Handler())
			for _, c := range []*testClient{a, b, spectator} {
				c.base = ts.URL
			}
			restored := current(a)
			trace = append(trace, paused, restored)
			if restored["status"] != "playing" || !s.rooms[id].Module.Paused || s.rooms[id].LastActive != old.Unix() {
				t.Fatal("restart lost paused retention")
			}
			if restored["game"].(map[string]any)["pauseExpiresAt"] != float64(expires) {
				t.Fatal("pause retention expiry missing or reset", restored["game"])
			}
			idleTick(s, time.UnixMilli(expires-1))
			if s.rooms[id].Status != "playing" {
				t.Fatal("paused room closed early")
			}
			a.command(current(a), "action", map[string]any{"type": "resume"}, 200)
			resumed := current(a)
			remaining := time.Until(time.UnixMilli(s.rooms[id].TurnDeadline)).Milliseconds()
			if s.rooms[id].Module.Paused || remaining > int64(budget) || remaining < int64(budget)-1000 {
				t.Fatal("resume changed remaining budget", remaining, budget)
			}
			if resumed["game"].(map[string]any)["data"].(map[string]any)["count"] != float64(0) {
				t.Fatal("resume changed game position")
			}
			trace = append(trace, resumed)
			setAutoPlay(b, current(b), true, 200)
			s.mu.Lock()
			s.runBots(time.Now().Add(time.Second))
			s.mu.Unlock()
			if !asynchronous {
				if calls.Load() != 1 || current(a)["game"].(map[string]any)["data"].(map[string]any)["count"] != float64(1) {
					t.Fatal("resume did not restore bot decisions")
				}
			}
			// A resumed room immediately returns to the normal 24-hour policy.
			last := s.rooms[id].LastActive
			idleTick(s, time.Unix(last, 0).Add(24*time.Hour))
			if s.rooms[id].Status != "closed" || s.rooms[id].CloseReason != "inactive" || s.rooms[id].Rated && s.rooms[id].Game.Finished {
				t.Fatal("resumed room did not restore ordinary idle closure")
			}
			trace = append(trace, current(a))
		})
	}
}

func TestModulePausedRetentionBoundaryHTTP(t *testing.T) {
	var calls atomic.Int64
	m := pauseFixtureModule(&calls)
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("pause retention")}})
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(s)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	defer s.Close()
	publishModuleForTest(t, ts.URL, "acceptance")
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("保留期限房主")
	b.register("保留期限队友")
	r := a.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "暂停到期", "capacity": 2}, 201)
	id := r["id"].(string)
	b.command(r, "join", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	a.command(current(a), "action", map[string]any{"type": "pause"}, 200)
	last := s.rooms[id].LastActive
	idleTick(s, time.Unix(last, 0).Add(30*24*time.Hour-time.Second))
	if s.rooms[id].Status != "playing" {
		t.Fatal("paused room closed before 30 days")
	}
	idleTick(s, time.Unix(last, 0).Add(30*24*time.Hour))
	if s.rooms[id].Status != "closed" || s.rooms[id].CloseReason != "inactive" || s.rooms[id].Game.Finished {
		t.Fatal("paused room did not close without result at 30 days")
	}
	var raw []byte
	if err = s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", s.rooms[id].MatchID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var record MatchRecord
	if err = json.Unmarshal(raw, &record); err != nil || record.Status != "closed" || record.Rated {
		t.Fatal("expired paused match incorrectly scored", err, record)
	}
	for _, p := range record.Players {
		if p.Won {
			t.Fatal("expired paused match has winner")
		}
	}
	out := filepath.Join("..", "..", ".local", "module-pause")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	proof, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "room": current(a), "history": record}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "retention-boundary.json"), proof, 0600); err != nil {
		t.Fatal(err)
	}
}

// This uses the live scheduler, unlike the deterministic retention-clock cases:
// an already running search must be cancelled when pause commits, and even an
// engine returning a late move must not modify the paused match or its journal.
func TestModulePausedPendingSearchHTTP(t *testing.T) {
	var calls atomic.Int64
	started, cancelled, release, returned := make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{}), make(chan struct{}, 1)
	m := pauseFixtureModule(&calls)
	m.BotSearch = &gameapi.BotSearch{
		Budget: 5 * time.Second,
		Input:  func(_ gameapi.Context, v gameapi.Match) (json.RawMessage, error) { return v.Data, nil },
		Think: func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
			started <- struct{}{}
			<-ctx.Done()
			cancelled <- struct{}{}
			<-release
			returned <- struct{}{}
			return json.RawMessage(`{"type":"advance"}`), nil
		},
	}
	f := newAsyncTable(t, m)
	r := f.start(t)
	id := r["id"].(string)
	asyncAwait(t, started)
	f.a.command(current(f.a), "action", map[string]any{"type": "pause"}, 200)
	paused := current(f.a)
	asyncAwait(t, cancelled)
	close(release)
	asyncAwait(t, returned)
	until := time.Now().Add(time.Second)
	for len(f.s.botSlots) > 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	f.s.mu.Lock()
	f.s.collectModuleBotSearches(time.Now())
	f.s.mu.Unlock()
	if calls.Load() != 0 || !reflect.DeepEqual(paused, current(f.a)) {
		t.Fatal("late search changed paused match")
	}
	if len(f.s.botSlots) != 0 {
		t.Fatal("cancelled worker retained slot")
	}
	var advances int
	if err := f.s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=? AND json_extract(action,'$.action.type')='advance'", id).Scan(&advances); err != nil {
		t.Fatal(err)
	}
	if advances != 0 {
		t.Fatal("late search wrote a move")
	}
	out := filepath.Join("..", "..", ".local", "module-pause")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "room": paused, "lateActions": advances}, "", "  ")
	if err := os.WriteFile(filepath.Join(out, "pending-search.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
