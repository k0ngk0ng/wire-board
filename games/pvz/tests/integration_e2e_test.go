package pvz_test

// These HTTP/SQLite contracts were written before registration and runtime
// implementation. Tests use public endpoints and the production Module.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/nettest"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func integrationModule(t *testing.T) gameapi.Module {
	t.Helper()
	var found []gameapi.Module
	for _, m := range games.Modules() {
		if m.ID == "pvz" {
			found = append(found, m)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want exactly one production pvz module; got %d", len(found))
	}
	return found[0]
}
func TestPvZProductionRegistration(t *testing.T) {
	m := integrationModule(t)
	if m.Name != "植物大战僵尸" || m.MinPlayers != 2 || m.MaxPlayers != 4 || !m.Realtime || m.BotAction == nil || m.Reclaim == nil || m.Restore == nil || m.SyncSnapshot == nil {
		t.Fatal("incomplete production contract")
	}
	for _, n := range []int{2, 4} {
		if _, err := m.Configure(n, nil); err != nil {
			t.Fatal(n, err)
		}
	}
	for _, n := range []int{1, 3, 5} {
		if _, err := m.Configure(n, nil); err == nil {
			t.Fatal("invalid capacity accepted", n)
		}
	}
	for _, raw := range []string{`{"scenario":"win"}`, `{"money":999999}`, `{"teams":[0,0]}`, `{"mode":"team","teams":[0,1]}`} {
		if _, err := m.Configure(2, json.RawMessage(raw)); err == nil {
			t.Fatal("invalid options accepted", raw)
		}
	}
}

type integrationClient struct {
	t      *testing.T
	http   *http.Client
	base   string
	serial int
}
type integrationTable struct {
	t        *testing.T
	cfg      server.Config
	host     *server.Server
	http     *httptest.Server
	people   []*integrationClient
	observer *integrationClient
	trace    []any
	out      string
}

func (c *integrationClient) request(method, path string, body any, want int) map[string]any {
	c.t.Helper()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(method, c.base+path, bytes.NewReader(raw))
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var v map[string]any
	if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
		c.t.Fatal(method, path, err)
	}
	if res.StatusCode != want {
		c.t.Fatalf("%s %s = %d want %d: %v", method, path, res.StatusCode, want, v)
	}
	return v
}
func (c *integrationClient) state() map[string]any { return c.request("GET", "/api/state", nil, 200) }
func (c *integrationClient) room() map[string]any {
	c.t.Helper()
	r, ok := c.state()["room"].(map[string]any)
	if !ok {
		c.t.Fatal("missing room")
	}
	return r
}
func integrationData(r map[string]any) map[string]any {
	return r["game"].(map[string]any)["data"].(map[string]any)
}
func (c *integrationClient) command(kind string, extra map[string]any, want int) map[string]any {
	c.t.Helper()
	r := c.room()
	c.serial++
	q := map[string]any{"type": kind, "version": r["version"], "nonce": fmt.Sprintf("pvz-http-%020d", c.serial)}
	for k, v := range extra {
		q[k] = v
	}
	return c.request("POST", "/api/rooms/"+r["id"].(string), q, want)
}
func (c *integrationClient) action(a map[string]any, want int) map[string]any {
	c.t.Helper()
	r := c.room()
	d := integrationData(r)
	a["epoch"] = d["epoch"]
	seat := int(r["you"].(float64))
	seq := 1.
	if seat >= 0 {
		seq = d["players"].([]any)[seat].(map[string]any)["seq"].(float64) + 1
	}
	a["seq"] = seq
	return c.command("action", map[string]any{"action": a}, want)
}
func integrationMoney(r map[string]any, seat int) float64 {
	return integrationData(r)["players"].([]any)[seat].(map[string]any)["money"].(float64)
}
func integrationNew(t *testing.T, n int) *integrationTable {
	t.Helper()
	out := filepath.Join("..", "..", "..", ".local", "pvz", "integration", strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	tb := &integrationTable{t: t, out: out, cfg: server.Config{DataDir: t.TempDir(), InviteCode: "pvz-browser", AdminUsername: "PvZRoot", AdminPassword: "pvz-browser-password", Modules: []gameapi.Module{integrationModule(t)}}}
	var err error
	tb.host, err = server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("PvZ HTTP E2E")}})
	if err != nil {
		t.Fatal(err)
	}
	tb.http = httptest.NewServer(tb.host.Handler())
	t.Cleanup(func() {
		tb.http.Close()
		tb.host.Close()
		raw, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "test": t.Name(), "players": n, "command": ". scripts/env.sh && go test ./games/pvz/tests -run '^" + t.Name() + "$' -count=1 -v", "scope": "production module; real HTTP, timers, authentication and SQLite; no injected money or positions", "trace": tb.trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, "summary.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	for i := 0; i <= n; i++ {
		jar, _ := cookiejar.New(nil)
		c := &integrationClient{t: t, http: &http.Client{Jar: jar, Timeout: 15 * time.Second}, base: tb.http.URL}
		if i == 0 {
			c.request("POST", "/api/login", map[string]any{"name": "PvZRoot", "password": "pvz-browser-password"}, 200)
		} else {
			c.request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("花园玩家%d", i), "password": "pvz-browser-password", "invite": "pvz-browser"}, 200)
		}
		if i < n {
			tb.people = append(tb.people, c)
		} else {
			tb.observer = c
		}
	}
	available := false
	for _, v := range tb.people[0].state()["availableGames"].([]any) {
		available = available || v == "pvz"
	}
	if !available {
		t.Fatal("pvz not published by default")
	}
	created := tb.people[0].request("POST", "/api/rooms", map[string]any{"kind": "pvz", "name": "植物大战僵尸验收", "capacity": n}, 201)
	id := created["id"].(string)
	for _, c := range tb.people[1:] {
		c.serial++
		c.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": tb.people[0].room()["version"], "nonce": fmt.Sprintf("pvz-join-%020d", c.serial)}, 200)
	}
	for _, c := range tb.people {
		c.command("ready", nil, 200)
	}
	tb.people[0].command("start", nil, 200)
	tb.observer.request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
	tb.trace = append(tb.trace, map[string]any{"kind": "natural-start", "room": tb.people[0].room()})
	return tb
}
func (tb *integrationTable) restart() {
	tb.t.Helper()
	tb.http.Close()
	tb.host.Close()
	var err error
	tb.host, err = server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("PvZ restart")}})
	if err != nil {
		tb.t.Fatal(err)
	}
	tb.http = httptest.NewServer(tb.host.Handler())
	for _, c := range append(append([]*integrationClient{}, tb.people...), tb.observer) {
		c.base = tb.http.URL
	}
}
func TestPvZIndependentEconomyAndOwnershipHTTP(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tb := integrationNew(t, n)
			a := tb.people[0]
			before := a.room()
			others := make([]float64, n)
			for i := range others {
				others[i] = integrationMoney(before, i)
			}
			a.action(map[string]any{"type": "plant", "kind": "sunflower", "row": 0, "col": 0}, 200)
			after := a.room()
			if integrationMoney(after, 0) >= others[0] {
				t.Fatal("planting did not debit personal wallet")
			}
			for i := 1; i < n; i++ {
				if integrationMoney(after, i) != others[i] {
					t.Fatal("planting changed another wallet")
				}
			}
			plants := integrationData(after)["plants"].([]any)
			if len(plants) != 1 || plants[0].(map[string]any)["owner"] != float64(0) {
				t.Fatal("plant owner not authenticated")
			}
			id := plants[0].(map[string]any)["id"]
			a.action(map[string]any{"type": "plant", "kind": "peashooter", "row": 0, "col": 0}, 400)
			a.action(map[string]any{"type": "plant", "kind": "peashooter", "row": -1, "col": 0}, 400)
			a.action(map[string]any{"type": "plant", "kind": "peashooter", "row": 0, "col": 1, "owner": 1}, 400)
			tb.people[1].action(map[string]any{"type": "spawn", "kind": "zombie", "row": 0}, 400)
			tb.observer.action(map[string]any{"type": "plant", "kind": "sunflower", "row": 1, "col": 1}, 400)
			if n == 4 {
				tb.people[2].action(map[string]any{"type": "shovel", "plantId": id}, 400)
			}
			money := integrationMoney(a.room(), 0)
			tb.restart()
			restored := a.room()
			if integrationMoney(restored, 0) != money || !reflect.DeepEqual(plants, integrationData(restored)["plants"]) {
				t.Fatal("restart lost plant or personal wallet")
			}
			a.action(map[string]any{"type": "shovel", "plantId": id}, 200)
			if len(integrationData(a.room())["plants"].([]any)) != 0 {
				t.Fatal("owner cannot shovel")
			}
			tb.trace = append(tb.trace, map[string]any{"kind": "personal-spend-ownership-restart", "room": a.room()})
		})
	}
}
func TestPvZConcurrentPlacementAndNonceHTTP(t *testing.T) {
	tb := integrationNew(t, 4)
	a, b := tb.people[0], tb.people[2]
	r := a.room()
	id := r["id"].(string)
	epoch := integrationData(r)["epoch"]
	bodies := []map[string]any{}
	for i := 0; i < 2; i++ {
		bodies = append(bodies, map[string]any{"type": "action", "version": r["version"], "nonce": fmt.Sprintf("pvz-concurrent-%020d", i), "action": map[string]any{"type": "plant", "epoch": epoch, "seq": 1, "kind": "sunflower", "row": 2, "col": 0}})
	}
	var wg sync.WaitGroup
	codes := make([]int, 2)
	clients := []*integrationClient{a, b}
	for i := range clients {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			raw, _ := json.Marshal(bodies[i])
			req, _ := http.NewRequest("POST", clients[i].base+"/api/rooms/"+id, bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			res, err := clients[i].http.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			defer res.Body.Close()
			codes[i] = res.StatusCode
		}(i)
	}
	wg.Wait()
	success := -1
	for i, c := range codes {
		if c == 200 {
			if success >= 0 {
				t.Fatal("both players occupied same cell")
			}
			success = i
		}
	}
	if success < 0 {
		t.Fatal("neither placement succeeded", codes)
	}
	after := a.room()
	if len(integrationData(after)["plants"].([]any)) != 1 {
		t.Fatal("duplicate entity")
	}
	money := []float64{integrationMoney(after, 0), integrationMoney(after, 2)}
	clients[success].request("POST", "/api/rooms/"+id, bodies[success], 200)
	retry := a.room()
	if integrationMoney(retry, 0) != money[0] || integrationMoney(retry, 2) != money[1] || len(integrationData(retry)["plants"].([]any)) != 1 {
		t.Fatal("nonce replay charged twice")
	}
	tb.trace = append(tb.trace, map[string]any{"kind": "concurrent-placement-nonce", "statuses": codes, "room": retry})
}

// Opt-in server uses the production registration/build. The browser script
// creates and plays real rooms; no test-only scenario option is accepted.
func TestPvZBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_PVZ_BROWSER") != "1" {
		t.Skip("browser fixture only")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/pvz/browser")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "pvz-browser", AdminUsername: "PvZRoot", AdminPassword: "pvz-browser-password", AssetsBaseURL: os.Getenv("PVZ_BROWSER_LEGACY_ASSETS"), ClientLoading: true, Modules: []gameapi.Module{integrationModule(t)}}, os.DirFS(filepath.Join(root, "web/dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	production := s.Handler()
	// Opt-in latency is isolated to one authenticated browser's test cookie.
	// Delayed websocket bytes remain pipelined and never delay the server clock.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, e := r.Cookie("pvz-test-rtt"); e == nil && c.Value == "320" {
			nettest.Delay(production, 320*time.Millisecond).ServeHTTP(w, r)
			return
		}
		production.ServeHTTP(w, r)
	})
	h := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	defer h.Close()
	go h.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for began := time.Now(); time.Since(began) < 85*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
