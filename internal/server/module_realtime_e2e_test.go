package server

// Failure scenarios specified before implementation: a realtime module still
// runs at 1 Hz; normal games are accidentally accelerated; the first bot starves
// later seats; idle bots or non-actors are applied; a timeout takes over the
// wrong seat; cancellation leaves a previous bot input active; a rejected bot,
// timer or SQL write leaks part of a frame; duplicate ticks duplicate history;
// a closed/restored room advances unexpectedly; timer-only/timeout activity
// renews an abandoned room; a frame writes one transaction per bot instead of
// one audited atomic update. All rooms and controls use authenticated HTTP.
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type realtimePosition struct {
	Frames int   `json:"frames"`
	Inputs []int `json:"inputs"`
	Drive  []int `json:"drive"`
}

func realtimeFixture(realtime bool, first time.Duration, finish int, fail string) gameapi.Module {
	m := acceptanceModule()
	m.ID, m.Name, m.MinPlayers, m.MaxPlayers = "realtime_fixture", "实时帧验收", 4, 4
	m.Realtime = realtime
	m.Configure = func(_ int, _ json.RawMessage) (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
	m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		return gameapi.Match{Schema: 1, Data: json.RawMessage(`{"frames":0,"inputs":[0,0,0,0],"drive":[0,0,0,0]}`), Actors: []int{0, 1, 2, 3}, Attention: []int{}, SystemDeadline: c.Now.Add(first).UnixMilli()}, nil
	}
	m.View = func(match gameapi.Match, _ int) (any, error) {
		var p realtimePosition
		err := json.Unmarshal(match.Data, &p)
		return p, err
	}
	m.Apply = func(c gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var action struct {
			Drive   int  `json:"drive"`
			Timeout bool `json:"timeout"`
		}
		if json.Unmarshal(raw, &action) != nil || action.Drive != 1 {
			return match, errors.New("invalid drive")
		}
		var p realtimePosition
		if err := json.Unmarshal(match.Data, &p); err != nil {
			return match, err
		}
		p.Inputs[c.Actor]++
		p.Drive[c.Actor] = 1
		match.Data, _ = json.Marshal(p)
		if fail == "apply" && c.Actor == 2 {
			return match, errors.New("injected late actor rejection")
		}
		if action.Timeout {
			match.TimeoutActors, match.Deadline = []int{c.Actor}, c.Now.Add(-time.Millisecond).UnixMilli()
		}
		return match, nil
	}
	m.BotAction = func(c gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
		if fail == "idle" && c.Actor == 1 {
			return nil, nil
		}
		if fail == "bot" && c.Actor == 2 {
			match.Data[0] = '!'
			return nil, errors.New("injected bot failure")
		}
		return json.RawMessage(`{"drive":1}`), nil
	}
	m.Reclaim = func(c gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		var p realtimePosition
		if err := json.Unmarshal(match.Data, &p); err != nil {
			return match, err
		}
		p.Drive[c.Actor] = 0
		match.Data, _ = json.Marshal(p)
		if match.Deadline > 0 {
			match.Deadline = c.Now.Add(time.Hour).UnixMilli()
		}
		return match, nil
	}
	m.OnTimer = func(c gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		if c.Actor != -1 || c.Now.UnixMilli() < match.SystemDeadline {
			return match, errors.New("invalid frame clock")
		}
		var p realtimePosition
		if err := json.Unmarshal(match.Data, &p); err != nil {
			return match, err
		}
		p.Frames++
		match.Data, _ = json.Marshal(p)
		match.SystemDeadline = c.Now.Add(100 * time.Millisecond).UnixMilli()
		if fail == "timer" {
			return match, errors.New("injected timer failure")
		}
		if fail == "deadline" {
			match.SystemDeadline = c.Now.UnixMilli()
		}
		if finish > 0 && p.Frames == finish {
			match.Actors, match.Attention, match.TimeoutActors, match.Deadline, match.SystemDeadline = nil, nil, nil, 0, 0
			match.Result = &gameapi.Result{Ranks: []int{1, 2, 3, 4}, Winners: []int{0}}
		}
		return match, nil
	}
	return m
}

type realtimeTable struct {
	s   *Server
	ts  *httptest.Server
	a   *testClient
	cfg Config
	id  string
}

func newRealtimeTable(t *testing.T, module gameapi.Module, live bool) *realtimeTable {
	t.Helper()
	f := &realtimeTable{cfg: Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{module}}}
	var err error
	f.s, err = New(f.cfg, fstest.MapFS{"index.html": {Data: []byte("realtime host E2E")}})
	if err != nil {
		t.Fatal(err)
	}
	if !live {
		f.s.cancel()
		<-f.s.done
	}
	f.ts = httptest.NewServer(f.s.Handler())
	t.Cleanup(func() { f.ts.Close(); f.s.Close() })
	publishModuleForTest(t, f.ts.URL, module.ID)
	f.a = newClient(t, f.ts.URL)
	f.a.register("实时帧验收玩家")
	room := f.a.post("/api/rooms", map[string]any{"kind": module.ID, "name": "实时帧验收", "capacity": 4}, 201)
	f.id = room["id"].(string)
	for i := 0; i < 3; i++ {
		f.a.command(current(f.a), "add_bot", nil, 200)
	}
	f.a.command(current(f.a), "ready", nil, 200)
	f.a.command(current(f.a), "start", nil, 200)
	return f
}

func realtimeData(t *testing.T, room map[string]any) realtimePosition {
	t.Helper()
	raw, err := json.Marshal(room["game"].(map[string]any)["data"])
	if err != nil {
		t.Fatal(err)
	}
	var p realtimePosition
	if err = json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func realtimeReport(t *testing.T, trace *[]any) {
	t.Helper()
	t.Cleanup(func() {
		out := filepath.Join("..", "..", ".local", "racing", "host")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, err := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "platform realtime contract via HTTP and production clock; no racing physics", "trace": *trace}, "", "  ")
		if err != nil {
			t.Error(err)
			return
		}
		if err = os.WriteFile(filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
}

func (f *realtimeTable) frame(at time.Time) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.expireRealtimeModuleTimers(at)
}

func TestModuleRealtimeLiveClockHTTP(t *testing.T) {
	trace := []any{}
	realtimeReport(t, &trace)
	for _, realtime := range []bool{true, false} {
		f := newRealtimeTable(t, realtimeFixture(realtime, 100*time.Millisecond, 0, ""), true)
		start := time.Now()
		var reached realtimePosition
		for until := start.Add(1400 * time.Millisecond); time.Now().Before(until); {
			reached = realtimeData(t, current(f.a))
			if realtime && reached.Frames >= 5 {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		elapsed := time.Since(start)
		trace = append(trace, map[string]any{"realtime": realtime, "elapsedMillis": elapsed.Milliseconds(), "data": reached})
		if realtime {
			if reached.Frames < 5 || elapsed >= time.Second {
				t.Fatal("realtime clock still advances at legacy speed", reached, elapsed)
			}
			for _, actor := range []int{1, 2, 3} {
				if reached.Inputs[actor] != reached.Frames {
					t.Fatal("controlled actor starved or scheduled twice", reached)
				}
			}
		} else if reached.Frames > 2 || reached.Inputs[1]+reached.Inputs[2]+reached.Inputs[3] > 2 {
			t.Fatal("ordinary module was accelerated", reached)
		}
	}
}

func TestModuleRealtimeAtomicFrameHTTP(t *testing.T) {
	for _, mode := range []string{"success", "idle", "bot", "apply", "timer", "deadline", "database"} {
		t.Run(mode, func(t *testing.T) {
			trace := []any{}
			realtimeReport(t, &trace)
			module := realtimeFixture(true, time.Hour, 0, mode)
			timer := module.OnTimer
			module.OnTimer = func(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
				next, err := timer(c, m)
				next.CheckpointKey = "critical-stage"
				return next, err
			}
			f := newRealtimeTable(t, module, false)
			before := current(f.a)
			var persisted []byte
			if err := f.s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", f.id).Scan(&persisted); err != nil {
				t.Fatal(err)
			}
			var countBefore int
			if err := f.s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=?", f.id).Scan(&countBefore); err != nil {
				t.Fatal(err)
			}
			if mode == "database" {
				if _, err := f.s.db.Exec("CREATE TRIGGER fail_frame BEFORE INSERT ON actions BEGIN SELECT RAISE(ABORT, 'injected frame failure'); END"); err != nil {
					t.Fatal(err)
				}
			}
			due := f.s.rooms[f.id].Module.SystemDeadline
			f.frame(time.UnixMilli(due - 1))
			if !reflect.DeepEqual(before, current(f.a)) {
				t.Fatal("early frame executed")
			}
			f.frame(time.UnixMilli(due))
			f.frame(time.UnixMilli(due))
			after := current(f.a)
			trace = append(trace, map[string]any{"before": before, "after": after})
			var saved []byte
			if err := f.s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", f.id).Scan(&saved); err != nil {
				t.Fatal(err)
			}
			var countAfter int
			if err := f.s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=?", f.id).Scan(&countAfter); err != nil {
				t.Fatal(err)
			}
			if mode != "success" && mode != "idle" {
				if !reflect.DeepEqual(before, after) || string(persisted) != string(saved) || countBefore != countAfter {
					t.Fatal("failed frame leaked state or journal")
				}
				return
			}
			p := realtimeData(t, after)
			want := []int{0, 1, 1, 1}
			if mode == "idle" {
				want[1] = 0
			}
			if p.Frames != 1 || !reflect.DeepEqual(p.Inputs, want) || after["version"].(float64) != before["version"].(float64)+1 || countAfter != countBefore+1 {
				t.Fatal("frame not one atomic fair update", p, countBefore, countAfter)
			}
			var record []byte
			if err := f.s.db.QueryRow("SELECT action FROM actions WHERE room_id=? ORDER BY version DESC LIMIT 1", f.id).Scan(&record); err != nil {
				t.Fatal(err)
			}
			var event struct {
				Type     string `json:"type"`
				Deadline int64  `json:"deadline"`
				Inputs   []struct {
					Actor  int             `json:"actor"`
					Action json.RawMessage `json:"action"`
				} `json:"inputs"`
			}
			if err := json.Unmarshal(record, &event); err != nil {
				t.Fatal(err)
			}
			if event.Type != "realtime_frame" || event.Deadline != due || len(event.Inputs) != 2+want[1] {
				t.Fatal("frame audit omitted inputs or timer", string(record))
			}
			for _, input := range event.Inputs {
				if input.Actor < 1 || input.Actor > 3 || string(input.Action) != `{"drive":1}` {
					t.Fatal("invalid audited input", string(record))
				}
			}
		})
	}
}

func TestModuleRealtimeControlRestoreAndClosureHTTP(t *testing.T) {
	trace := []any{}
	realtimeReport(t, &trace)
	f := newRealtimeTable(t, realtimeFixture(true, time.Hour, 0, ""), false)
	setAutoPlay(f.a, current(f.a), true, 200)
	due := f.s.rooms[f.id].Module.SystemDeadline
	f.frame(time.UnixMilli(due))
	p := realtimeData(t, current(f.a))
	if !reflect.DeepEqual(p.Inputs, []int{1, 1, 1, 1}) {
		t.Fatal("explicit controlled player not included", p)
	}
	setAutoPlay(f.a, current(f.a), false, 200)
	p = realtimeData(t, current(f.a))
	if p.Drive[0] != 0 {
		t.Fatal("cancelling control retained bot drive")
	}
	f.frame(time.UnixMilli(due + 100))
	p = realtimeData(t, current(f.a))
	if !reflect.DeepEqual(p.Inputs, []int{1, 2, 2, 2}) {
		t.Fatal("cancellation resurrected old bot control", p)
	}
	f.a.command(current(f.a), "action", map[string]any{"drive": 1, "timeout": true}, 200)
	f.frame(time.UnixMilli(due + 200))
	room := current(f.a)
	if room["seats"].([]any)[0].(map[string]any)["autoPlay"] == true || f.s.rooms[f.id].Seats[0].TimeoutAutoPlay || f.s.rooms[f.id].Module.Deadline != 0 {
		t.Fatal("idle realtime human was taken over")
	}
	setAutoPlay(f.a, room, true, 200)
	setAutoPlay(f.a, current(f.a), false, 200)
	if f.s.rooms[f.id].Seats[0].AutoPlay || f.s.rooms[f.id].Module.Deadline != 0 || realtimeData(t, current(f.a)).Drive[0] != 0 {
		t.Fatal("manual cancellation did not reclaim without a turn clock")
	}
	// Restart the real persisted room; the next deadline is in the future and
	// no fixture state is injected. HTTP sessions survive the reopened database.
	saved := current(f.a)
	f.ts.Close()
	f.s.Close()
	var err error
	f.s, err = New(f.cfg, fstest.MapFS{"index.html": {Data: []byte("restored realtime")}})
	if err != nil {
		t.Fatal(err)
	}
	f.s.cancel()
	<-f.s.done
	f.ts = httptest.NewServer(f.s.Handler())
	f.a.base = f.ts.URL
	if !reflect.DeepEqual(saved, current(f.a)) {
		t.Fatal("restart changed authoritative frame")
	}
	f.frame(time.UnixMilli(due + 300))
	if realtimeData(t, current(f.a)).Frames != 4 {
		t.Fatal("restored deadline failed")
	}
	f.a.command(current(f.a), "close", nil, 200)
	closed := current(f.a)
	f.frame(time.UnixMilli(due + 400))
	if !reflect.DeepEqual(closed, current(f.a)) {
		t.Fatal("closed room advanced")
	}
	trace = append(trace, saved, closed)
}

func TestModuleRealtimeResultAndInactivityHTTP(t *testing.T) {
	trace := []any{}
	realtimeReport(t, &trace)
	f := newRealtimeTable(t, realtimeFixture(true, time.Hour, 2, ""), false)
	due := f.s.rooms[f.id].Module.SystemDeadline
	f.frame(time.UnixMilli(due))
	f.frame(time.UnixMilli(due + 100))
	f.frame(time.UnixMilli(due + 200))
	if current(f.a)["status"] != "finished" {
		t.Fatal("realtime result not archived")
	}
	var histories, ratings int
	if err := f.s.db.QueryRow("SELECT COUNT(*) FROM match_history WHERE id=?", f.s.rooms[f.id].MatchID).Scan(&histories); err != nil {
		t.Fatal(err)
	}
	if err := f.s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger").Scan(&ratings); err != nil {
		t.Fatal(err)
	}
	if histories != 1 || ratings != 0 {
		t.Fatal("duplicate history or rated bot game", histories, ratings)
	}
	trace = append(trace, current(f.a))
	// A timer-only fixture and a timed-out human must not count as activity.
	for _, timeout := range []bool{false, true} {
		m := realtimeFixture(true, time.Hour, 0, "")
		m.BotAction = func(c gameapi.Context, _ gameapi.Match) (json.RawMessage, error) {
			if c.Actor == 0 && timeout {
				return json.RawMessage(`{"drive":1}`), nil
			}
			return nil, nil
		}
		g := newRealtimeTable(t, m, false)
		if timeout {
			g.a.command(current(g.a), "action", map[string]any{"drive": 1, "timeout": true}, 200)
		}
		before := g.s.rooms[g.id].LastActive
		d := g.s.rooms[g.id].Module.SystemDeadline
		g.frame(time.UnixMilli(d))
		if g.s.rooms[g.id].LastActive != before {
			t.Fatal("automatic frame renewed abandoned room", timeout)
		}
		trace = append(trace, map[string]any{"timeout": timeout, "lastActive": before, "data": realtimeData(t, current(g.a))})
	}
}

func TestModuleRealtimeRegistrationContract(t *testing.T) {
	m := realtimeFixture(true, time.Hour, 0, "")
	m.OnTimer = nil
	if m.Validate() == nil {
		t.Fatal("realtime without clock accepted")
	}
	m = realtimeFixture(true, time.Hour, 0, "")
	m.BotSearch = asyncFixture(func(context.Context, json.RawMessage) (json.RawMessage, error) { return nil, nil }).BotSearch
	if m.Validate() == nil {
		t.Fatal("unbounded async work mixed into realtime frames")
	}
	m = realtimeFixture(true, time.Hour, 0, "")
	raw, err := json.Marshal(m)
	var public map[string]any
	_ = json.Unmarshal(raw, &public)
	if err != nil || public["realtime"] != nil || public["Realtime"] != nil {
		t.Fatal(fmt.Sprint("internal scheduler capability leaked", string(raw), err))
	}
}
