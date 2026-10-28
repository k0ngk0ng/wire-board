package server

// Failure inventory before implementation:
// - A paused real-time game cannot advance presentation, or calls ordinary
//   OnTimer/BotAction/timeouts and accidentally moves the game or auto-resumes.
// - A paused callback changes actors, pause/result/rating/schema/attention or
//   other control metadata, including by mutating aliased input slices.
// - Loading, future/cleared deadlines or closed rooms still invoke callbacks;
//   automatic presentation refreshes LastActive and evades 30-day expiration.
// - Rejected/error/panicking callbacks or failed critical commits leak a half
//   frame, journal, checkpoint, notification or outcome.
// - Pause/resume retries duplicate transitions; persisted presentation/deadline
//   is lost on restart; journal cannot identify/replay the paused callback.
// Authenticated HTTP and WS use a tiny presentation fixture and real SQLite;
// only the production scheduler's time is advanced, never a stored outcome.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type pausedTimerCounts struct{ normal, paused, bots atomic.Int64 }
type pausedTimerData struct {
	Physics, Presentation int
	Cue                   bool
}

func presentationTimerModule(mode string, counts *pausedTimerCounts, startPaused bool) gameapi.Module {
	m := realtimeFixture(true, time.Hour, 0, "")
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Schema: 1, Data: json.RawMessage(`{"Physics":0,"Presentation":0,"Cue":true}`), Actors: []int{0, 1, 2, 3}, Attention: []int{}, Paused: startPaused, SystemDeadline: c.Now.Add(time.Hour).UnixMilli(), CheckpointKey: "initial"}, nil
	}
	m.View = func(v gameapi.Match, _ int) (any, error) {
		var d pausedTimerData
		err := json.Unmarshal(v.Data, &d)
		return d, err
	}
	m.Reclaim = func(_ gameapi.Context, v gameapi.Match) (gameapi.Match, error) { return v, nil }
	m.BotAction = func(gameapi.Context, gameapi.Match) (json.RawMessage, error) { counts.bots.Add(1); return nil, nil }
	m.Apply = func(c gameapi.Context, v gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var a struct{ Type string }
		if json.Unmarshal(raw, &a) != nil || c.Actor != 0 {
			return v, errors.New("unauthorized fixture action")
		}
		switch a.Type {
		case "pause":
			if v.Paused {
				return v, errors.New("already paused")
			}
			v.Paused = true
			v.SystemDeadline = c.Now.Add(time.Hour).UnixMilli()
			if mode == "disabled" {
				v.SystemDeadline = 0
			}
			v.CheckpointKey = "paused"
		case "resume":
			if !v.Paused {
				return v, errors.New("not paused")
			}
			v.Paused = false
			v.SystemDeadline = c.Now.Add(time.Hour).UnixMilli()
			v.CheckpointKey = "resumed"
		default:
			return v, errors.New("unknown fixture action")
		}
		return v, nil
	}
	m.OnTimer = func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) {
		counts.normal.Add(1)
		var d pausedTimerData
		_ = json.Unmarshal(v.Data, &d)
		d.Physics++
		v.Data, _ = json.Marshal(d)
		v.SystemDeadline = c.Now.Add(time.Hour).UnixMilli()
		return v, nil
	}
	callback := func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) {
		counts.paused.Add(1)
		if c.Actor != -1 || !v.Paused || c.Now.UnixMilli() < v.SystemDeadline {
			return v, errors.New("wrong presentation context")
		}
		var d pausedTimerData
		if err := json.Unmarshal(v.Data, &d); err != nil {
			return v, err
		}
		d.Presentation++
		if d.Presentation >= 3 {
			d.Cue = false
			v.CheckpointKey = "cue-ended"
		}
		v.Data, _ = json.Marshal(d)
		v.SystemDeadline = c.Now.Add(time.Hour).UnixMilli()
		switch mode {
		case "unpause":
			v.Paused = false
		case "actors":
			v.Actors[0], v.Actors[1] = v.Actors[1], v.Actors[0]
		case "deadline":
			v.Deadline = 10
			v.TimeoutActors = []int{0}
		case "expired-deadline":
			v.SystemDeadline = c.Now.UnixMilli()
		case "attention":
			v.Attention = []int{0}
		case "schema":
			v.Schema++
		case "computer":
			v.HasComputerPlayers = true
		case "practice":
			v.Unrated = true
		case "log":
			v.Log = append(v.Log, "unauthorized timer log")
		case "eliminated":
			v.Actors = v.Actors[1:]
			v.Eliminated = []int{0}
		case "result":
			v.Paused = false
			v.Actors = nil
			v.Attention = nil
			v.SystemDeadline = 0
			v.Result = &gameapi.Result{Cooperative: true, Ranks: []int{1, 1, 1, 1}, Winners: []int{0, 1, 2, 3}}
		case "error":
			v.Data[0] = '!'
			return v, errors.New("injected paused callback error")
		case "panic":
			v.Actors[0] = 3
			panic("injected paused callback panic")
		case "stop":
			v.SystemDeadline = 0
		case "critical":
			v.CheckpointKey = fmt.Sprintf("presentation-%d", d.Presentation)
		}
		return v, nil
	}
	if mode != "disabled" {
		field := reflect.ValueOf(&m).Elem().FieldByName("OnPausedTimer")
		if field.IsValid() {
			field.Set(reflect.ValueOf(callback))
		}
	}
	return m
}

type pausedTimerHTTP struct {
	*realtimeTable
	t      *testing.T
	trace  []any
	counts *pausedTimerCounts
}

func newPausedTimerHTTP(t *testing.T, mode string, loading, startPaused bool) *pausedTimerHTTP {
	t.Helper()
	counts := &pausedTimerCounts{}
	m := presentationTimerModule(mode, counts, startPaused)
	f := &realtimeTable{cfg: Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{m}, ClientLoading: loading}}
	h := &pausedTimerHTTP{realtimeTable: f, t: t, counts: counts}
	var err error
	f.s, err = New(f.cfg, fstest.MapFS{"index.html": {Data: []byte("paused presentation timer")}})
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(f.s)
	f.ts = httptest.NewServer(f.s.Handler())
	t.Cleanup(func() {
		f.ts.Close()
		f.s.Close()
		out := filepath.Join("..", "..", ".local", "paused-timer", "evidence")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "trace": h.trace, "callbacks": map[string]int64{"normal": counts.normal.Load(), "paused": counts.paused.Load(), "bots": counts.bots.Load()}}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	f.a = newClient(t, f.ts.URL)
	f.a.register("暂停呈现玩家")
	r := f.a.post("/api/rooms", map[string]any{"kind": m.ID, "name": "暂停呈现", "capacity": 4}, 201)
	f.id = r["id"].(string)
	for i := 0; i < 3; i++ {
		f.a.command(current(f.a), "add_bot", nil, 200)
	}
	f.a.command(current(f.a), "ready", nil, 200)
	f.a.command(current(f.a), "start", nil, 200)
	return h
}
func (h *pausedTimerHTTP) action(kind string) {
	h.t.Helper()
	h.a.command(current(h.a), "action", map[string]any{"type": kind}, 200)
}
func (h *pausedTimerHTTP) state() pausedTimerData {
	h.t.Helper()
	r := current(h.a)
	h.trace = append(h.trace, r)
	raw, _ := json.Marshal(r["game"].(map[string]any)["data"])
	var d pausedTimerData
	_ = json.Unmarshal(raw, &d)
	return d
}
func (h *pausedTimerHTTP) tick() error {
	h.s.mu.Lock()
	defer h.s.mu.Unlock()
	return h.s.runRealtimeModuleFrame(h.s.rooms[h.id], time.UnixMilli(h.s.rooms[h.id].Module.SystemDeadline))
}
func (h *pausedTimerHTTP) restart() {
	h.t.Helper()
	h.ts.Close()
	h.s.Close()
	var err error
	h.s, err = New(h.cfg, fstest.MapFS{"index.html": {Data: []byte("paused presentation timer")}})
	if err != nil {
		h.t.Fatal(err)
	}
	stopBotTicker(h.s)
	h.ts = httptest.NewServer(h.s.Handler())
	h.a.base = h.ts.URL
}

func TestModulePausedTimerPresentationHTTP(t *testing.T) {
	h := newPausedTimerHTTP(t, "normal", false, false)
	h.action("pause")
	last := h.s.rooms[h.id].LastActive
	for i := 1; i <= 3; i++ {
		if err := h.tick(); err != nil {
			t.Fatal(err)
		}
		d := h.state()
		if d.Physics != 0 || d.Presentation != i || d.Cue != (i < 3) {
			t.Fatal("wrong paused frame", d)
		}
	}
	if h.counts.normal.Load() != 0 || h.counts.bots.Load() != 0 || h.s.rooms[h.id].LastActive != last {
		t.Fatal("paused presentation ran gameplay/renewed retention")
	}
	if stored := pauseStoredRoom(t, h.realtimeTable); !stored.Module.Paused || stored.Module.CheckpointKey != "cue-ended" {
		t.Fatal("critical cue ending was not saved")
	}
	before := h.state()
	deadline := h.s.rooms[h.id].Module.SystemDeadline
	h.restart()
	if h.state() != before || !h.s.rooms[h.id].Module.Paused || h.s.rooms[h.id].Module.SystemDeadline != deadline {
		t.Fatal("restart lost paused presentation/deadline")
	}
	h.action("resume")
	if err := h.tick(); err != nil {
		t.Fatal(err)
	}
	d := h.state()
	if d.Physics != 1 || d.Presentation != 3 || d.Cue {
		t.Fatal("resume replayed old cue/phase", d)
	}
}

func TestModulePausedTimerControlIsolationHTTP(t *testing.T) {
	for _, mode := range []string{"unpause", "actors", "deadline", "expired-deadline", "attention", "schema", "computer", "practice", "log", "eliminated", "result", "error", "panic"} {
		t.Run(mode, func(t *testing.T) {
			h := newPausedTimerHTTP(t, mode, false, false)
			h.action("pause")
			before := current(h.a)
			stored := pauseStoredRoom(t, h.realtimeTable)
			if err := h.tick(); err == nil {
				t.Fatal("invalid paused callback accepted", mode)
			}
			if !reflect.DeepEqual(before, current(h.a)) || !reflect.DeepEqual(stored, pauseStoredRoom(t, h.realtimeTable)) {
				t.Fatal("rejected callback leaked candidate")
			}
			h.trace = append(h.trace, before, current(h.a))
		})
	}
}

func TestModulePausedTimerLoadingExpiryAndOptOutHTTP(t *testing.T) {
	t.Run("loading", func(t *testing.T) {
		h := newPausedTimerHTTP(t, "normal", true, true)
		before := current(h.a)
		h.frame(time.Now().Add(48 * time.Hour))
		if h.counts.paused.Load() != 0 || !reflect.DeepEqual(before, current(h.a)) {
			t.Fatal("loading advanced paused clock")
		}
		g := loadInfo(t, h.a)
		for _, kind := range []string{"load_begin", "load_ready"} {
			code, data := loadingCommand(h.a, current(h.a), kind, g["token"].(string), g["build"].(string), "paused-client")
			if code != 200 {
				t.Fatal(code, data)
			}
		}
		h.s.mu.Lock()
		due := h.s.rooms[h.id].Module.SystemDeadline
		offset := h.s.rooms[h.id].ClockOffsetMS
		h.s.mu.Unlock()
		h.frame(time.UnixMilli(due + offset))
		if h.state().Presentation != 1 {
			t.Fatal("loaded presentation did not advance")
		}
	})
	t.Run("opt-out", func(t *testing.T) {
		h := newPausedTimerHTTP(t, "disabled", false, false)
		h.action("pause")
		before := current(h.a)
		h.frame(time.Now().Add(48 * time.Hour))
		if !reflect.DeepEqual(before, current(h.a)) || h.counts.normal.Load() != 0 {
			t.Fatal("ordinary pause behavior changed")
		}
	})
	t.Run("cleared-deadline", func(t *testing.T) {
		h := newPausedTimerHTTP(t, "stop", false, false)
		h.action("pause")
		if err := h.tick(); err != nil {
			t.Fatal(err)
		}
		before := current(h.a)
		h.frame(time.Now().Add(48 * time.Hour))
		if !reflect.DeepEqual(before, current(h.a)) {
			t.Fatal("stopped clock advanced")
		}
	})
	t.Run("expiry", func(t *testing.T) {
		h := newPausedTimerHTTP(t, "normal", false, false)
		h.action("pause")
		old := time.Now().Add(-29 * 24 * time.Hour).Truncate(time.Second)
		h.s.mu.Lock()
		h.s.rooms[h.id].LastActive = old.Unix()
		h.s.mu.Unlock()
		for i := 0; i < 3; i++ {
			if err := h.tick(); err != nil {
				t.Fatal(err)
			}
		}
		if h.s.rooms[h.id].LastActive != old.Unix() {
			t.Fatal("timer renewed idle retention")
		}
		idleTick(h.s, old.Add(30*24*time.Hour-time.Second))
		if current(h.a)["status"] != "playing" {
			t.Fatal("expired early")
		}
		idleTick(h.s, old.Add(30*24*time.Hour))
		if current(h.a)["status"] != "closed" {
			t.Fatal("paused presentation evaded expiry")
		}
		before := current(h.a)
		h.frame(time.Now().Add(60 * 24 * time.Hour))
		if !reflect.DeepEqual(before, current(h.a)) {
			t.Fatal("closed presentation advanced")
		}
	})
}

func TestModulePausedTimerDurableReplayHTTP(t *testing.T) {
	h := newPausedTimerHTTP(t, "critical", false, false)
	h.action("pause")
	before := pauseStoredRoom(t, h.realtimeTable)
	live := current(h.a)
	if _, err := h.s.db.Exec("CREATE TRIGGER fail_paused_clock BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'paused clock rollback'); END"); err != nil {
		t.Fatal(err)
	}
	if err := h.tick(); err == nil {
		t.Fatal("critical frame bypassed storage")
	}
	if !reflect.DeepEqual(live, current(h.a)) || !reflect.DeepEqual(before, pauseStoredRoom(t, h.realtimeTable)) {
		t.Fatal("failed critical frame published")
	}
	if _, err := h.s.db.Exec("DROP TRIGGER fail_paused_clock"); err != nil {
		t.Fatal(err)
	}
	if err := h.tick(); err != nil {
		t.Fatal(err)
	}
	stored := pauseStoredRoom(t, h.realtimeTable)
	var raw []byte
	if err := h.s.db.QueryRow("SELECT action FROM actions WHERE room_id=? AND nonce=?", h.id, fmt.Sprintf("frame-%d", stored.Version)).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type        string
		PausedTimer bool
		TimerAt     int64
		Inputs      []any
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "realtime_frame" || !event.PausedTimer || event.TimerAt != before.Module.SystemDeadline || len(event.Inputs) != 0 {
		t.Fatal("paused journal cannot identify replay", string(raw))
	}
	callback := reflect.ValueOf(h.cfg.Modules[0]).FieldByName("OnPausedTimer")
	if !callback.IsValid() || callback.IsNil() {
		t.Fatal("missing paused callback")
	}
	ctx := moduleContext(&before, -1, before.Version, time.UnixMilli(event.TimerAt+before.ClockOffsetMS))
	result := callback.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(*before.Module)})
	if !result[1].IsNil() || !reflect.DeepEqual(result[0].Interface(), *stored.Module) {
		t.Fatal("journal replay differs from committed paused frame")
	}
	h.restart()
	beforeJSON, _ := json.Marshal(stored.Module)
	afterJSON, _ := json.Marshal(h.s.rooms[h.id].Module)
	if string(beforeJSON) != string(afterJSON) {
		t.Fatal("restart changed checkpoint")
	}
	h.trace = append(h.trace, map[string]any{"journal": json.RawMessage(raw), "before": before, "after": stored})
}

func TestModulePausedTimerStreamAndStartupExpiryHTTP(t *testing.T) {
	t.Run("stream", func(t *testing.T) {
		h := newPausedTimerHTTP(t, "normal", false, false)
		c := streamDial(t, h.a, true)
		defer c.CloseNow()
		streamUntil(t, c, "state")
		request := map[string]any{"type": "input", "roomId": h.id, "version": current(h.a)["version"], "nonce": "paused-clock-stream", "action": map[string]any{"type": "pause"}}
		streamSend(t, c, request)
		ack := streamUntil(t, c, "ack")
		if ack["status"] != float64(200) || ack["durable"] != true {
			t.Fatal("pause was not durable", ack)
		}
		streamSend(t, c, request)
		repeated := streamUntil(t, c, "ack")
		if repeated["version"] != ack["version"] {
			t.Fatal("duplicate pause changed version", repeated)
		}
		due := h.s.rooms[h.id].Module.SystemDeadline
		h.frame(time.UnixMilli(due - 1))
		if h.counts.paused.Load() != 0 {
			t.Fatal("early callback")
		}
		h.frame(time.UnixMilli(due))
		var packet map[string]any
		for attempts := 0; attempts < 5; attempts++ {
			packet = streamUntil(t, c, "game")
			if packet["game"].(map[string]any)["data"].(map[string]any)["Presentation"] == float64(1) {
				break
			}
		}
		game := packet["game"].(map[string]any)
		if game["paused"] != true || game["data"].(map[string]any)["Physics"] != float64(0) {
			t.Fatal("incorrect paused stream", packet)
		}
		h.frame(time.UnixMilli(due))
		if h.state().Presentation != 1 || h.counts.paused.Load() != 1 {
			t.Fatal("same due time replayed callback")
		}
		h.s.mu.Lock()
		err := h.s.flushRealtimeCheckpoints(time.Now())
		h.s.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		h.trace = append(h.trace, map[string]any{"ack": ack, "duplicate": repeated, "packet": packet, "stored": pauseStoredRoom(t, h.realtimeTable)})
	})
	t.Run("expired-on-startup", func(t *testing.T) {
		h := newPausedTimerHTTP(t, "normal", false, false)
		h.action("pause")
		h.s.mu.Lock()
		h.s.rooms[h.id].LastActive = time.Now().Add(-31 * 24 * time.Hour).Unix()
		err := h.s.save(h.s.rooms[h.id])
		h.s.mu.Unlock()
		if err != nil {
			t.Fatal(err)
		}
		h.restart()
		if current(h.a)["status"] != "closed" || h.counts.paused.Load() != 0 {
			t.Fatal("startup presentation evaded expiration")
		}
		h.trace = append(h.trace, current(h.a))
	})
}
