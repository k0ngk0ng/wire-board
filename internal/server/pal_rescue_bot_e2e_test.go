package server

// Actual original solo joining checkpoint + authenticated HTTP/SQLite/ticker.
// Verify ordinary magic, personal save/restart without Start replay, original
// account privacy, unchanged spell resources and both roles owned by the same
// human, stale input rejection, explicit manual reclaim and original victory.
import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALOriginalSoloRescueMagicSaveHTTP(t *testing.T) {
	checkpoint, err := os.ReadFile("../../.local/pal/verification/first-party-joining-state-1.json")
	if err != nil {
		t.Fatal("run original solo campaign first", err)
	}
	var starts atomic.Int32
	module := func() gameapi.Module {
		m := palBoardModule(t)
		resume := m.ResumeSaved
		m.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
			starts.Add(1)
			return resume(c, gameapi.Match{Schema: 1, Data: append(json.RawMessage(nil), checkpoint...)})
		}
		return m
	}
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{module()}}
	files := fstest.MapFS{"index.html": {Data: []byte("original solo rescue magic save")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer func() { ts.Close(); s.Close() }()
	a, stranger := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("救援个人存档")
	stranger.register("救援陌生账号")
	r := a.post("/api/rooms", map[string]any{"kind": "pal", "name": "原版救援个人存档", "capacity": 1}, 201)
	id := r["id"].(string)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	data := func() map[string]any { return current(a)["game"].(map[string]any)["data"].(map[string]any) }
	mana := func(d map[string]any) []any {
		var result []any
		for _, p := range d["party"].([]any) {
			m := p.(map[string]any)
			if m["controller"] != float64(0) {
				t.Fatal("solo rescue lost full-party ownership")
			}
			result = append(result, []any{m["role"], m["mp"], m["controller"]})
		}
		return result
	}
	initial := data()
	cash := initial["cash"].(float64)
	beforeMana := mana(initial)
	setAutoPlay(a, current(a), true, 200)
	wait := func(predicate func(map[string]any) bool) map[string]any {
		t.Helper()
		until := time.Now().Add(120 * time.Second)
		for time.Now().Before(until) {
			d := data()
			if d["cash"].(float64) < cash {
				t.Fatal("solo rescue restarted after defeat")
			}
			if predicate(d) {
				return d
			}
			time.Sleep(40 * time.Millisecond)
		}
		d := data()
		t.Fatal("solo rescue checkpoint unavailable", d["phase"], d["cash"], d["party"])
		return nil
	}
	paused := wait(func(d map[string]any) bool { return d["battle"] != nil && !reflect.DeepEqual(mana(d), beforeMana) })
	pausedMana := mana(paused)
	oldPage := current(a)
	a.command(current(a), "save", nil, 200)
	if a.state()["room"] != nil || len(stranger.state()["saves"].([]any)) != 0 {
		t.Fatal("personal magic save retained the seat or leaked to another account")
	}
	stranger.post("/api/saves/"+id+"/resume", map[string]any{}, 404)
	ts.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Modules = []gameapi.Module{module()}
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	a.base, stranger.base = ts.URL, ts.URL
	a.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	resumed := data()
	if current(a)["status"] != "playing" || !reflect.DeepEqual(mana(resumed), pausedMana) || resumed["cash"] != paused["cash"] || resumed["epoch"] == paused["epoch"] || starts.Load() != 1 {
		t.Fatal("personal magic restore replayed/reset the original spell", starts.Load())
	}
	if current(a)["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
		t.Fatal("solo restore lost explicit autoplay")
	}
	setAutoPlay(a, current(a), false, 200)
	a.command(oldPage, "action", map[string]any{"type": "input", "epoch": paused["epoch"], "seq": 999, "direction": 4, "keys": 32768, "controlAt": paused["controlAt"]}, 409)
	setAutoPlay(a, current(a), true, 200)
	finished := wait(func(d map[string]any) bool { return d["phase"] == "field" && d["cash"] == cash+96 })
	if len(finished["party"].([]any)) != 2 {
		t.Fatal("solo rescue lost an original role")
	}
	_ = mana(finished)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "players": 1, "checkpointStarts": starts.Load(), "cashBefore": cash, "cashAfter": finished["cash"], "scope": "actual original solo joining checkpoint, authenticated personal autoplay/magic save, SQLite restart, original-account immediate continuation, both role owners, manual reclaim and original rescue victory; checkpoint entry only, not whole autoplay"}, "", "  ")
	if err := os.WriteFile("../../.local/pal/verification/solo-rescue-magic-save-http.json", report, 0600); err != nil {
		t.Fatal(err)
	}
}
