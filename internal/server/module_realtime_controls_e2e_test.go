package server

// Failure scenarios written before stale realtime lifecycle controls: a valid
// host cannot close a room while 100 ms frames advance its revision; an exited
// racer cannot leave; relaxing version checks bypasses host/seat/elimination
// permissions; future, waiting-room and previous-race requests become valid;
// ordinary games unexpectedly admit stale controls; gameplay action versions
// are weakened. HTTP commands create, ready, start, close and restart the room.
import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func realtimeControlTable(t *testing.T, module gameapi.Module, live bool) (*realtimeTable, *testClient, map[string]any) {
	t.Helper()
	f := &realtimeTable{cfg: Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{module}}}
	var err error
	f.s, err = New(f.cfg, fstest.MapFS{"index.html": {Data: []byte("realtime lifecycle HTTP")}})
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
	f.a.register("实时房主控制")
	b := newClient(t, f.ts.URL)
	b.register("实时普通席位")
	r := f.a.post("/api/rooms", map[string]any{"kind": module.ID, "name": "结束与离桌验收", "capacity": 4}, 201)
	f.id = r["id"].(string)
	b.command(r, "join", nil, 200)
	for i := 0; i < 2; i++ {
		f.a.command(current(f.a), "add_bot", nil, 200)
	}
	waiting := current(f.a)
	f.a.command(waiting, "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	f.a.command(current(f.a), "start", nil, 200)
	return f, b, waiting
}

func TestModuleRealtimeStaleControlLiveHTTP(t *testing.T) {
	trace := []any{}
	realtimeReport(t, &trace)
	f, b, waiting := realtimeControlTable(t, realtimeFixture(true, 100*time.Millisecond, 0, ""), true)
	viewer := newClient(t, f.ts.URL)
	viewer.register("实时观众控制")
	viewer.post("/api/rooms/"+f.id+"/watch", map[string]any{}, 200)
	old := current(f.a)
	var latest map[string]any
	for limit := time.Now().Add(2 * time.Second); time.Now().Before(limit); {
		latest = current(f.a)
		if latest["version"].(float64) >= old["version"].(float64)+2 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if latest["version"].(float64) < old["version"].(float64)+2 {
		t.Fatal("natural frames did not make the control stale")
	}
	// These requests use one deliberately stale snapshot; no retry or fresh
	// version lookup occurs between sending a command and accepting its result.
	trace = append(trace, map[string]any{"oldVersion": old["version"], "advancedVersion": latest["version"]})
	for _, kind := range []string{"close", "leave"} {
		trace = append(trace, map[string]any{"command": kind, "case": "prestart_snapshot", "result": f.a.command(waiting, kind, nil, 409)})
	}
	future := map[string]any{"id": f.id, "version": latest["version"].(float64) + 100000}
	for _, kind := range []string{"close", "leave"} {
		f.a.command(future, kind, nil, 409)
	}
	b.command(old, "close", nil, 400)
	viewer.command(old, "close", nil, 400)
	viewer.command(old, "leave", nil, 400)
	f.a.command(old, "leave", nil, 400) // Active racers remain bound to their seat.
	f.a.command(old, "action", map[string]any{"drive": 1}, 409)
	trace = append(trace, map[string]any{"command": "close", "case": "same_race_stale_host", "result": f.a.command(old, "close", nil, 200)})
	closed := current(f.a)
	if closed["status"] != "closed" {
		t.Fatal("single stale close did not end the live room")
	}
	trace = append(trace, closed)
}

func TestModuleRealtimeStaleControlSessionBoundsHTTP(t *testing.T) {
	trace := []any{}
	realtimeReport(t, &trace)
	f, b, waiting := realtimeControlTable(t, realtimeFixture(true, time.Hour, 0, ""), false)
	first := current(f.a)
	due := f.s.rooms[f.id].Module.SystemDeadline
	f.frame(time.UnixMilli(due))
	f.a.command(first, "close", nil, 200)
	f.a.command(current(f.a), "rematch", nil, 200)
	rematchWaiting := current(f.a)
	f.a.command(rematchWaiting, "ready", nil, 200)
	for _, kind := range []string{"close", "leave"} {
		f.a.command(rematchWaiting, kind, nil, 409) // Waiting rooms retain exact-version semantics.
		f.a.command(first, kind, nil, 409)
	}
	b.command(current(b), "ready", nil, 200)
	f.a.command(current(f.a), "start", nil, 200)
	second := current(f.a)
	if f.s.rooms[f.id].SetupVersion <= int(first["version"].(float64)) {
		t.Fatal("rematch did not establish a new revision boundary")
	}
	for _, old := range []map[string]any{waiting, first, rematchWaiting} {
		for _, kind := range []string{"close", "leave"} {
			trace = append(trace, map[string]any{"version": old["version"], "command": kind, "result": f.a.command(old, kind, nil, 409)})
		}
	}
	f.frame(time.UnixMilli(f.s.rooms[f.id].Module.SystemDeadline))
	f.a.command(second, "close", nil, 200)
	trace = append(trace, current(f.a))
	// The same snapshot aging in an ordinary module remains a 409; only a
	// current-version close succeeds. No rules/gameplay callback is relaxed.
	g, _, _ := realtimeControlTable(t, realtimeFixture(false, time.Hour, 0, ""), false)
	old := current(g.a)
	g.s.mu.Lock()
	g.s.expireModuleTimers(time.UnixMilli(g.s.rooms[g.id].Module.SystemDeadline))
	g.s.mu.Unlock()
	for _, kind := range []string{"close", "leave"} {
		g.a.command(old, kind, nil, 409)
	}
	g.a.command(current(g.a), "close", nil, 200)
	trace = append(trace, map[string]any{"ordinaryStaleControlsRejected": true})
}

func TestModuleRealtimeStaleEliminatedLeaveHTTP(t *testing.T) {
	trace := []any{}
	realtimeReport(t, &trace)
	m := realtimeFixture(true, time.Hour, 0, "")
	onTimer := m.OnTimer
	m.OnTimer = func(c gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		next, err := onTimer(c, match)
		if err == nil {
			next.Actors = []int{1, 2, 3}
			next.Eliminated = []int{0}
		}
		return next, err
	}
	f, b, _ := realtimeControlTable(t, m, false)
	old := current(f.a)
	due := f.s.rooms[f.id].Module.SystemDeadline
	f.frame(time.UnixMilli(due))
	before := current(f.a)
	trace = append(trace, map[string]any{"before": before, "staleVersion": old["version"]})
	f.a.command(old, "leave", nil, 200)
	if f.a.state()["room"] != nil {
		t.Fatal("eliminated racer remained bound after stale leave")
	}
	after := current(b)
	if after["status"] != "playing" || after["host"] != b.state()["user"].(map[string]any)["id"] {
		t.Fatal("departure reset match or lost host transfer")
	}
	seats := after["seats"].([]any)
	if len(seats) != 4 || seats[0].(map[string]any)["left"] != true {
		t.Fatal("departure discarded original roster")
	}
	f.frame(time.UnixMilli(due + 100))
	if realtimeData(t, current(b)).Frames != 2 {
		t.Fatal("remaining racers stopped after departure")
	}
	// No saved snapshot injection was used; elimination was the fixture's
	// ordinary OnTimer result, persisted by the same atomic frame transaction.
	raw, _ := json.Marshal(after)
	trace = append(trace, json.RawMessage(raw))
}
