package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

type testClient struct {
	t      *testing.T
	client *http.Client
	base   string
}

func newClient(t *testing.T, base string) *testClient {
	jar, _ := cookiejar.New(nil)
	return &testClient{t, &http.Client{Jar: jar, Timeout: 10 * time.Second}, base}
}
func (c *testClient) request(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	b, _ := json.Marshal(body)
	r, e := http.NewRequest(method, c.base+path, bytes.NewReader(b))
	if e != nil {
		c.t.Fatal(e)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	res, e := c.client.Do(r)
	if e != nil {
		c.t.Fatal(e)
	}
	defer res.Body.Close()
	var v map[string]any
	if e = json.NewDecoder(res.Body).Decode(&v); e != nil {
		c.t.Fatal(e)
	}
	return res.StatusCode, v
}
func (c *testClient) post(path string, body any, want int) map[string]any {
	c.t.Helper()
	code, v := c.request("POST", path, body)
	if code != want {
		c.t.Fatalf("POST %s got %d want %d: %v", path, code, want, v)
	}
	return v
}
func (c *testClient) state() map[string]any {
	c.t.Helper()
	code, v := c.request("GET", "/api/state", nil)
	if code != 200 {
		c.t.Fatal(code, v)
	}
	return v
}
func (c *testClient) register(name string) {
	c.post("/api/register", map[string]string{"name": name, "password": "test-password-123", "invite": "test-invite"}, 200)
}
func (c *testClient) command(room map[string]any, kind string, action any, want int) map[string]any {
	return c.post("/api/rooms/"+room["id"].(string), map[string]any{"type": kind, "version": room["version"], "nonce": randomID(12), "action": action}, want)
}
func current(c *testClient) map[string]any { return c.state()["room"].(map[string]any) }
func setupServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	s, e := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite"}, fstest.MapFS{"index.html": {Data: []byte("<h1>Wire Board</h1>")}})
	if e != nil {
		t.Fatal(e)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	return s, ts
}
func TestAuthCSRFAndStatic(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	code, _ := c.request("GET", "/api/state", nil)
	if code != 401 {
		t.Fatal("anonymous allowed")
	}
	c.post("/api/register", map[string]string{"name": "Alice", "password": "test-password-123", "invite": "wrong"}, 403)
	c.register("Alice")
	c.post("/api/register", map[string]string{"name": "Alice", "password": "test-password-123", "invite": "test-invite"}, 409)
	if c.state()["user"].(map[string]any)["name"] != "Alice" {
		t.Fatal("wrong user")
	}
	req, _ := http.NewRequest("POST", ts.URL+"/api/logout", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://attacker.example")
	res, e := c.client.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("CSRF accepted")
	}
	c.post("/api/logout", map[string]any{}, 200)
	code, _ = c.request("GET", "/api/state", nil)
	if code != 401 {
		t.Fatal("logout ineffective")
	}
	c.post("/api/login", map[string]string{"name": "Alice", "password": "test-password-123"}, 200)
	res, e = c.client.Get(ts.URL + "/")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(b), "Wire Board") {
		t.Fatal("static file serving failed")
	}
	code, _ = c.request("GET", "/api/no-such-route", nil)
	if code != 404 {
		t.Fatal("API fallback")
	}
}
func startRoom(t *testing.T, a, b *testClient, kind string) map[string]any {
	t.Helper()
	room := a.post("/api/rooms", map[string]any{"name": "Friends", "kind": kind, "capacity": 2}, 201)
	b.command(room, "join", nil, 200)
	a.command(current(a), "start", nil, 400)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	return current(a)
}
func TestRoomActionsIsolationIdempotencyAndRestart(t *testing.T) {
	s, ts := setupServer(t)
	a, b, other := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	other.register("Carol")
	room := startRoom(t, a, b, "splendor")
	other.command(room, "action", map[string]any{"type": "take", "tokens": []int{1, 1, 1, 0, 0, 0}}, 400)
	if _, ok := other.state()["room"]; ok {
		t.Fatal("outsider got game")
	}
	nonce := randomID(12)
	body := map[string]any{"type": "action", "version": room["version"], "nonce": nonce, "action": map[string]any{"type": "reserve", "tier": 1}}
	path := "/api/rooms/" + room["id"].(string)
	a.post(path, body, 200)
	updated := current(a)
	a.post(path, body, 200)
	if current(a)["version"] != updated["version"] {
		t.Fatal("duplicate command applied")
	}
	body["nonce"] = randomID(12)
	a.post(path, body, 409)
	a.command(current(a), "action", map[string]any{"type": "take", "tokens": []int{1, 1, 1, 0, 0, 0}}, 400)
	view := current(b)["game"].(map[string]any)["splendor"].(map[string]any)
	op := view["players"].([]any)[0].(map[string]any)
	if _, ok := op["reserved"]; ok {
		t.Fatal("private reservation sent")
	}
	a.command(current(a), "leave", nil, 400)
	before, _ := json.Marshal(current(a))
	ts.Close()
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	restarted, e := New(s.cfg, s.files)
	if e != nil {
		t.Fatal(e)
	}
	defer restarted.Close()
	http2 := httptest.NewServer(restarted.Handler())
	defer http2.Close()
	a.base = http2.URL
	after, _ := json.Marshal(current(a))
	if !bytes.Equal(before, after) {
		t.Fatal("restart changed state")
	}
	a.post(path, map[string]any{"type": "action", "version": room["version"], "nonce": nonce, "action": map[string]any{"type": "reserve", "tier": 1}}, 200)
	if current(a)["version"] != updated["version"] {
		t.Fatal("retry after restart applied again")
	}
}
func TestPasswordAndReadyRoomLifecycle(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := a.post("/api/rooms", map[string]any{"name": "Private", "kind": "rail", "capacity": 3, "password": "room-secret"}, 201)
	if _, ok := r["password"]; ok {
		t.Fatal("password hash leaked")
	}
	b.command(r, "join", nil, 400)
	b.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "join", "version": r["version"], "nonce": randomID(12), "password": "room-secret"}, 200)
	rows, e := s.db.Query("SELECT action FROM actions")
	if e != nil {
		t.Fatal(e)
	}
	for rows.Next() {
		var raw string
		rows.Scan(&raw)
		if strings.Contains(raw, "room-secret") {
			t.Fatal("plaintext password logged")
		}
	}
	rows.Close()
	a.command(current(a), "leave", nil, 200)
	if current(b)["host"] != b.state()["user"].(map[string]any)["id"] {
		t.Fatal("host not transferred")
	}
	b.command(current(b), "leave", nil, 200)
	if len(a.state()["rooms"].([]any)) != 0 {
		t.Fatal("empty room not deleted")
	}
}
func TestFinishedDeparturePreservesPlayerIndices(t *testing.T) {
	s, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := startRoom(t, a, b, "splendor")
	s.mu.Lock()
	room := s.rooms[r["id"].(string)]
	room.Status = "finished"
	room.Game.Finished = true
	room.Game.Winners = []int{1}
	room.Game.Splendor.Players[1].Score = 20
	_ = s.save(room)
	s.mu.Unlock()
	a.command(current(a), "leave", nil, 200)
	br := current(b)
	if br["you"] != float64(1) {
		t.Fatal("seat index changed")
	}
	g := br["game"].(map[string]any)
	if g["winners"].([]any)[0] != float64(1) {
		t.Fatal("winner changed")
	}
	if _, ok := a.state()["room"]; ok {
		t.Fatal("departed user still seated")
	}
	b.command(br, "rematch", nil, 200)
	br = current(b)
	if len(br["seats"].([]any)) != 1 || br["you"] != float64(0) || br["status"] != "waiting" {
		t.Fatal("rematch did not prune departed seats")
	}
}
func TestConcurrentCommandsOnlyOneCommits(t *testing.T) {
	_, ts := setupServer(t)
	a := newClient(t, ts.URL)
	a.register("Alice")
	r := a.post("/api/rooms", map[string]any{"name": "Friends", "kind": "splendor", "capacity": 2}, 201)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := a.request("POST", "/api/rooms/"+r["id"].(string), map[string]any{"type": "ready", "version": r["version"], "nonce": randomID(12)})
			codes <- code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatal(counts)
	}
}
func TestWebsocketNotificationsAuthAndOrigin(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("Alice")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, res, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", &websocket.DialOptions{})
	if conn != nil {
		conn.CloseNow()
	}
	if e == nil || res.StatusCode != 401 {
		t.Fatal("anonymous websocket accepted")
	}
	conn, _, e = websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPClient: c.client})
	if e != nil {
		t.Fatal(e)
	}
	defer conn.CloseNow()
	_, raw, e := conn.Read(ctx)
	if e != nil || string(raw) != `{"type":"changed"}` {
		t.Fatal("initial notification", e, string(raw))
	}
	c.post("/api/rooms", map[string]any{"name": "Friends", "kind": "splendor", "capacity": 2}, 201)
	_, raw, e = conn.Read(ctx)
	if e != nil || string(raw) != `{"type":"changed"}` {
		t.Fatal("mutation notification", e)
	}
	h := http.Header{}
	h.Set("Origin", "https://attacker.example")
	bad, res, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", &websocket.DialOptions{HTTPClient: c.client, HTTPHeader: h})
	if bad != nil {
		bad.CloseNow()
	}
	if e == nil || res.StatusCode != 403 {
		t.Fatal("cross-origin websocket accepted")
	}
}
func TestRailHiddenTasksThroughAPI(t *testing.T) {
	_, ts := setupServer(t)
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	r := startRoom(t, a, b, "rail")
	mine := r["game"].(map[string]any)["rail"].(map[string]any)
	other := current(b)["game"].(map[string]any)["rail"].(map[string]any)
	if _, ok := mine["pending"]; !ok {
		t.Fatal("active player has no tickets")
	}
	if _, ok := other["pending"]; ok {
		t.Fatal("pending tickets leaked")
	}
	for _, key := range []string{"deck", "ticketDeck", "discard"} {
		if _, ok := mine[key]; ok {
			t.Fatal("hidden pile leaked", key)
		}
	}
	for _, p := range other["players"].([]any)[:1] {
		raw, _ := json.Marshal(p)
		if strings.Contains(string(raw), `"hand":`) || strings.Contains(string(raw), `"tickets":`) {
			t.Fatal("opponent secrets leaked")
		}
	}
}
func TestMissingInviteRejected(t *testing.T) {
	_, e := New(Config{DataDir: t.TempDir()}, fstest.MapFS{})
	if e == nil {
		t.Fatal("missing invite accepted")
	}
}
func TestHealth(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	code, v := c.request("GET", "/healthz", nil)
	if code != 200 || fmt.Sprint(v["ok"]) != "true" {
		t.Fatal(code, v)
	}
}
