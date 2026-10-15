package server

// Written before draw support. Real HTTP/SQLite checks: draw becomes a win;
// awarded/deducted points; duplicate history/ledger on replay or restart; empty
// winner list becomes null; bot games become rated; history loses draw reason.
// This is a platform fixture, not a substitute for a real game's rule tests.
import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func drawFixture() gameapi.Module {
	return gameapi.Module{ID: "drawcheck", Name: "和棋接入验收", MinPlayers: 2, MaxPlayers: 2,
		Configure: func(n int, _ json.RawMessage) (json.RawMessage, error) {
			if n != 2 {
				return nil, errors.New("two players")
			}
			return json.RawMessage(`{}`), nil
		},
		Start: func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
			return gameapi.Match{Schema: 1, Data: json.RawMessage(`{"offered":false}`), Actors: []int{0}, TimeoutActors: []int{0}, Deadline: c.Now.Add(120 * time.Second).UnixMilli()}, nil
		},
		Apply: func(c gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
			var a struct{ Type string }
			if json.Unmarshal(raw, &a) != nil || len(m.Actors) != 1 || m.Actors[0] != c.Actor {
				return m, errors.New("invalid action")
			}
			if c.Actor == 0 && a.Type == "offer" {
				m.Data = json.RawMessage(`{"offered":true}`)
				m.Actors = []int{1}
				m.TimeoutActors = []int{1}
				m.Deadline = c.Now.Add(120 * time.Second).UnixMilli()
				return m, nil
			}
			if c.Actor != 1 || a.Type != "accept" {
				return m, errors.New("must agree")
			}
			m.Actors, m.TimeoutActors, m.Deadline = nil, nil, 0
			m.Data = json.RawMessage(`{"offered":true,"agreed":true}`)
			var result gameapi.Result
			if e := json.Unmarshal([]byte(`{"draw":true,"ranks":[1,1],"winners":[],"details":{"reason":"agreement"}}`), &result); e != nil {
				return m, e
			}
			m.Result = &result
			m.Log = []string{"双方同意和棋"}
			return m, nil
		},
		View: func(m gameapi.Match, _ int) (any, error) { var v any; e := json.Unmarshal(m.Data, &v); return v, e },
		BotAction: func(c gameapi.Context, _ gameapi.Match) (json.RawMessage, error) {
			if c.Actor == 0 {
				return json.RawMessage(`{"type":"offer"}`), nil
			}
			return json.RawMessage(`{"type":"accept"}`), nil
		},
		Reclaim: func(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
			m.Deadline = c.Now.Add(120 * time.Second).UnixMilli()
			return m, nil
		},
	}
}

func drawEvidence(t *testing.T, data any) {
	t.Helper()
	p := filepath.Join("..", "..", ".local", "chess-family", "draw")
	if e := os.MkdirAll(p, 0755); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "evidence": data}, "", "  ")
	if e := os.WriteFile(filepath.Join(p, t.Name()+".json"), raw, 0600); e != nil {
		t.Fatal(e)
	}
}

func TestModuleDrawHistoryHTTP(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{drawFixture()}}
	s, e := New(cfg, fstest.MapFS{"index.html": {Data: []byte("draw HTTP")}})
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	publishModuleForTest(t, ts.URL, "drawcheck")
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("和棋甲")
	b.register("和棋乙")
	r := a.post("/api/rooms", map[string]any{"kind": "drawcheck", "name": "和棋积分验收", "capacity": 2}, 201)
	b.command(r, "join", nil, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	a.command(current(a), "action", map[string]string{"type": "offer"}, 200)
	r = current(b)
	req := map[string]any{"type": "action", "version": r["version"], "nonce": "draw-replay-accept", "action": map[string]string{"type": "accept"}}
	path := "/api/rooms/" + r["id"].(string)
	b.post(path, req, 200)
	b.post(path, req, 200)
	ended := current(a)
	assertDraw := func(r map[string]any) {
		t.Helper()
		result := r["result"].(map[string]any)
		if r["status"] != "finished" || result["draw"] != true || result["rated"] != true {
			t.Fatal("draw not explicit", r)
		}
		if winners, ok := r["game"].(map[string]any)["winners"].([]any); !ok || len(winners) != 0 {
			t.Fatal("draw has winners or null list", r)
		}
		if result["details"].(map[string]any)["reason"] != "agreement" {
			t.Fatal("draw reason missing")
		}
		for _, p := range result["players"].([]any) {
			if p.(map[string]any)["won"] != false {
				t.Fatal("draw counted as a win")
			}
		}
	}
	assertDraw(ended)
	ids := []string{}
	for _, p := range ended["seats"].([]any) {
		ids = append(ids, p.(map[string]any)["id"].(string))
	}
	for _, id := range ids {
		v, err := s.rating(id)
		if err != nil || v.Points != 1000 || v.Played != 1 || v.Wins != 0 {
			t.Fatal("draw rating incorrect", v, err)
		}
	}
	var entries, delta int
	if e = s.db.QueryRow("SELECT COUNT(*),SUM(delta) FROM rating_ledger").Scan(&entries, &delta); e != nil || entries != 2 || delta != 0 {
		t.Fatal("duplicate/wrong ledger", entries, delta, e)
	}
	ts.Close()
	s.Close()
	s, e = New(cfg, fstest.MapFS{"index.html": {Data: []byte("restored draw")}})
	if e != nil {
		t.Fatal(e)
	}
	ts = httptest.NewServer(s.Handler())
	a.base, b.base = ts.URL, ts.URL
	assertDraw(current(a))
	if !reflect.DeepEqual(ended["result"], current(a)["result"]) {
		t.Fatal("draw changed on restart")
	}
	a.command(current(a), "rematch", nil, 200)
	admin := newClient(t, ts.URL)
	admin.post("/api/login", map[string]string{"name": "ModuleRoot", "password": "module-root-test-123"}, 200)
	admin.post("/api/admin/games/drawcheck", map[string]bool{"enabled": false}, 200)
	code, profile := a.request("GET", "/api/players/"+ids[0], nil)
	if code != 200 {
		t.Fatal(profile)
	}
	history := profile["history"].([]any)
	if len(history) != 1 || history[0].(map[string]any)["draw"] != true {
		t.Fatal("draw missing from history", profile)
	}
	stat := profile["stats"].(map[string]any)["drawcheck"].(map[string]any)
	if stat["played"] != float64(1) || stat["wins"] != float64(0) {
		t.Fatal("draw statistics incorrect", stat)
	}
	drawEvidence(t, map[string]any{"result": ended, "history": profile, "ledgerEntries": entries, "ledgerTotal": delta})
}

// Unpublished new games must not leak into a player's empty game-stat cards.
// Actual historical matches remain visible after later unpublication (above).
func TestModuleHiddenProfileHTTP(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{drawFixture()}}
	s, e := New(cfg, fstest.MapFS{"index.html": {Data: []byte("hidden profiles")}})
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	a := newClient(t, ts.URL)
	a.register("未上架棋类验收")
	id := a.state()["user"].(map[string]any)["id"].(string)
	_, profile := a.request("GET", "/api/players/"+id, nil)
	if _, found := profile["stats"].(map[string]any)["drawcheck"]; found {
		t.Fatal("unpublished game displayed in empty stats")
	}
	publishModuleForTest(t, ts.URL, "drawcheck")
	_, profile = a.request("GET", "/api/players/"+id, nil)
	if _, found := profile["stats"].(map[string]any)["drawcheck"]; !found {
		t.Fatal("published game omitted")
	}
	drawEvidence(t, profile)
}

func TestModuleDrawBotUnratedHTTP(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{drawFixture()}}
	s, e := New(cfg, fstest.MapFS{"index.html": {Data: []byte("draw bot")}})
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	publishModuleForTest(t, ts.URL, "drawcheck")
	a := newClient(t, ts.URL)
	a.register("和棋电脑验收")
	a.post("/api/rooms", map[string]any{"kind": "drawcheck", "name": "电脑和棋", "capacity": 2}, 201)
	for _, cmd := range []string{"add_bot", "ready", "start"} {
		a.command(current(a), cmd, nil, 200)
	}
	a.command(current(a), "action", map[string]string{"type": "offer"}, 200)
	var r map[string]any
	for until := time.Now().Add(4 * time.Second); time.Now().Before(until); {
		r = current(a)
		if r["status"] == "finished" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if r["status"] != "finished" || r["result"].(map[string]any)["draw"] != true || r["result"].(map[string]any)["rated"] != false {
		t.Fatal("bot draw incorrectly rated", r)
	}
	var n int
	if e = s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger").Scan(&n); e != nil || n != 0 {
		t.Fatal("bot draw created rating", n, e)
	}
	drawEvidence(t, r)
}

func TestModuleDrawBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_DRAW_BROWSER") != "1" {
		t.Skip("opt-in browser acceptance")
	}
	root, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local", "chess-family", "draw", "browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	s, e := New(Config{DataDir: t.TempDir(), InviteCode: "draw-browser", AdminUsername: "DrawRoot", AdminPassword: "draw-browser-password", Modules: []gameapi.Module{drawFixture()}}, os.DirFS(filepath.Join(root, "web", "dist")))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	h := http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(l)
	if e = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+l.Addr().String()), 0600); e != nil {
		t.Fatal(e)
	}
	for until := time.Now().Add(5 * time.Minute); time.Now().Before(until); {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
