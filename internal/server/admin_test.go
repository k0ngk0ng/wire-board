package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
)

func adminServer(t *testing.T) (*Server, *httptest.Server, *testClient) {
	t.Helper()
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "RootKeeper", AdminPassword: "root-test-password-123"}, fstest.MapFS{"index.html": {Data: []byte("Wire Board")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	c := newClient(t, ts.URL)
	c.post("/api/login", map[string]string{"name": "RootKeeper", "password": "root-test-password-123"}, 200)
	return s, ts, c
}
func overview(t *testing.T, c *testClient) map[string]any {
	t.Helper()
	code, data := c.request("GET", "/api/admin", nil)
	if code != 200 {
		t.Fatal(code, data)
	}
	return data
}
func managed(t *testing.T, c *testClient, id string) map[string]any {
	t.Helper()
	for _, v := range overview(t, c)["users"].([]any) {
		u := v.(map[string]any)
		if u["id"] == id {
			return u
		}
	}
	t.Fatal("missing user", id)
	return nil
}

func TestAdminExistingAccountPermanentAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{DataDir: dir, InviteCode: "test-invite"}
	files := fstest.MapFS{"index.html": {Data: []byte("Wire Board")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	c := newClient(t, ts.URL)
	c.register("ExistingOwner")
	id := c.state()["user"].(map[string]any)["id"].(string)
	var oldHash string
	if err = s.db.QueryRow("SELECT password FROM users WHERE id=?", id).Scan(&oldHash); err != nil {
		t.Fatal(err)
	}
	ts.Close()
	s.Close()
	cfg.AdminExistingUsername = "ExistingOwner"
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	var hash string
	s.db.QueryRow("SELECT count(*) FROM users").Scan(&count)
	s.db.QueryRow("SELECT password FROM users WHERE id=?", id).Scan(&hash)
	if count != 1 || hash != oldHash || s.role(id) != "superadmin" {
		t.Fatal("promotion changed account/password", count)
	}
	s.Close()
	cfg.AdminExistingUsername = ""
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts = httptest.NewServer(s.Handler())
	defer ts.Close()
	c = newClient(t, ts.URL)
	c.post("/api/login", map[string]string{"name": "ExistingOwner", "password": "test-password-123"}, 200)
	if c.state()["user"].(map[string]any)["role"] != "superadmin" {
		t.Fatal("permanent role was lost")
	}
	overview(t, c)
	for _, action := range []string{"disable", "enable", "logout", "password", "role"} {
		c.post("/api/admin/users/"+id, map[string]any{"action": action, "role": "player", "password": "new-test-password"}, 403)
	}
}

func TestAdminBootstrapDoesNotClaimExistingName(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{DataDir: dir, InviteCode: "test-invite"}
	files := fstest.MapFS{}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	c := newClient(t, ts.URL)
	c.register("TakenName")
	ts.Close()
	s.Close()
	cfg.AdminUsername = "TakenName"
	cfg.AdminPassword = "new-root-password-123"
	if s, err = New(cfg, files); err == nil {
		s.Close()
		t.Fatal("silently claimed existing account")
	}
	cfg.AdminUsername = ""
	cfg.AdminPassword = ""
	cfg.AdminExistingUsername = "MissingOwner"
	if s, err = New(cfg, files); err == nil {
		s.Close()
		t.Fatal("accepted nonexistent account")
	}
	cfg.AdminExistingUsername = "TakenName"
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	cfg.AdminExistingUsername = "DifferentName"
	if s, err = New(cfg, files); err == nil {
		s.Close()
		t.Fatal("reassigned permanent account")
	}
}

func TestAdminAuthorizationAndModeration(t *testing.T) {
	s, ts, root := adminServer(t)
	anon := newClient(t, ts.URL)
	player := newClient(t, ts.URL)
	player.register("PlayerOne")
	id := player.state()["user"].(map[string]any)["id"].(string)
	for _, c := range []*testClient{anon, player} {
		want := 403
		if c == anon {
			want = 401
		}
		for _, path := range []string{"/api/admin", "/api/admin/audit"} {
			code, _ := c.request("GET", path, nil)
			if code != want {
				t.Fatal("admin data exposed", path, code)
			}
		}
		c.post("/api/admin/games/dota", map[string]bool{"enabled": false}, want)
		c.post("/api/admin/users/"+id, map[string]string{"action": "role", "role": "superadmin"}, want)
		c.post("/api/admin/rooms/missing/close", map[string]any{}, want)
	}
	rootID := root.state()["user"].(map[string]any)["id"].(string)
	root.post("/api/admin/users/"+id, map[string]string{"action": "role", "role": "admin"}, 200)
	if player.state()["user"].(map[string]any)["role"] != "admin" {
		t.Fatal("role not refreshed")
	}
	player.post("/api/admin/users/"+rootID, map[string]string{"action": "disable"}, 403)
	other := newClient(t, ts.URL)
	other.register("PlayerTwo")
	otherID := other.state()["user"].(map[string]any)["id"].(string)
	player.post("/api/admin/users/"+otherID, map[string]string{"action": "role", "role": "admin"}, 403)
	player.post("/api/admin/users/"+otherID, map[string]string{"action": "disable"}, 200)
	if code, _ := other.request("GET", "/api/state", nil); code != 401 {
		t.Fatal("disabled session still works")
	}
	other.post("/api/login", map[string]string{"name": "PlayerTwo", "password": "test-password-123"}, 401)
	player.post("/api/admin/users/"+otherID, map[string]string{"action": "enable"}, 200)
	other.post("/api/login", map[string]string{"name": "PlayerTwo", "password": "test-password-123"}, 200)
	player.post("/api/admin/users/"+otherID, map[string]string{"action": "password", "password": "short"}, 400)
	newPassword := "changed-password-1234"
	player.post("/api/admin/users/"+otherID, map[string]string{"action": "password", "password": newPassword}, 200)
	if code, _ := other.request("GET", "/api/state", nil); code != 401 {
		t.Fatal("reset did not revoke sessions")
	}
	other.post("/api/login", map[string]string{"name": "PlayerTwo", "password": "test-password-123"}, 401)
	other.post("/api/login", map[string]string{"name": "PlayerTwo", "password": newPassword}, 200)
	player.post("/api/admin/users/"+otherID, map[string]string{"action": "logout"}, 200)
	if code, _ := other.request("GET", "/api/state", nil); code != 401 {
		t.Fatal("forced logout ineffective")
	}
	root.post("/api/admin/users/"+id, map[string]string{"action": "role", "role": "player"}, 200)
	if code, _ := player.request("GET", "/api/admin", nil); code != 403 {
		t.Fatal("demoted session still admin")
	}
	code, logs := root.request("GET", "/api/admin/audit", nil)
	raw, _ := json.Marshal(logs)
	if code != 200 || len(logs["entries"].([]any)) < 7 || strings.Contains(string(raw), newPassword) {
		t.Fatal("audit incomplete or leaked secret", logs)
	}
	var persisted string
	s.db.QueryRow("SELECT password FROM users WHERE id=?", otherID).Scan(&persisted)
	if persisted == newPassword {
		t.Fatal("stored plaintext")
	}
	req, _ := http.NewRequest("POST", ts.URL+"/api/admin/games/dota", strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Origin", "https://evil.example")
	res, err := root.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("admin CSRF accepted")
	}
}

func TestAdminGameVisibilityExistingPlayAndClosure(t *testing.T) {
	s, ts, root := adminServer(t)
	a, b, c := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alpha")
	b.register("Bravo")
	c.register("Charlie")
	playing := startRoom(t, a, b, "splendor")
	waiting := c.post("/api/rooms", map[string]any{"name": "Waiting", "kind": "splendor", "capacity": 2}, 201)
	root.post("/api/admin/games/splendor", map[string]bool{"enabled": false}, 200)
	state := a.state()
	if len(state["rooms"].([]any)) != 0 || current(a)["status"] != "playing" || current(c)["status"] != "closed" {
		t.Fatal("downlisting behavior", state)
	}
	for _, v := range state["availableGames"].([]any) {
		if v == "splendor" {
			t.Fatal("hidden game exposed")
		}
	}
	root.post("/api/rooms", map[string]any{"name": "Blocked", "kind": "splendor", "capacity": 2}, 403)
	root.command(playing, "join", nil, 403)
	root.post("/api/rooms/"+playing["id"].(string)+"/watch", map[string]any{}, 403)
	c.command(current(c), "rematch", nil, 403)
	// Existing players may still operate normally, including handing over to bots.
	a.post("/api/rooms/"+playing["id"].(string), map[string]any{"type": "autoplay", "version": current(a)["version"], "nonce": randomID(12), "enabled": true}, 200)
	root.post("/api/admin/rooms/"+playing["id"].(string)+"/close", map[string]any{}, 200)
	if current(a)["status"] != "closed" {
		t.Fatal("admin closure ineffective")
	}
	var points int
	s.db.QueryRow("SELECT count(*) FROM rating_ledger").Scan(&points)
	if points != 0 {
		t.Fatal("aborted game rated")
	}
	root.post("/api/admin/games/splendor", map[string]bool{"enabled": true}, 200)
	c.command(current(c), "rematch", nil, 200)
	if current(c)["status"] != "waiting" || current(c)["id"] != waiting["id"] {
		t.Fatal("republishing failed")
	}
	// Every supported game is controlled server-side, not only the frontend cards.
	for _, kind := range gameKinds {
		root.post("/api/admin/games/"+kind, map[string]bool{"enabled": false}, 200)
		root.post("/api/rooms", map[string]any{"name": "Blocked", "kind": kind, "capacity": 4}, 403)
	}
	if len(root.state()["availableGames"].([]any)) != 0 {
		t.Fatal("all games hidden should be empty")
	}
	root.post("/api/admin/games/unknown", map[string]bool{"enabled": false}, 404)
	root.post("/api/admin/games/dota", map[string]any{}, 400)
}

func TestAdminRevocationTakesOverGameAndClosesSockets(t *testing.T) {
	s, ts, root := adminServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alpha")
	b.register("Bravo")
	room := startRoom(t, a, b, "splendor")
	id := a.state()["user"].(map[string]any)["id"].(string)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPClient: a.client})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if _, _, err = conn.Read(ctx); err != nil {
		t.Fatal(err)
	}
	root.post("/api/admin/users/"+id, map[string]string{"action": "disable"}, 200)
	if _, _, err = conn.Read(ctx); err == nil {
		t.Fatal("revoked socket remains open")
	}
	s.mu.Lock()
	p := s.rooms[room["id"].(string)].Seats[0]
	s.mu.Unlock()
	if !p.AutoPlay || p.Left || p.ID != id {
		t.Fatal("seat was not safely handed to computer", p)
	}
	if u := managed(t, root, id); u["online"] != false || u["disabled"] != true || u["activity"] != "电脑托管" {
		t.Fatal("bad moderated presence", u)
	}
}

func TestAdminPresenceCountsUsersNotTabs(t *testing.T) {
	s, ts, root := adminServer(t)
	a := newClient(t, ts.URL)
	a.register("ConnectedPlayer")
	id := a.state()["user"].(map[string]any)["id"].(string)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connections := []*websocket.Conn{}
	for i := 0; i < 2; i++ {
		c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPClient: a.client})
		if err != nil {
			t.Fatal(err)
		}
		defer c.CloseNow()
		connections = append(connections, c)
		if _, _, err = c.Read(ctx); err != nil {
			t.Fatal(err)
		}
	}
	u := managed(t, root, id)
	if u["connections"] != float64(2) || u["online"] != true {
		t.Fatal("multiple tabs miscounted", u)
	}
	if overview(t, root)["stats"].(map[string]any)["online"] != float64(2) {
		t.Fatal("tabs inflated online users")
	}
	connections[0].CloseNow()
	connections[1].CloseNow()
	s.mu.Lock()
	s.presence[id] = time.Now().Add(-61 * time.Second)
	s.mu.Unlock()
	if managed(t, root, id)["online"] != false {
		t.Fatal("stale presence remained online")
	}
}

func TestAdminChangesPersistAndFailedAuditRollsBack(t *testing.T) {
	s, ts, root := adminServer(t)
	a := newClient(t, ts.URL)
	a.register("PlayerOne")
	id := a.state()["user"].(map[string]any)["id"].(string)
	room := a.post("/api/rooms", map[string]any{"name": "Waiting", "kind": "dota", "capacity": 2}, 201)
	_, err := s.db.Exec("CREATE TRIGGER reject_audit BEFORE INSERT ON admin_audit BEGIN SELECT RAISE(ABORT,'test failure'); END")
	if err != nil {
		t.Fatal(err)
	}
	root.post("/api/admin/games/dota", map[string]bool{"enabled": false}, 500)
	root.post("/api/admin/users/"+id, map[string]string{"action": "disable"}, 500)
	if current(a)["status"] != "waiting" || managed(t, root, id)["disabled"] != false {
		t.Fatal("failed transaction changed room/user")
	}
	s.mu.Lock()
	hidden := s.hiddenGames["dota"]
	s.mu.Unlock()
	if hidden {
		t.Fatal("failed change mutated memory")
	}
	s.db.Exec("DROP TRIGGER reject_audit")
	root.post("/api/admin/games/dota", map[string]bool{"enabled": false}, 200)
	root.post("/api/admin/users/"+id, map[string]string{"action": "disable"}, 200)
	// A separate read of the persisted snapshots verifies the administrative
	// transaction, while a full restart verifies role, user and game reload.
	dir := s.cfg.DataDir
	ts.Close()
	s.Close()
	restarted, err := New(Config{DataDir: dir, InviteCode: "test-invite"}, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if !restarted.hiddenGames["dota"] || restarted.rooms[room["id"].(string)].CloseReason != "unpublished" {
		t.Fatal("settings/room not restored")
	}
	var disabled bool
	restarted.db.QueryRow("SELECT disabled FROM user_controls WHERE user_id=?", id).Scan(&disabled)
	if !disabled {
		t.Fatal("disabled account not restored")
	}
}
