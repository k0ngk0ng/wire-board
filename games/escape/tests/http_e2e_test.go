package escape_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/nettest"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

type httpPlayer struct {
	t      *testing.T
	client *http.Client
	base   string
	serial int
}

func (c *httpPlayer) request(method, path string, body any, status int) map[string]any {
	c.t.Helper()
	raw, _ := json.Marshal(body)
	req, e := http.NewRequest(method, c.base+path, bytes.NewReader(raw))
	if e != nil {
		c.t.Fatal(e)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := c.client.Do(req)
	if e != nil {
		c.t.Fatal(e)
	}
	defer res.Body.Close()
	var v map[string]any
	if e = json.NewDecoder(res.Body).Decode(&v); e != nil {
		c.t.Fatal(e)
	}
	if res.StatusCode != status {
		c.t.Fatalf("%s %s got%d want%d: %v", method, path, res.StatusCode, status, v)
	}
	return v
}
func (c *httpPlayer) room() map[string]any {
	c.t.Helper()
	return obj(c.request("GET", "/api/state", nil, 200)["room"])
}
func (c *httpPlayer) command(kind string, extra map[string]any, status int) map[string]any {
	c.t.Helper()
	r := c.room()
	c.serial++
	q := map[string]any{"type": kind, "version": r["version"], "nonce": fmt.Sprintf("escape-%024d", c.serial)}
	for k, v := range extra {
		q[k] = v
	}
	return c.request("POST", "/api/rooms/"+text(r, "id"), q, status)
}
func (c *httpPlayer) action(a map[string]any, status int) {
	c.t.Helper()
	r := c.room()
	d := obj(obj(r["game"])["data"])
	who := int(number(r, "you"))
	a["epoch"] = d["epoch"]
	a["seq"] = number(obj(array(d, "players")[who]), "seq") + 1
	c.command("action", map[string]any{"action": a}, status)
}

func TestEscapeHTTPPersistenceSpectatorAndAutoPlay(t *testing.T) {
	cfg := server.Config{DataDir: t.TempDir(), InviteCode: "escape-e2e", AdminUsername: "密室验收", AdminPassword: "escape-only-password", Modules: []gameapi.Module{module(t)}}
	app, e := server.New(cfg, fstest.MapFS{"index.html": {Data: []byte("escape HTTP E2E")}})
	if e != nil {
		t.Fatal(e)
	}
	srv := httptest.NewServer(app.Handler())
	people := []*httpPlayer{}
	for i := 0; i < 3; i++ {
		jar, _ := cookiejar.New(nil)
		c := &httpPlayer{t: t, client: &http.Client{Jar: jar, Timeout: 10 * time.Second}, base: srv.URL}
		if i == 0 {
			c.request("POST", "/api/login", map[string]any{"name": "密室验收", "password": "escape-only-password"}, 200)
		} else {
			c.request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("探索者%d", i), "password": "escape-only-password", "invite": "escape-e2e"}, 200)
		}
		people = append(people, c)
	}
	defer func() { srv.Close(); app.Close() }()
	a, b, watch := people[0], people[1], people[2]
	found := false
	for _, id := range array(a.request("GET", "/api/state", nil, 200), "availableGames") {
		found = found || id == "escape"
	}
	if !found {
		t.Fatal("new game not available by default")
	}
	created := a.request("POST", "/api/rooms", map[string]any{"kind": "escape", "name": "双人封锁站", "capacity": 2, "options": map[string]any{"scenario": "station", "difficulty": "normal", "seed": "http-e2e"}}, 201)
	b.request("POST", "/api/rooms/"+text(created, "id"), map[string]any{"type": "join", "version": a.room()["version"], "nonce": "escape-join-00000000001"}, 200)
	a.command("ready", nil, 200)
	b.command("ready", nil, 200)
	a.command("start", nil, 200)
	for _, c := range []*httpPlayer{a, b} {
		r := c.room()
		if loading := obj(r["loading"]); len(loading) > 0 {
			c.command("load_ready", map[string]any{"loadToken": loading["token"], "loadBuild": loading["build"], "loadClient": "escape-http"}, 200)
		}
	}
	watch.request("POST", "/api/rooms/"+text(created, "id")+"/watch", map[string]any{}, 200)
	initialX := number(obj(array(obj(obj(a.room()["game"])["data"]), "players")[0]), "x")
	a.action(map[string]any{"type": "input", "dx": 1, "dy": 0}, 200)
	time.Sleep(300 * time.Millisecond)
	a.action(map[string]any{"type": "input", "dx": 0, "dy": 0}, 200)
	before := obj(obj(a.room()["game"])["data"])
	if number(obj(array(before, "players")[0]), "x") <= initialX {
		t.Fatal("native action did not move authenticated seat")
	}
	// Active observer can read discovered rooms but cannot submit a game action.
	wr := watch.request("GET", "/api/state", nil, 200)
	wroom := obj(wr["room"])
	if len(wroom) == 0 {
		wroom = obj(wr["watching"])
	}
	watch.request("POST", "/api/rooms/"+text(created, "id"), map[string]any{"type": "action", "version": a.room()["version"], "nonce": "escape-watch-000000000001", "action": map[string]any{"type": "input", "epoch": before["epoch"], "seq": 1, "dx": 1}}, 400)
	a.action(map[string]any{"type": "pause"}, 200)
	b.action(map[string]any{"type": "pause"}, 200)
	paused := obj(obj(a.room()["game"])["data"])
	srv.Close()
	app.Close()
	app, e = server.New(cfg, fstest.MapFS{"index.html": {Data: []byte("restart")}})
	if e != nil {
		t.Fatal(e)
	}
	srv = httptest.NewServer(app.Handler())
	for _, c := range people {
		c.base = srv.URL
	}
	restored := obj(obj(a.room()["game"])["data"])
	if text(restored, "epoch") != text(paused, "epoch") || number(restored, "remaining") != number(paused, "remaining") || number(restored, "power") != number(paused, "power") {
		t.Fatal("SQLite restart changed paused campaign")
	}
	a.action(map[string]any{"type": "resume"}, 200)
	b.action(map[string]any{"type": "resume"}, 200)
	a.command("autoplay", map[string]any{"enabled": true}, 200)
	time.Sleep(200 * time.Millisecond)
	a.command("autoplay", map[string]any{"enabled": false}, 200)
	if boolv(obj(array(a.room(), "seats")[0]), "autoPlay") {
		t.Fatal("cannot regain manual control")
	}
	evidence(t, "http-persistence", map[string]any{"passed": true, "before": before, "paused": paused, "restored": restored, "observer": wroom, "scope": "real HTTP, authentication, ordinary movement, spectator rejection, pause and SQLite restart"})
}

func TestEscapeBrowserServer(t *testing.T) {
	if os.Getenv("ESCAPE_BROWSER_SERVER") != "1" {
		t.Skip("explicit browser acceptance server")
	}
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/escape/browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	app, e := server.New(server.Config{DataDir: filepath.Join(out, "data"), InviteCode: "escape-browser", AdminUsername: "密室体验", AdminPassword: "escape-browser-password", Modules: []gameapi.Module{module(t)}}, os.DirFS(filepath.Join(root, "web/dist")))
	if e != nil {
		t.Fatal(e)
	}
	defer app.Close()
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	type connectionKey struct{}
	var connectionsMu sync.Mutex
	connections := map[net.Conn]bool{}
	drop := func(onlySlow bool) int {
		connectionsMu.Lock()
		list := []net.Conn{}
		for c, slow := range connections {
			if !onlySlow || slow {
				list = append(list, c)
				delete(connections, c)
			}
		}
		connectionsMu.Unlock()
		for _, c := range list {
			_ = c.Close()
		}
		return len(list)
	}
	defer drop(false)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms, _ := strconv.Atoi(r.Header.Get("X-Escape-Test-RTT"))
		if r.Method == "POST" && r.URL.Path == "/_test/disconnect" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int{"closed": drop(true)})
			return
		}
		if r.Header.Get("Upgrade") == "websocket" {
			if c, ok := r.Context().Value(connectionKey{}).(net.Conn); ok {
				connectionsMu.Lock()
				connections[c] = ms > 0
				connectionsMu.Unlock()
			}
		}
		if ms > 0 && ms <= 1000 {
			nettest.Delay(app.Handler(), time.Duration(ms)*time.Millisecond).ServeHTTP(w, r)
			return
		}
		app.Handler().ServeHTTP(w, r)
	})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ConnContext: func(ctx context.Context, c net.Conn) context.Context {
		return context.WithValue(ctx, connectionKey{}, c)
	}}
	defer srv.Close()
	go srv.Serve(listener)
	if e = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); e != nil {
		t.Fatal(e)
	}
	until := time.Now().Add(90 * time.Minute)
	for time.Now().Before(until) {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("browser server acceptance deadline")
}
