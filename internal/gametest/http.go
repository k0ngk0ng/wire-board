// Package gametest provides test-only HTTP/session plumbing, never game rules.
// Production games do not import it. Each game's tests supply their own module.
package gametest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

type Client struct {
	t      *testing.T
	HTTP   *http.Client
	Base   string
	Index  int
	serial int
}
type HTTP struct {
	T         *testing.T
	Host      *server.Server
	Server    *httptest.Server
	Config    server.Config
	People    []*Client
	Spectator *Client
	Game      string
	trace     []any
}

func Data(room map[string]any) map[string]any {
	return room["game"].(map[string]any)["data"].(map[string]any)
}
func (c *Client) Request(method, url string, body any, status int) map[string]any {
	c.t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req, e := http.NewRequest(method, c.Base+url, bytes.NewReader(raw))
	if e != nil {
		c.t.Fatal(e)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := c.HTTP.Do(req)
	if e != nil {
		c.t.Fatal(e)
	}
	defer res.Body.Close()
	var data map[string]any
	if e = json.NewDecoder(res.Body).Decode(&data); e != nil {
		c.t.Fatal(e)
	}
	if res.StatusCode != status {
		c.t.Fatalf("%s %s: got %d want %d: %v", method, url, res.StatusCode, status, data)
	}
	return data
}
func (c *Client) State() map[string]any { return c.Request("GET", "/api/state", nil, 200) }
func (c *Client) Room() map[string]any {
	c.t.Helper()
	r, ok := c.State()["room"].(map[string]any)
	if !ok {
		c.t.Fatal("no current room")
	}
	return r
}
func (c *Client) ActionRequest(action map[string]any) map[string]any {
	r := c.Room()
	c.serial++
	return map[string]any{"type": "action", "version": r["version"], "nonce": fmt.Sprintf("poker-%d-%012d", c.Index, c.serial), "action": action}
}
func (c *Client) Action(action map[string]any, status int) map[string]any {
	r := c.Room()
	return c.Request("POST", "/api/rooms/"+r["id"].(string), c.ActionRequest(action), status)
}
func (c *Client) Command(kind string, extra map[string]any, status int) map[string]any {
	r := c.Room()
	c.serial++
	req := map[string]any{"type": kind, "version": r["version"], "nonce": fmt.Sprintf("poker-%d-%012d", c.Index, c.serial)}
	for k, v := range extra {
		req[k] = v
	}
	return c.Request("POST", "/api/rooms/"+r["id"].(string), req, status)
}
func NewHTTP(t *testing.T, n int, module gameapi.Module, kind string) *HTTP {
	t.Helper()
	h := &HTTP{T: t, Game: kind, Config: server.Config{DataDir: t.TempDir(), InviteCode: "poker-http", AdminUsername: "牌局管理员", AdminPassword: "poker-test-password", Modules: []gameapi.Module{module}}}
	h.open()
	for i := 0; i < n+1; i++ {
		jar, _ := cookiejar.New(nil)
		c := &Client{t: t, HTTP: &http.Client{Jar: jar, Timeout: 15 * time.Second}, Base: h.Server.URL, Index: i}
		if i == 0 {
			c.Request("POST", "/api/login", map[string]any{"name": "牌局管理员", "password": "poker-test-password"}, 200)
		} else {
			c.Request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("牌局玩家%d", i), "password": "poker-test-password", "invite": "poker-http"}, 200)
		}
		if i < n {
			h.People = append(h.People, c)
		} else {
			h.Spectator = c
		}
	}
	t.Cleanup(func() {
		h.Server.Close()
		h.Host.Close()
		dir, e := os.Getwd()
		if e != nil {
			t.Error(e)
			return
		}
		for !exists(filepath.Join(dir, "go.mod")) {
			parent := filepath.Dir(dir)
			if parent == dir {
				t.Error("repository not found")
				return
			}
			dir = parent
		}
		dir = filepath.Join(dir, ".local", "poker", kind, "http")
		if e = os.MkdirAll(dir, 0755); e != nil {
			t.Error(e)
			return
		}
		b, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "real HTTP sessions, real game module, real SQLite restore; test identities only", "trace": h.trace}, "", "  ")
		if e = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "-")+".json"), b, 0600); e != nil {
			t.Error(e)
		}
	})
	return h
}
func exists(name string) bool { _, e := os.Stat(name); return e == nil }
func (h *HTTP) open() {
	var e error
	h.Host, e = server.New(h.Config, fstest.MapFS{"index.html": {Data: []byte("Poker HTTP acceptance")}})
	if e != nil {
		h.T.Fatal(e)
	}
	h.Server = httptest.NewServer(h.Host.Handler())
}
func (h *HTTP) Restart() {
	h.Server.Close()
	h.Host.Close()
	h.open()
	for _, c := range append(append([]*Client{}, h.People...), h.Spectator) {
		c.Base = h.Server.URL
	}
	h.Record("restarted persisted room")
}
func (h *HTTP) Record(label string) {
	views := []any{}
	for _, p := range h.People {
		views = append(views, p.Room())
	}
	h.trace = append(h.trace, map[string]any{"label": label, "views": views, "spectator": h.Spectator.State()["room"]})
}
func (h *HTTP) AssertUnpublished() {
	h.T.Helper()
	for _, c := range []*Client{h.People[0], h.Spectator} {
		for _, id := range c.State()["availableGames"].([]any) {
			if id == h.Game {
				h.T.Fatal("new game published without admin action")
			}
		}
		c.Request("POST", "/api/rooms", map[string]any{"kind": h.Game, "name": "默认下架检查", "capacity": len(h.People)}, 403)
	}
	h.Spectator.Request("POST", "/api/admin/games/"+h.Game, map[string]bool{"enabled": true}, 403)
}
func (h *HTTP) Start(options map[string]any) {
	admin := h.People[0]
	admin.Request("POST", "/api/admin/games/"+h.Game, map[string]bool{"enabled": true}, 200)
	created := admin.Request("POST", "/api/rooms", map[string]any{"kind": h.Game, "name": "扑克牌协议验收", "capacity": len(h.People), "options": options}, 201)
	id := created["id"].(string)
	for _, c := range h.People[1:] {
		c.serial++
		c.Request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": admin.Room()["version"], "nonce": fmt.Sprintf("poker-%d-%012d", c.Index, c.serial)}, 200)
	}
	for _, c := range h.People {
		c.Command("ready", nil, 200)
	}
	admin.Command("start", nil, 200)
	h.Spectator.Request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
	h.Record("started real room")
}
