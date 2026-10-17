package server

// Failure inventory before implementation: missing/wrong-length seat metadata;
// bots omitted; manual/timeout takeover or reclaim ordering misrepresented;
// forged HTTP/spectator flags accepted; restored seats lose control state;
// callbacks share slices or mutation leaks into later callbacks/room seats.
// Commands, roster creation and restored reads use authenticated HTTP. Only the
// production frame clock is stepped explicitly; no room state is injected.
import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type automatedContextRecord struct {
	Callback  string `json:"callback"`
	Actor     int    `json:"actor"`
	Players   int    `json:"players"`
	Automated []bool `json:"automated"`
}

func TestModuleAutomatedSeatContextHTTP(t *testing.T) {
	var mu sync.Mutex
	var records []automatedContextRecord
	var refs [][]bool
	record := func(name string, c gameapi.Context) {
		mu.Lock()
		defer mu.Unlock()
		records = append(records, automatedContextRecord{name, c.Actor, c.Players, append([]bool(nil), c.Automated...)})
		refs = append(refs, c.Automated)
		// Deliberately hostile callback: host-derived metadata must be detached.
		for i := range c.Automated {
			c.Automated[i] = !c.Automated[i]
		}
	}
	snapshot := func() []automatedContextRecord {
		mu.Lock()
		defer mu.Unlock()
		return append([]automatedContextRecord(nil), records...)
	}
	t.Cleanup(func() {
		out := filepath.Join("..", "..", ".local", "module-context")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "authenticated HTTP lifecycle with production timer entry; no game rules", "callbacks": snapshot()}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, "automated-http.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	m := realtimeFixture(true, time.Minute, 0, "")
	start, apply, bot, timer, reclaim := m.Start, m.Apply, m.BotAction, m.OnTimer, m.Reclaim
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		record("Start", c)
		return start(c, o)
	}
	m.Apply = func(c gameapi.Context, v gameapi.Match, a json.RawMessage) (gameapi.Match, error) {
		record("Apply", c)
		return apply(c, v, a)
	}
	m.BotAction = func(c gameapi.Context, v gameapi.Match) (json.RawMessage, error) {
		record("BotAction", c)
		return bot(c, v)
	}
	m.OnTimer = func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) {
		record("OnTimer", c)
		return timer(c, v)
	}
	m.Reclaim = func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) {
		record("Reclaim", c)
		return reclaim(c, v)
	}
	m.Restore = func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) { record("Restore", c); return v, nil }
	f, b, _ := realtimeControlTable(t, m, false)
	check := func(from int, want []bool, names ...string) {
		t.Helper()
		rows := snapshot()[from:]
		found := map[string]bool{}
		for _, r := range rows {
			if r.Players != 4 || !reflect.DeepEqual(r.Automated, want) {
				t.Fatalf("bad metadata %+v want %v", r, want)
			}
			found[r.Callback] = true
		}
		for _, name := range names {
			if !found[name] {
				t.Fatalf("missing %s callback: %+v", name, rows)
			}
		}
	}
	manual := []bool{false, false, true, true}
	controlledB := []bool{false, true, true, true}
	check(0, manual, "Start")
	viewer := newClient(t, f.ts.URL)
	viewer.register("席位元数据观众")
	viewer.post("/api/rooms/"+f.id+"/watch", map[string]any{}, 200)
	before := len(snapshot())
	room := current(f.a)
	viewer.command(room, "action", map[string]any{"drive": 1, "automated": []bool{true, true, false, false}}, 400)
	setAutoPlay(viewer, room, true, 400)
	if len(snapshot()) != before {
		t.Fatal("spectator reached module callback")
	}
	// Even an authenticated action cannot replace host-derived Context fields.
	f.a.command(current(f.a), "action", map[string]any{"drive": 1, "automated": []bool{true, true, false, false}}, 200)
	check(before, manual, "Apply")
	setAutoPlay(b, current(b), true, 200)
	before = len(snapshot())
	due := f.s.rooms[f.id].Module.SystemDeadline
	f.frame(time.UnixMilli(due))
	check(before, controlledB, "BotAction", "Apply", "OnTimer")
	before = len(snapshot())
	setAutoPlay(b, current(b), false, 200)
	check(before, controlledB, "Reclaim") // Flag changes after Reclaim transaction callback.
	before = len(snapshot())
	f.frame(time.UnixMilli(due + 100))
	check(before, manual, "BotAction", "OnTimer")
	before = len(snapshot())
	f.a.command(current(f.a), "action", map[string]any{"drive": 1, "timeout": true}, 200)
	check(before, manual, "Apply")
	before = len(snapshot())
	f.frame(time.UnixMilli(due + 200))
	check(before, []bool{true, false, true, true}, "BotAction", "OnTimer")
	before = len(snapshot())
	setAutoPlay(f.a, current(f.a), false, 200)
	check(before, []bool{true, false, true, true}, "Reclaim")
	before = len(snapshot())
	f.frame(time.UnixMilli(due + 300))
	check(before, manual, "OnTimer")
	setAutoPlay(b, current(b), true, 200)
	saved := current(f.a)
	seats := saved["seats"].([]any)
	for i, s := range seats {
		v := s.(map[string]any)
		if (v["bot"] == true) != (i >= 2) || (v["autoPlay"] == true) != (i == 1) {
			t.Fatal("callback mutation changed host seats", seats)
		}
	}
	before = len(snapshot())
	f.ts.Close()
	f.s.Close()
	var err error
	f.s, err = New(f.cfg, fstest.MapFS{"index.html": {Data: []byte("restored automated context")}})
	if err != nil {
		t.Fatal(err)
	}
	f.s.cancel()
	<-f.s.done
	f.ts = httptest.NewServer(f.s.Handler())
	f.a.base, b.base = f.ts.URL, f.ts.URL
	check(before, controlledB, "Restore")
	if !reflect.DeepEqual(saved, current(f.a)) {
		t.Fatal("restore changed authoritative room")
	}
	before = len(snapshot())
	f.frame(time.UnixMilli(due + 400))
	check(before, controlledB, "BotAction", "OnTimer")
	mu.Lock()
	defer mu.Unlock()
	pointers := map[*bool]bool{}
	for _, r := range refs {
		if len(r) != 4 {
			t.Fatal("missing detached slice")
		}
		if pointers[&r[0]] {
			t.Fatal("callbacks reused automated slice")
		}
		pointers[&r[0]] = true
	}
	t.Logf("verified %d independent callback contexts through HTTP, takeover, reclaim, timers and persisted restore", len(records))
}
