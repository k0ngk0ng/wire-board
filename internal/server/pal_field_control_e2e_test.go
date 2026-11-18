package server

// Authenticated solo autoplay/save regression using the original opening
// checkpoint, production ticker and SQLite; the same human resumes/reclaims.
import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALFieldAutoplaySavedContinuationHTTP(t *testing.T) {
	module := palFieldCheckpointModule(palBoardModule(t))
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{module}}
	files := fstest.MapFS{"index.html": {Data: []byte("original PAL field autoplay HTTP")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer func() { ts.Close(); s.Close() }()
	a := newClient(t, ts.URL)
	a.register("探索托管存档玩家")
	r := a.post("/api/rooms", map[string]any{"kind": "pal", "name": "原版探索托管续玩", "capacity": 1}, 201)
	id := r["id"].(string)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	setAutoPlay(a, current(a), true, 200)
	data := func() map[string]any {
		return current(a)["game"].(map[string]any)["data"].(map[string]any)
	}
	wait := func(predicate func(map[string]any) bool) map[string]any {
		t.Helper()
		for until := time.Now().Add(45 * time.Second); ; {
			d := data()
			if predicate(d) {
				return d
			}
			if time.Now().After(until) {
				t.Fatal("original field autoplay checkpoint unavailable", d["phase"], d["scene"], d["x"], d["y"], d["cash"])
			}
			time.Sleep(40 * time.Millisecond)
		}
	}
	wait(func(d map[string]any) bool { return d["scene"] == float64(2) && d["x"].(float64) < 1344 })
	a.command(current(a), "save", nil, 200)
	if a.state()["room"] != nil {
		t.Fatal("field autoplay save did not release the original human seat")
	}
	ts.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Modules = []gameapi.Module{palBoardModule(t)}
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	a.base = ts.URL
	a.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	d := wait(func(d map[string]any) bool { return d["cash"] == float64(500) && d["phase"] == "field" })
	setAutoPlay(a, current(a), false, 200)
	after := data()
	if after["cash"] != d["cash"] || len(after["party"].([]any)) != 1 || current(a)["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
		t.Fatal("manual reclaim changed original reward/party or left autoplay enabled")
	}
	streamEvidence(t, "pal-field-autoplay-save", map[string]any{"passed": true, "scope": "authenticated HTTP, production ticker, explicit single-human autoplay, SQLite save and fresh-runtime server restart, original guest reward and manual reclaim; not full campaign bots", "cash": after["cash"], "partyCount": len(after["party"].([]any)), "savedSeatReleased": true})
}
