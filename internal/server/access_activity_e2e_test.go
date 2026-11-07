package server

// Failure scenarios: registration IP overwritten by login; forged forwarding
// headers trusted; proxy chain selects attacker input; failed auth logged as
// success; activity/IP exposed to players; missing audit IP; pagination repeats;
// restart loses access history or migration fails on existing metadata tables.
import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/production"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestAccessActivityHTTPPersistence(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{DataDir: dir, InviteCode: "test-invite", AdminUsername: "AccessRoot", AdminPassword: "access-root-password-123", TrustedProxies: "127.0.0.1/32"}
	files := fstest.MapFS{"index.html": {Data: []byte("Wire Board")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	player := newClient(t, ts.URL)
	root := newClient(t, ts.URL)
	post := func(c *testClient, path, body, ip string, want int) {
		t.Helper()
		req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", ip)
		res, e := c.client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("%s: %d", path, res.StatusCode)
		}
	}
	post(player, "/api/register", `{"name":"AccessPlayer","password":"test-password-123","invite":"test-invite"}`, "198.51.100.10", 200)
	id := player.state()["user"].(map[string]any)["id"].(string)
	post(player, "/api/logout", `{}`, "198.51.100.10", 200)
	post(player, "/api/login", `{"name":"AccessPlayer","password":"wrong-password"}`, "198.51.100.90", 401)
	post(player, "/api/login", `{"name":"AccessPlayer","password":"test-password-123"}`, "203.0.113.20", 200)
	post(root, "/api/login", `{"name":"AccessRoot","password":"access-root-password-123"}`, "203.0.113.30", 200)
	u := managed(t, root, id)
	if u["registeredIP"] != "198.51.100.10" || u["lastLoginIP"] != "203.0.113.20" || u["activity"] != "大厅" {
		t.Fatal(u)
	}
	root.post("/api/admin/users/"+id, map[string]any{"action": "role", "role": "admin"}, 200)
	code, events := root.request("GET", "/api/admin/users/"+id+"/activity", nil)
	if code != 200 || len(events["entries"].([]any)) != 3 {
		t.Fatal(code, events)
	}
	for _, v := range events["entries"].([]any) {
		if v.(map[string]any)["ip"] == "" {
			t.Fatal(v)
		}
	}
	player.post("/api/logout", map[string]any{}, 200)
	player.post("/api/login", map[string]any{"name": "AccessPlayer", "password": "test-password-123"}, 200)
	root.post("/api/admin/users/"+id, map[string]any{"action": "role", "role": "player"}, 200)
	if code, _ = player.request("GET", "/api/admin/users/"+id+"/activity", nil); code != 403 {
		t.Fatal("activity privacy", code)
	}
	code, audit := root.request("GET", "/api/admin/audit", nil)
	if code != 200 || audit["entries"].([]any)[0].(map[string]any)["ip"] != "127.0.0.1" {
		t.Fatal(code, audit)
	}
	ts.Close()
	s.Close()
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts = httptest.NewServer(s.Handler())
	defer ts.Close()
	root = newClient(t, ts.URL)
	root.post("/api/login", map[string]any{"name": "AccessRoot", "password": "access-root-password-123"}, 200)
	if managed(t, root, id)["registeredIP"] != "198.51.100.10" {
		t.Fatal("lost registration IP")
	}
	code, events = root.request("GET", "/api/admin/users/"+id+"/activity", nil)
	if code != 200 || len(events["entries"].([]any)) != 5 {
		t.Fatal("lost history", code, events)
	}
	player = newClient(t, ts.URL)
	player.post("/api/login", map[string]any{"name": "AccessPlayer", "password": "test-password-123"}, 200)
	rootID := root.state()["user"].(map[string]any)["id"].(string)
	for i := 0; i < 26; i++ {
		for _, action := range []string{"request", "remove"} {
			player.post("/api/players/"+rootID+"/friend", map[string]any{"action": action}, 200)
		}
	}
	code, events = root.request("GET", "/api/admin/users/"+id+"/activity", nil)
	page := events["entries"].([]any)
	if code != 200 || len(page) != 50 || events["hasMore"] != true {
		t.Fatal("first page", code, events)
	}
	before := int64(page[len(page)-1].(map[string]any)["id"].(float64))
	code, next := root.request("GET", fmt.Sprintf("/api/admin/users/%s/activity?before=%d", id, before), nil)
	rest := next["entries"].([]any)
	if code != 200 || len(rest) != 8 || next["hasMore"] != false {
		t.Fatal("second page", code, next)
	}
	for _, v := range rest {
		if int64(v.(map[string]any)["id"].(float64)) >= before {
			t.Fatal("repeated pagination row", v)
		}
	}
	for _, path := range []string{"?before=0", "?before=nope", "?before=-1"} {
		if code, _ := root.request("GET", "/api/admin/users/"+id+"/activity"+path, nil); code != 400 {
			t.Fatal("invalid cursor accepted", path)
		}
	}
	anon := newClient(t, ts.URL)
	if code, _ := anon.request("GET", "/api/admin/users/"+id+"/activity", nil); code != 401 {
		t.Fatal("anonymous activity access", code)
	}
	_, public := player.request("GET", "/api/players/"+id, nil)
	for _, key := range []string{"registeredIP", "lastLoginIP", "activity"} {
		if _, ok := public[key]; ok {
			t.Fatal("private field in player profile", key)
		}
	}

}

func TestClientIPTrustBoundary(t *testing.T) {
	for _, tc := range []struct{ peer, forwarded, want string }{
		{"192.0.2.8:1234", "1.1.1.1", "192.0.2.8"},
		{"127.0.0.1:1234", "1.1.1.1, 198.51.100.8", "198.51.100.8"},
		{"127.0.0.1:1234", "garbage", "127.0.0.1"},
		{"[::1]:1234", "2001:db8::1", "2001:db8::1"},
	} {
		s := &Server{cfg: Config{TrustedProxies: "127.0.0.1/32,::1/128"}}
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.forwarded)
		if got := s.clientIP(r); got != tc.want {
			t.Fatalf("%s: %s != %s", tc.peer, got, tc.want)
		}
	}
}

// Real platform routes, SQLite and registered modules; no production data.
func TestPlatformAdminBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_PLATFORM_ADMIN_BROWSER") != "1" {
		t.Skip("local browser acceptance only")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	modules, err := production.Modules()
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "page-accept-code", AdminUsername: "DialogAdmin", AdminPassword: "dialog-local-password", Modules: modules}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:18099")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer srv.Close()
	go srv.Serve(listener)
	stop := filepath.Join(root, ".local/platform-admin-acceptance/stop")
	for start := time.Now(); time.Since(start) < 20*time.Minute; {
		if _, err = os.Stat(stop); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser acceptance timed out")
}
