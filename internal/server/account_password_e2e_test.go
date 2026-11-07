package server

// Failure inventory, before implementation: unauthenticated/disabled accounts
// change a password; wrong current password logs the player out; another user ID
// is accepted; short/oversize UTF-8 values or trailing JSON bypass validation;
// plaintext leaks into logs; old HTTP/WebSocket sessions survive; the current
// browser loses its game or gains autoplay; concurrent requests both commit;
// a failed hash/session transaction partially revokes credentials; restart
// forgets the change; configured bootstrap credentials silently change back.
// These tests use real authenticated HTTP, WebSockets and SQLite. No handler
// mocks. Persisted result receipts contain no submitted passwords or tokens.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"golang.org/x/crypto/bcrypt"
)

const passwordPath = "/api/account/password"

func passwordReceipt(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		out := filepath.Join("..", "..", ".local", "account-password", "http")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "real HTTP sessions, WebSockets and SQLite; no password/token payload retained", "command": ". scripts/env.sh && go test ./internal/server -run '^" + t.Name() + "$' -count=1 -v"}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
}

func passwordBody(old, next string) map[string]string {
	return map[string]string{"currentPassword": old, "newPassword": next}
}

func TestAccountPasswordSessionRotationHTTP(t *testing.T) {
	passwordReceipt(t)
	s, ts, _ := adminServer(t)
	a, b, other, guest := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("PasswordPlayer")
	b.register("PasswordPeer")
	other.post("/api/login", map[string]string{"name": "PasswordPlayer", "password": "test-password-123"}, 200)
	guest.post(passwordPath, passwordBody("test-password-123", "changed-password-123"), 401)
	if a.state()["user"].(map[string]any)["passwordChangeAllowed"] != true {
		t.Fatal("ordinary account cannot open password settings")
	}
	created := a.post("/api/rooms", map[string]any{"kind": "splendor", "capacity": 2, "name": "password-game-preservation"}, 201)
	b.command(created, "join", nil, 200)
	for _, c := range []*testClient{a, b} {
		c.command(current(c), "ready", nil, 200)
	}
	a.command(current(a), "start", nil, 200)
	before := current(a)
	id := a.state()["user"].(map[string]any)["id"].(string)
	u, _ := url.Parse(ts.URL)
	oldCookie := a.client.Jar.Cookies(u)[0].String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPClient: other.client})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if _, _, err = conn.Read(ctx); err != nil {
		t.Fatal(err)
	}
	a.post(passwordPath, passwordBody("wrong-password", "changed-password-123"), 403)
	a.state()
	other.state()
	a.post(passwordPath, passwordBody("test-password-123", "changed-password-123"), 200)
	after := current(a)
	if before["id"] != after["id"] || before["version"] != after["version"] {
		t.Fatal("password change reset the current table")
	}
	for _, raw := range after["seats"].([]any) {
		if raw.(map[string]any)["autoPlay"] == true {
			t.Fatal("self password change enabled autoplay")
		}
	}
	if code, _ := other.request("GET", "/api/state", nil); code != 401 {
		t.Fatal("other session still authenticated")
	}
	if _, _, err = conn.Read(ctx); err == nil {
		t.Fatal("old websocket survived password rotation")
	}
	request, _ := http.NewRequest("GET", ts.URL+"/api/state", nil)
	request.Header.Set("Cookie", oldCookie)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("original cookie was not rotated")
	}
	b.state()
	var hash string
	s.db.QueryRow("SELECT password FROM users WHERE id=?", id).Scan(&hash)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("changed-password-123")) != nil {
		t.Fatal("new password not hashed correctly")
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM user_activity WHERE user_id=? AND action='password_change' AND target=''", id).Scan(&count)
	if count != 1 {
		t.Fatal("password change not recorded exactly once")
	}
	var leaked int
	s.db.QueryRow("SELECT count(*) FROM user_activity WHERE target LIKE '%changed-password%' OR target LIKE '%test-password%' ").Scan(&leaked)
	if leaked != 0 {
		t.Fatal("password appeared in activity")
	}
	other.post("/api/login", map[string]string{"name": "PasswordPlayer", "password": "test-password-123"}, 401)
	other.post("/api/login", map[string]string{"name": "PasswordPlayer", "password": "changed-password-123"}, 200)
}

func TestAccountPasswordValidationHTTP(t *testing.T) {
	passwordReceipt(t)
	_, ts, _ := adminServer(t)
	a := newClient(t, ts.URL)
	a.register("PasswordBounds")
	for _, next := range []string{"", "short", strings.Repeat("x", 73), strings.Repeat("密", 25), "test-password-123"} {
		a.post(passwordPath, passwordBody("test-password-123", next), 400)
	}
	for _, body := range []string{`{"currentPassword":"test-password-123","newPassword":"next-password-123","userId":"other"}`, `{"currentPassword":"test-password-123","newPassword":"next-password-123"} {}`, strings.Repeat("x", 5000)} {
		req, _ := http.NewRequest("POST", ts.URL+passwordPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := a.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 400 {
			t.Fatal("invalid or oversized JSON accepted", res.StatusCode)
		}
	}
	req, _ := http.NewRequest("POST", ts.URL+passwordPath, strings.NewReader(`{"currentPassword":"test-password-123","newPassword":"next-password-123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://untrusted.invalid")
	res, err := a.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("untrusted origin accepted")
	}
	a.state()
	// 24 three-byte characters are valid at bcrypt's exact 72-byte boundary.
	a.post(passwordPath, passwordBody("test-password-123", strings.Repeat("密", 24)), 200)
	a.state()
}

func TestAccountPasswordRateLimitHTTP(t *testing.T) {
	passwordReceipt(t)
	_, ts, _ := adminServer(t)
	a := newClient(t, ts.URL)
	a.register("PasswordRate")
	for i := 0; i < 5; i++ {
		a.post(passwordPath, passwordBody("wrong-password", "next-password-123"), 403)
	}
	a.post(passwordPath, passwordBody("wrong-password", "next-password-123"), 429)
	a.state()
}

func TestAccountPasswordTransactionRollbackHTTP(t *testing.T) {
	for _, table := range []string{"users", "sessions", "user_activity"} {
		t.Run(table, func(t *testing.T) {
			passwordReceipt(t)
			s, ts, _ := adminServer(t)
			a, other := newClient(t, ts.URL), newClient(t, ts.URL)
			a.register("PasswordRollback")
			other.post("/api/login", map[string]string{"name": "PasswordRollback", "password": "test-password-123"}, 200)
			id := a.state()["user"].(map[string]any)["id"].(string)
			trigger := "CREATE TRIGGER reject_password_fixture BEFORE INSERT ON " + table + " WHEN NEW.user_id='" + id + "' BEGIN SELECT RAISE(ABORT,'fixture'); END;"
			if table == "users" {
				trigger = "CREATE TRIGGER reject_password_fixture BEFORE UPDATE OF password ON users WHEN NEW.id='" + id + "' BEGIN SELECT RAISE(ABORT,'fixture'); END;"
			}
			if _, err := s.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			a.post(passwordPath, passwordBody("test-password-123", "changed-password-123"), 500)
			a.state()
			other.state()
			var hash string
			s.db.QueryRow("SELECT password FROM users WHERE id=?", id).Scan(&hash)
			if bcrypt.CompareHashAndPassword([]byte(hash), []byte("test-password-123")) != nil {
				t.Fatal("failed transaction changed password")
			}
		})
	}
}

func TestAccountPasswordConcurrentHTTP(t *testing.T) {
	passwordReceipt(t)
	s, ts, _ := adminServer(t)
	a := newClient(t, ts.URL)
	a.register("PasswordConcurrent")
	u, _ := url.Parse(ts.URL)
	cookie := a.client.Jar.Cookies(u)[0].String()
	id := a.state()["user"].(map[string]any)["id"].(string)
	start := make(chan struct{})
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			raw, _ := json.Marshal(passwordBody("test-password-123", fmt.Sprintf("concurrent-password-%d", i)))
			req, _ := http.NewRequest("POST", ts.URL+passwordPath, bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Cookie", cookie)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				codes <- 0
				return
			}
			res.Body.Close()
			codes <- res.StatusCode
		}(i)
	}
	close(start)
	wg.Wait()
	close(codes)
	success := 0
	for code := range codes {
		if code == 200 {
			success++
		} else if code != 401 && code != 409 {
			t.Fatal("unexpected concurrent result", code)
		}
	}
	if success != 1 {
		t.Fatal("concurrent credential changes did not commit once", success)
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM sessions WHERE user_id=?", id).Scan(&count)
	if count != 1 {
		t.Fatal("rotation did not leave exactly one current session", count)
	}
}

func TestAccountPasswordManagedRootHTTP(t *testing.T) {
	passwordReceipt(t)
	_, _, root := adminServer(t)
	if root.state()["user"].(map[string]any)["passwordChangeAllowed"] != false {
		t.Fatal("configured root password incorrectly editable")
	}
	root.post(passwordPath, passwordBody("root-test-password-123", "new-root-password-123"), 403)
	root.state()
}

func TestAccountPasswordManagedRootWhitespaceHTTP(t *testing.T) {
	passwordReceipt(t)
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "  RootKeeper  ", AdminPassword: "root-test-password-123"}, fstest.MapFS{"index.html": {Data: []byte("normalized administrator")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	root := newClient(t, ts.URL)
	root.post("/api/login", map[string]string{"name": "RootKeeper", "password": "root-test-password-123"}, 200)
	if root.state()["user"].(map[string]any)["passwordChangeAllowed"] != false {
		t.Fatal("configured root policy ignored username normalization")
	}
	root.post(passwordPath, passwordBody("root-test-password-123", "new-root-password-123"), 403)
}

func TestAccountPasswordExistingRootRestartHTTP(t *testing.T) {
	passwordReceipt(t)
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite"}
	files := fstest.MapFS{"index.html": {Data: []byte("password persistence")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	a := newClient(t, ts.URL)
	a.register("ExistingPasswordRoot")
	ts.Close()
	s.Close()
	cfg.AdminExistingUsername = "ExistingPasswordRoot"
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	a.base = ts.URL
	a.post("/api/login", map[string]string{"name": "ExistingPasswordRoot", "password": "test-password-123"}, 200)
	a.post(passwordPath, passwordBody("test-password-123", "persistent-password-123"), 200)
	ts.Close()
	s.Close()
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts = httptest.NewServer(s.Handler())
	defer ts.Close()
	a.base = ts.URL
	a.state()
	a.post("/api/login", map[string]string{"name": "ExistingPasswordRoot", "password": "test-password-123"}, 401)
	a.post("/api/login", map[string]string{"name": "ExistingPasswordRoot", "password": "persistent-password-123"}, 200)
}
