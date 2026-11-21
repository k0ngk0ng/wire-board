package starfort_test

// Failure inventory and budgets frozen before implementation: late histories
// keep obsolete target snapshots in each save/timer/view; the one-second cutoff
// loses attacks, changes damage or resets event IDs; legacy schema/rules change;
// a projection mutates input; player/spectator controls become shared; stored
// state grows again over many waves; SQLite restart loses progress; socket
// reconnect stalls notification; or invalid old events are silently discarded.
// Public Module replay isolates allocation from HTTP/SQLite noise. The second
// scenario uses real authenticated HTTP/WebSocket clients and a live database.
// Normal captured late-game budgets: 128 KiB saved Data, 512 KiB allocated per
// timer or per player view (including the host's Match JSON round trip).

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func memoryEvidence(t *testing.T, label string, value any) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", ".local", "starfort-memory-fix")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, label+".json"), append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func memoryView(t *testing.T, m gameapi.Match, actor int) []byte {
	t.Helper()
	v, err := starfort.Module().View(m, actor)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestStarfortMemoryProductionReplay(t *testing.T) {
	legacy := stalledCraterMatch(t)
	original := bytes.Clone(legacy.Data)
	var state starfort.State
	if err := json.Unmarshal(legacy.Data, &state); err != nil {
		t.Fatal(err)
	}
	compact, err := state.Envelope()
	if err != nil {
		t.Fatal(err)
	}
	if compact.Schema != legacy.Schema {
		t.Fatal("old game rules migrated")
	}
	for _, actor := range []int{-1, 0, 1} {
		if !bytes.Equal(memoryView(t, legacy, actor), memoryView(t, compact, actor)) {
			t.Fatalf("actor %d lost visible events, state or controls", actor)
		}
	}
	var oldFields, newFields map[string]json.RawMessage
	_ = json.Unmarshal(legacy.Data, &oldFields)
	_ = json.Unmarshal(compact.Data, &newFields)
	delete(oldFields, "events")
	delete(newFields, "events")
	if !reflect.DeepEqual(oldFields, newFields) {
		t.Fatal("compaction changed gameplay state")
	}
	if !bytes.Equal(original, legacy.Data) {
		t.Fatal("projection mutated the old archive")
	}
	if len(compact.Data) > 128<<10 {
		t.Errorf("captured late-game snapshot %d exceeds 128 KiB", len(compact.Data))
	}
	mod := starfort.Module()
	n := len(state.Players)
	ctx := gameapi.Context{Players: n, Actor: -1, Now: time.UnixMilli(compact.SystemDeadline)}
	// Timer output must match the legacy input's next public state exactly.
	oldNext, err := mod.OnTimer(ctx, legacy)
	if err != nil {
		t.Fatal(err)
	}
	newNext, err := mod.OnTimer(ctx, compact)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(memoryView(t, oldNext, 0), memoryView(t, newNext, 0)) {
		t.Fatal("dropping obsolete presentation events changed combat")
	}
	allocated := map[string]uint64{}
	for _, name := range []string{"timer", "view"} {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		for i := 0; i < 32; i++ {
			if name == "timer" {
				if _, err := mod.OnTimer(ctx, compact); err != nil {
					t.Fatal(err)
				}
			} else {
				raw, err := json.Marshal(compact)
				if err != nil {
					t.Fatal(err)
				}
				var detached gameapi.Match
				if err = json.Unmarshal(raw, &detached); err != nil {
					t.Fatal(err)
				}
				memoryView(t, detached, 0)
			}
		}
		runtime.ReadMemStats(&after)
		allocated[name] = (after.TotalAlloc - before.TotalAlloc) / 32
		if allocated[name] > 512<<10 {
			t.Errorf("late-game %s allocated %d bytes/op, budget 512 KiB", name, allocated[name])
		}
	}
	match := compact
	maxBytes := len(match.Data)
	lastEventID := state.EventSerial
	for second := 0; second < 300 && match.SystemDeadline > 0; second++ {
		ctx.Now = time.UnixMilli(match.SystemDeadline)
		match, err = mod.OnTimer(ctx, match)
		if err != nil {
			t.Fatal(err)
		}
		maxBytes = max(maxBytes, len(match.Data))
		var saved starfort.State
		if err = json.Unmarshal(match.Data, &saved); err != nil {
			t.Fatal(err)
		}
		if saved.EventSerial < lastEventID {
			t.Fatal("event cursor moved backwards")
		}
		lastEventID = saved.EventSerial
		for i, event := range saved.Events {
			at := event.Tick * starfort.StepMS
			if event.AtMS != nil {
				at = *event.AtMS
			}
			if i < len(saved.Events)-1 && at <= saved.ElapsedMS-1000 {
				t.Fatal("obsolete target snapshot returned to the save")
			}
		}
	}
	if maxBytes > 128<<10 {
		t.Errorf("replay snapshot grew to %d bytes", maxBytes)
	}
	memoryEvidence(t, "module-replay", map[string]any{"passed": !t.Failed(), "sourceBytes": len(original), "compactBytes": len(compact.Data), "maxReplayBytes": maxBytes, "allocatedBytesPerOperation": allocated, "schema": compact.Schema})
}

func TestStarfortMemoryHTTPPersistenceAndRestart(t *testing.T) {
	mod := starfort.Module()
	mod.Start = func(ctx gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		match := stalledCraterMatch(t)
		var state starfort.State
		if err := json.Unmarshal(match.Data, &state); err != nil {
			return match, err
		}
		state.ServerAt = ctx.Now.UnixMilli()
		// Deliberately import the complete old ring, bypassing Envelope, to prove
		// that the first normal host timer replaces an existing large save.
		var err error
		match.Data, err = json.Marshal(state)
		match.SystemDeadline = state.ServerAt + 1000
		return match, err
	}
	tb := newHost(t, 2, mod)
	tb.start(nil)
	a := tb.people[0]
	id := a.room()["id"].(string)
	readSaved := func() (gameapi.Match, int) {
		t.Helper()
		db, err := sql.Open("sqlite", "file:"+filepath.Join(tb.cfg.DataDir, "wire-board.db")+"?mode=ro")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var raw []byte
		if err = db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var room struct{ Module gameapi.Match }
		if err = json.Unmarshal(raw, &room); err != nil {
			t.Fatal(err)
		}
		return room.Module, len(raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	connect := func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(a.base, "http")+"/api/ws", &websocket.DialOptions{HTTPClient: a.http})
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err = conn.Read(ctx); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	conn := connect()
	initialTick := data(a.room())["tick"].(float64)
	wait(t, "first compacted timer", func() bool { return data(a.room())["tick"].(float64) > initialTick })
	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatal("no socket update", err)
	}
	conn.CloseNow()
	saved, bytesBeforeRestart := readSaved()
	if len(saved.Data) > 128<<10 {
		t.Errorf("SQLite still stores %d bytes of game data", len(saved.Data))
	}
	for _, person := range append(tb.people, tb.spectator) {
		if data(person.room())["tick"].(float64) <= initialTick {
			t.Fatal("viewer did not see the committed battle")
		}
	}
	tb.http.Close()
	if err := tb.host.Close(); err != nil {
		t.Fatal(err)
	}
	persisted, _ := readSaved()
	var previous starfort.State
	_ = json.Unmarshal(persisted.Data, &previous)
	host, err := server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Starfort memory recovery")}})
	if err != nil {
		t.Fatal(err)
	}
	tb.host, tb.http = host, httptest.NewServer(host.Handler())
	for _, person := range append(tb.people, tb.spectator) {
		person.base = tb.http.URL
	}
	restored := data(a.room())
	if restored["tick"].(float64) < float64(previous.Tick) || restored["wave"] != float64(previous.Wave) {
		t.Fatal("restart lost progress")
	}
	if len(restored["towers"].([]any)) != len(previous.Towers) {
		t.Fatal("restart lost tower investments")
	}
	conn = connect()
	defer conn.CloseNow()
	tick := data(a.room())["tick"].(float64)
	wait(t, "restored timer", func() bool { return data(a.room())["tick"].(float64) > tick })
	if _, _, err = conn.Read(ctx); err != nil {
		t.Fatal("reconnected socket stalled", err)
	}
	after, bytesAfterRestart := readSaved()
	if len(after.Data) > 128<<10 || after.Schema != persisted.Schema {
		t.Fatal("restored save grew or changed rules")
	}
	tb.record("legacy full event ring compacted through HTTP/timer/SQLite; players, spectator and reconnected socket continue after restart")
	memoryEvidence(t, "http-restart", map[string]any{"passed": !t.Failed(), "sqliteBytesBeforeRestart": bytesBeforeRestart, "sqliteBytesAfterRestart": bytesAfterRestart, "restoredTick": previous.Tick, "schema": after.Schema})
}
