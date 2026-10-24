package conquest_test

// Written before implementation. Failure scenarios: registration hidden by
// default, unsupported settings admitted, unauthenticated/spectator commands,
// invalid actions committing, nonce replay, leaked plans, partial loading,
// restart losing orders/deadlines, bots unable to conquer every province,
// automatic takeover/reset changing the other players, and duplicate results.
// Full games start naturally and use real HTTP requests, SQLite commits and
// the production bot policy. No resources, outcomes or positions are injected.

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	conquest "github.com/k0ngk0ng/wire-board/games/conquest/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

type client struct {
	t      *testing.T
	http   *http.Client
	base   string
	serial int
}
type table struct {
	accelerated bool
	clockOffset atomic.Int64
	t           *testing.T
	cfg         server.Config
	host        *server.Server
	http        *httptest.Server
	people      []*client
	spectator   *client
	mu          sync.Mutex
	latest      gameapi.Match
	initial     gameapi.Match
	steps       int
	trace       *os.File
	out         string
}

func artifactDir(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", ".local", "conquest", "http", strings.ReplaceAll(t.Name(), "/", "-"))
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	return p
}
func (c *client) requestStatus(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		c.t.Fatal(err)
	}
	r, err := http.NewRequest(method, c.base+path, bytes.NewReader(raw))
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(r)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var v map[string]any
	if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
		c.t.Fatal(path, err)
	}
	return res.StatusCode, v
}
func (c *client) request(method, path string, body any, want int) map[string]any {
	c.t.Helper()
	status, v := c.requestStatus(method, path, body)
	if status != want {
		c.t.Fatalf("%s %s = %d want %d: %v", method, path, status, want, v)
	}
	return v
}
func (c *client) state() map[string]any { return c.request("GET", "/api/state", nil, 200) }
func (c *client) room() map[string]any {
	c.t.Helper()
	r, ok := c.state()["room"].(map[string]any)
	if !ok {
		c.t.Fatal("missing room")
	}
	return r
}
func data(r map[string]any) map[string]any {
	return r["game"].(map[string]any)["data"].(map[string]any)
}
func (c *client) command(kind string, extra map[string]any, want int) map[string]any {
	c.t.Helper()
	r := c.room()
	c.serial++
	q := map[string]any{"type": kind, "version": r["version"], "nonce": fmt.Sprintf("conquest-%012d", c.serial)}
	for k, v := range extra {
		q[k] = v
	}
	return c.request("POST", "/api/rooms/"+r["id"].(string), q, want)
}

// Only long, non-browser matrices advance virtual time. Production state and
// callbacks are unchanged; native contexts/envelopes are what replay records.
func (tb *table) clockContext(c gameapi.Context, due int64) gameapi.Context {
	if !tb.accelerated {
		return c
	}
	for {
		old := tb.clockOffset.Load()
		next := max(old, due-c.Now.UnixMilli())
		if tb.clockOffset.CompareAndSwap(old, next) {
			c.Now = c.Now.Add(time.Duration(next) * time.Millisecond)
			return c
		}
	}
}
func (tb *table) nativeMatch(m gameapi.Match) (gameapi.Match, error) {
	if !tb.accelerated {
		return m, nil
	}
	tb.mu.Lock()
	defer tb.mu.Unlock()
	if m.Schema != tb.latest.Schema || !bytes.Equal(m.Data, tb.latest.Data) {
		return m, fmt.Errorf("HTTP clock lost the native committed snapshot")
	}
	return tb.latest, nil
}
func (tb *table) hostMatch(m gameapi.Match) gameapi.Match {
	if !tb.accelerated {
		return m
	}
	offset := tb.clockOffset.Load()
	if m.Deadline > 0 {
		m.Deadline -= offset
	}
	if m.SystemDeadline > 0 {
		m.SystemDeadline = time.Now().UnixMilli() + 1
	}
	return m
}
func (tb *table) capture(kind string, c gameapi.Context, action json.RawMessage, next gameapi.Match, err error) (gameapi.Match, error) {
	if err != nil {
		return next, err
	}
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.latest = next
	if kind == "start" {
		tb.initial = next
	}
	b, _ := json.Marshal(next)
	sum := sha256.Sum256(b)
	record := map[string]any{"kind": kind, "context": c, "action": action, "sha256": fmt.Sprintf("%x", sum)}
	if kind == "start" || next.Result != nil {
		record["snapshot"] = next
	}
	if e := json.NewEncoder(tb.trace).Encode(record); e != nil {
		tb.t.Error(e)
	}
	tb.steps++
	return tb.hostMatch(next), nil
}
func (tb *table) match() gameapi.Match { tb.mu.Lock(); defer tb.mu.Unlock(); return tb.latest }
func newTable(t *testing.T, n int, mapID string, loading bool) *table {
	return newTableWithHumans(t, n, n, mapID, loading)
}
func newTableWithHumans(t *testing.T, n, humans int, mapID string, loading bool) *table {
	t.Helper()
	tb := &table{t: t, out: artifactDir(t), accelerated: strings.HasPrefix(t.Name(), "TestConquestFullGamesHTTP")}
	var err error
	tb.trace, err = os.Create(filepath.Join(tb.out, "replay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	m := conquest.Module()
	start, apply, timer, restore, reclaim := m.Start, m.Apply, m.OnTimer, m.Restore, m.Reclaim
	m.Start = func(c gameapi.Context, o json.RawMessage) (gameapi.Match, error) {
		c = tb.clockContext(c, 0)
		v, e := start(c, o)
		return tb.capture("start", c, o, v, e)
	}
	m.Apply = func(c gameapi.Context, s gameapi.Match, a json.RawMessage) (gameapi.Match, error) {
		native, err := tb.nativeMatch(s)
		if err != nil {
			return s, err
		}
		c = tb.clockContext(c, 0)
		v, e := apply(c, native, a)
		return tb.capture("apply", c, a, v, e)
	}
	if timer != nil {
		m.OnTimer = func(c gameapi.Context, s gameapi.Match) (gameapi.Match, error) {
			native, err := tb.nativeMatch(s)
			if err != nil {
				return s, err
			}
			c = tb.clockContext(c, native.SystemDeadline)
			v, e := timer(c, native)
			return tb.capture("timer", c, nil, v, e)
		}
	}
	if restore != nil {
		m.Restore = func(c gameapi.Context, s gameapi.Match) (gameapi.Match, error) {
			native, err := tb.nativeMatch(s)
			if err != nil {
				return s, err
			}
			c = tb.clockContext(c, 0)
			v, e := restore(c, native)
			return tb.capture("restore", c, nil, v, e)
		}
	}
	if reclaim != nil {
		m.Reclaim = func(c gameapi.Context, s gameapi.Match) (gameapi.Match, error) {
			native, err := tb.nativeMatch(s)
			if err != nil {
				return s, err
			}
			c = tb.clockContext(c, 0)
			v, e := reclaim(c, native)
			return tb.capture("reclaim", c, nil, v, e)
		}
	}
	if policy := m.BotAction; tb.accelerated && policy != nil {
		m.BotAction = func(c gameapi.Context, s gameapi.Match) (json.RawMessage, error) {
			native, err := tb.nativeMatch(s)
			if err != nil {
				return nil, err
			}
			return policy(tb.clockContext(c, 0), native)
		}
	}
	if accept := m.AcceptStale; tb.accelerated && accept != nil {
		m.AcceptStale = func(c gameapi.Context, s gameapi.Match, a json.RawMessage) bool {
			native, err := tb.nativeMatch(s)
			if err != nil {
				return false
			}
			return accept(tb.clockContext(c, 0), native, a)
		}
	}
	tb.cfg = server.Config{DataDir: t.TempDir(), InviteCode: "conquest-test", AdminUsername: "征服管理员", AdminPassword: "conquest-test-password", ClientLoading: loading, Modules: []gameapi.Module{m}}
	tb.host, err = server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Conquest HTTP E2E")}})
	if err != nil {
		t.Fatal(err)
	}
	tb.http = httptest.NewServer(tb.host.Handler())
	t.Cleanup(func() {
		tb.http.Close()
		tb.host.Close()
		tb.trace.Close()
		b, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "map": mapID, "players": n, "scope": "natural start; real HTTP and SQLite; production bot decisions through authenticated human test accounts; long matrix advances only monotonic test time and replays native contexts", "acceleratedClock": tb.accelerated, "steps": tb.steps, "final": tb.latest}, "", "  ")
		if e := os.WriteFile(filepath.Join(tb.out, "summary.json"), b, 0600); e != nil {
			t.Error(e)
		}
	})
	for i := 0; i <= humans; i++ {
		jar, _ := cookiejar.New(nil)
		c := &client{t: t, http: &http.Client{Jar: jar, Timeout: 15 * time.Second}, base: tb.http.URL}
		if i == 0 {
			c.request("POST", "/api/login", map[string]any{"name": "征服管理员", "password": "conquest-test-password"}, 200)
		} else {
			c.request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("征服玩家%d", i), "password": "conquest-test-password", "invite": "conquest-test"}, 200)
		}
		if i < humans {
			tb.people = append(tb.people, c)
		} else {
			tb.spectator = c
		}
	}
	found := false
	for _, g := range tb.people[0].state()["availableGames"].([]any) {
		if g == "conquest" {
			found = true
		}
	}
	if !found {
		t.Fatal("new game is not enabled by default")
	}
	r := tb.people[0].request("POST", "/api/rooms", map[string]any{"kind": "conquest", "name": "征服时代验收", "capacity": n, "options": map[string]any{"mapId": mapID}}, 201)
	id := r["id"].(string)
	for _, c := range tb.people[1:] {
		c.serial++
		c.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": tb.people[0].room()["version"], "nonce": fmt.Sprintf("conquest-%012d", c.serial)}, 200)
	}
	for i := humans; i < n; i++ {
		tb.people[0].command("add_bot", nil, 200)
	}
	for _, c := range tb.people {
		c.command("ready", nil, 200)
	}
	tb.people[0].command("start", nil, 200)
	tb.spectator.request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return tb
}
func (tb *table) restart() {
	tb.t.Helper()
	before := tb.people[0].room()
	tb.http.Close()
	tb.host.Close()
	var err error
	tb.host, err = server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Conquest HTTP E2E")}})
	if err != nil {
		tb.t.Fatal(err)
	}
	tb.http = httptest.NewServer(tb.host.Handler())
	for _, c := range append(append([]*client{}, tb.people...), tb.spectator) {
		c.base = tb.http.URL
	}
	after := tb.people[0].room()
	if !reflect.DeepEqual(data(before), data(after)) {
		tb.t.Fatal("restart changed game view")
	}
}

func TestConquestRegistrationAndInvalidHTTP(t *testing.T) {
	for _, mapID := range []string{"world", "europe", "usa", "china"} {
		t.Run(mapID, func(t *testing.T) {
			tb := newTable(t, 2, mapID, false)
			a := tb.people[0]
			before := a.room()
			a.command("action", map[string]any{"action": map[string]any{"type": "invented", "gold": 999999}}, 400)
			if !reflect.DeepEqual(data(before), data(a.room())) {
				t.Fatal("rejected action changed data")
			}
			tb.spectator.command("action", map[string]any{"action": map[string]any{"type": "commit"}}, 400)
			tb.restart()
		})
	}
}

func moveAction(t *testing.T, c *client) map[string]any {
	t.Helper()
	d := data(c.room())
	seat := int(c.room()["you"].(float64))
	var source string
	for _, v := range d["provinces"].([]any) {
		p := v.(map[string]any)
		if p["owner"] == float64(seat) && p["troops"].(float64) > 1 {
			source = p["id"].(string)
			break
		}
	}
	if source == "" {
		t.Fatal("no initial army")
	}
	b, e := os.ReadFile(filepath.Join("..", "maps", d["mapId"].(string)+".json"))
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Provinces []struct {
			ID        string   `json:"id"`
			Neighbors []string `json:"neighbors"`
			Sea       []string `json:"seaNeighbors"`
		} `json:"provinces"`
	}
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	for _, p := range m.Provinces {
		if p.ID == source {
			targets := append(p.Neighbors, p.Sea...)
			if len(targets) > 0 {
				return map[string]any{"type": "move", "from": source, "to": targets[0], "amount": 1, "session": d["session"], "epoch": d["epoch"]}
			}
		}
	}
	t.Fatal("initial region has no route")
	return nil
}
func (c *client) act(kind string, extra map[string]any, want int) map[string]any {
	c.t.Helper()
	d := data(c.room())
	a := map[string]any{"type": kind, "session": d["session"], "epoch": d["epoch"]}
	for k, v := range extra {
		a[k] = v
	}
	return c.command("action", map[string]any{"action": a}, want)
}
func TestConquestConcurrentPlansSpectatorRestoreHTTP(t *testing.T) {
	tb := newTable(t, 4, "europe", false)
	a, b := tb.people[0], tb.people[1]
	actionA, actionB := moveAction(t, a), moveAction(t, b)
	original := a.room()
	version := original["version"]
	id := original["id"].(string)
	q := map[string]any{"type": "action", "version": version, "nonce": "plan-duplicate-00001", "action": actionA}
	a.request("POST", "/api/rooms/"+id, q, 200)
	a.request("POST", "/api/rooms/"+id, q, 200)
	if len(data(a.room())["orders"].([]any)) != 1 {
		t.Fatal("replayed nonce queued an order twice")
	}
	// Same planning epoch admits a different player's original room version.
	b.request("POST", "/api/rooms/"+id, map[string]any{"type": "action", "version": version, "nonce": "plan-concurrent-0001", "action": actionB}, 200)
	if len(data(b.room())["orders"].([]any)) != 1 {
		t.Fatal("independent plan was not accepted")
	}
	if len(data(tb.spectator.room())["orders"].([]any)) != 0 || len(data(tb.people[2].room())["orders"].([]any)) != 0 {
		t.Fatal("enemy/spectator received private orders")
	}
	if !reflect.DeepEqual(data(original)["provinces"], data(a.room())["provinces"]) {
		t.Fatal("queued move changed public troop positions before resolution")
	}
	before := data(a.room())
	bad := moveAction(t, a)
	bad["amount"] = -1
	a.command("action", map[string]any{"action": bad}, 400)
	if !reflect.DeepEqual(before, data(a.room())) {
		t.Fatal("negative movement mutated plan")
	}
	tb.restart()
	a.act("submit", nil, 200)
	a.command("action", map[string]any{"action": actionA}, 400)
	a.act("unsubmit", nil, 200)
	orders := data(a.room())["orders"].([]any)
	a.act("undo", map[string]any{"id": orders[0].(map[string]any)["id"]}, 200)
	if len(data(a.room())["orders"].([]any)) != 0 {
		t.Fatal("undo did not remove queued move")
	}
	for _, c := range tb.people {
		c.act("submit", nil, 200)
	}
	for until := time.Now().Add(15 * time.Second); time.Now().Before(until); {
		if data(a.room())["round"].(float64) > data(original)["round"].(float64) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if data(a.room())["round"].(float64) <= data(original)["round"].(float64) {
		t.Fatal("submitted round did not resolve")
	}
	a.command("action", map[string]any{"action": actionA}, 400)
	if err := tb.trace.Sync(); err != nil {
		t.Fatal(err)
	}
	verifyReplay(t, filepath.Join(tb.out, "replay.jsonl"))
}

func TestConquestPauseAutoPlayHTTP(t *testing.T) {
	tb := newTable(t, 3, "world", false)
	a := tb.people[0]
	for _, c := range tb.people {
		c.act("pause", map[string]any{"enabled": true}, 200)
	}
	r := a.room()
	if r["game"].(map[string]any)["paused"] != true || data(r)["phase"] != "paused" {
		t.Fatal("unanimous pause did not pause")
	}
	a.act("submit", nil, 400)
	before := data(a.room())
	tb.restart()
	a.command("autoplay", map[string]any{"enabled": true}, 200)
	time.Sleep(1200 * time.Millisecond)
	if !reflect.DeepEqual(before, data(a.room())) {
		t.Fatal("autoplay progressed paused game")
	}
	a.command("autoplay", map[string]any{"enabled": false}, 200)
	for _, c := range tb.people {
		c.act("resume", map[string]any{"enabled": true}, 200)
	}
	if data(a.room())["phase"] != "planning" {
		t.Fatal("resume did not restore planning")
	}
	deadline := data(a.room())["players"].([]any)[0].(map[string]any)["deadline"]
	b := tb.people[1]
	b.command("autoplay", map[string]any{"enabled": true}, 200)
	for until := time.Now().Add(10 * time.Second); time.Now().Before(until); {
		if len(data(b.room())["orders"].([]any)) > 0 || data(b.room())["players"].([]any)[1].(map[string]any)["submitted"] == true {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if len(data(b.room())["orders"].([]any)) == 0 && data(b.room())["players"].([]any)[1].(map[string]any)["submitted"] != true {
		t.Fatal("autoplay did not make a legal plan")
	}
	b.command("autoplay", map[string]any{"enabled": false}, 200)
	if data(a.room())["players"].([]any)[0].(map[string]any)["deadline"] != deadline {
		t.Fatal("manual takeover changed another player's deadline")
	}
}

func TestConquestLoadingHTTP(t *testing.T) {
	tb := newTable(t, 2, "europe", true)
	a, b := tb.people[0], tb.people[1]
	r := a.room()
	gate, ok := r["loading"].(map[string]any)
	if !ok {
		t.Fatal("missing initial client loading barrier")
	}
	initial := data(r)
	a.act("submit", nil, 400)
	tb.spectator.command("load_ready", map[string]any{"loadToken": gate["token"], "loadBuild": gate["build"], "loadClient": "observer"}, 400)
	for i, c := range []*client{a, b} {
		extra := map[string]any{"loadToken": gate["token"], "loadBuild": gate["build"], "loadClient": fmt.Sprintf("conquest-loading-%d", i)}
		c.command("load_begin", extra, 200)
		c.command("load_ready", extra, 200)
		if i == 0 {
			if _, ok := a.room()["loading"].(map[string]any); !ok {
				t.Fatal("first ready human prematurely started game")
			}
		}
	}
	if _, ok := a.room()["loading"].(map[string]any); ok {
		t.Fatal("all humans ready did not release loading")
	}
	if data(a.room())["round"] != initial["round"] {
		t.Fatal("loading advanced a round")
	}
	a.act("submit", nil, 200)
	if _, ok := b.room()["loading"].(map[string]any); ok {
		t.Fatal("ordinary snapshot re-entered loading")
	}
}

// Failure discovered by independent review before the pause-budget fix: a
// submitted seat had its deadline erased, then could not safely unsubmit.
func TestConquestSubmittedPauseReclaimHTTP(t *testing.T) {
	tb := newTable(t, 3, "europe", false)
	a := tb.people[0]
	before := data(a.room())["players"].([]any)[0].(map[string]any)["deadline"].(float64)
	a.act("submit", nil, 200)
	for _, c := range tb.people {
		c.act("pause", map[string]any{"enabled": true}, 200)
	}
	time.Sleep(100 * time.Millisecond)
	tb.restart()
	for _, c := range tb.people {
		c.act("resume", map[string]any{"enabled": true}, 200)
	}
	a.act("unsubmit", nil, 200)
	after := data(a.room())["players"].([]any)[0].(map[string]any)["deadline"].(float64)
	if after < before || after > before+3000 {
		t.Fatalf("submitted seat lost or reset frozen budget: %v -> %v", before, after)
	}
	if e := tb.match().Validate(3); e != nil {
		t.Fatal(e)
	}
	matched := false
	for _, seat := range tb.match().TimeoutActors {
		if seat == 0 {
			matched = true
		}
	}
	if !matched {
		t.Fatal("unsubmitted seat disappeared from shared deadline")
	}
}

func TestConquestAutoPlayMixedCompleteHTTP(t *testing.T) {
	if os.Getenv("WIRE_BOARD_CONQUEST_SCHEDULED") != "1" {
		t.Skip("opt-in paced platform scheduler full game; natural complete HTTP matrix is always run")
	}
	tb := newTableWithHumans(t, 2, 1, "europe", false)
	a := tb.people[0]
	if a.room()["seats"].([]any)[1].(map[string]any)["bot"] != true {
		t.Fatal("opponent is not a platform computer seat")
	}
	a.command("autoplay", map[string]any{"enabled": true}, 200)
	for until := time.Now().Add(25 * time.Minute); time.Now().Before(until); {
		match := tb.match()
		if match.Result != nil {
			if len(match.Result.Winners) != 1 || match.Result.Draw {
				t.Fatal("scheduler game did not finish normally")
			}
			if err := tb.trace.Sync(); err != nil {
				t.Fatal(err)
			}
			verifyReplay(t, filepath.Join(tb.out, "replay.jsonl"))
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("paced platform computers did not complete a game")
}

func TestConquestFullGamesHTTP(t *testing.T) {
	for _, mapID := range []string{"world", "europe", "usa", "china"} {
		for n := 2; n <= 8; n++ {
			t.Run(fmt.Sprintf("%s-%d", mapID, n), func(t *testing.T) {
				t.Parallel()
				tb := newTable(t, n, mapID, false)
				policy := conquest.Module().BotAction
				counts := map[string]int{}
				until := time.Now().Add(12 * time.Minute)
				cursor := 0
				for step := 0; step < 100000 && time.Now().Before(until); step++ {
					match := tb.match()
					if match.Result != nil {
						if len(match.Result.Winners) != 1 || match.Result.Draw {
							t.Fatal("natural game did not produce one conqueror", match.Result)
						}
						winner := match.Result.Winners[0]
						room := tb.people[winner].room()
						if room["game"].(map[string]any)["finished"] != true {
							t.Fatal("result was not committed through HTTP")
						}
						for _, p := range data(room)["players"].([]any) {
							player := p.(map[string]any)
							if player["seat"] != float64(winner) && player["eliminated"] != true {
								t.Fatal("victory with a surviving rival", player)
							}
						}
						b, _ := json.MarshalIndent(counts, "", "  ")
						if e := os.WriteFile(filepath.Join(tb.out, "action-counts.json"), b, 0600); e != nil {
							t.Fatal(e)
						}
						if err := tb.trace.Sync(); err != nil {
							t.Fatal(err)
						}
						verifyReplay(t, filepath.Join(tb.out, "replay.jsonl"))
						return
					}
					acted := false
					for j := 0; j < n; j++ {
						seat := (cursor + j) % n
						eligible := false
						for _, a := range match.Actors {
							if a == seat {
								eligible = true
							}
						}
						if !eligible {
							continue
						}
						raw, err := policy(tb.clockContext(gameapi.Context{Actor: seat, Players: n, Now: time.Now(), Automated: make([]bool, n)}, 0), match)
						if err != nil {
							t.Fatal("bot policy", seat, err)
						}
						if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
							continue
						}
						var action map[string]any
						if err = json.Unmarshal(raw, &action); err != nil {
							t.Fatal(err)
						}
						c := tb.people[seat]
						r := c.room()
						c.serial++
						code, response := c.requestStatus("POST", "/api/rooms/"+r["id"].(string), map[string]any{"type": "action", "version": r["version"], "nonce": fmt.Sprintf("conquest-%012d", c.serial), "action": action})
						if code != 200 {
							if fresh := c.room(); (code == 409 || code == 400) && fresh["version"] != r["version"] {
								acted = true
								break
							}
							t.Fatal("bot decision rejected", step, seat, string(raw), code, response)
						}
						counts[action["type"].(string)]++
						cursor = (seat + 1) % n
						acted = true
						break
					}
					if !acted {
						time.Sleep(20 * time.Millisecond)
					}
				}
				t.Fatal("production bots did not finish a natural complete game", counts)
			})
		}
	}
}

// Opt-in real production module and full web build, no scenario/backdoor.
func TestConquestBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_CONQUEST_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local", "conquest", "browser")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	s, err := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "conquest-browser", AdminUsername: "征服验收管理员", AdminPassword: "conquest-browser-test", ClientLoading: true, Modules: []gameapi.Module{conquest.Module()}}, os.DirFS(filepath.Join(root, "web", "dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	hs := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer hs.Close()
	go hs.Serve(ln)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+ln.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for begin := time.Now(); time.Since(begin) < 30*time.Minute; {
		if _, e := os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture exceeded 30 minutes")
}

// Replay checks every committed successful callback against its original clock
// and input, proving that the saved initial state contains necessary randomness.
func verifyReplay(t *testing.T, path string) {
	t.Helper()
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	d := json.NewDecoder(f)
	m := conquest.Module()
	var state gameapi.Match
	for step := 0; ; step++ {
		var r struct {
			Kind     string          `json:"kind"`
			Context  gameapi.Context `json:"context"`
			Action   json.RawMessage `json:"action"`
			SHA      string          `json:"sha256"`
			Snapshot gameapi.Match   `json:"snapshot"`
		}
		if e = d.Decode(&r); e == io.EOF {
			break
		} else if e != nil {
			t.Fatal(e)
		}
		switch r.Kind {
		case "start":
			state = r.Snapshot
		case "apply":
			state, e = m.Apply(r.Context, state, r.Action)
		case "timer":
			state, e = m.OnTimer(r.Context, state)
		case "restore":
			state, e = m.Restore(r.Context, state)
		case "reclaim":
			state, e = m.Reclaim(r.Context, state)
		default:
			t.Fatal("unknown replay record", r.Kind)
		}
		if e != nil {
			t.Fatal("replay failed", step, e)
		}
		b, _ := json.Marshal(state)
		sum := sha256.Sum256(b)
		if fmt.Sprintf("%x", sum) != r.SHA {
			t.Fatal("replay diverged", step, r.Kind)
		}
	}
}

// The full HTTP matrix advances only test time. These checks retain real HTTP
// and SQLite, preserve production event durations, and replay native envelopes.
func TestConquestFullGamesHTTPClockE2E(t *testing.T) {
	tb := newTable(t, 2, "europe", false)
	a, b := tb.people[0], tb.people[1]
	d := data(a.room())
	home := d["players"].([]any)[0].(map[string]any)["king"].(string)
	a.act("recruit", map[string]any{"province": home, "amount": 10}, 200)
	a.act("submit", nil, 200)
	started := time.Now()
	b.act("submit", nil, 200)
	for time.Since(started) < 700*time.Millisecond && data(a.room())["round"].(float64) == 1 {
		time.Sleep(10 * time.Millisecond)
	}
	d = data(a.room())
	if d["round"].(float64) != 2 {
		t.Fatal("full-game fixture still waits for wall-clock animation")
	}
	events := d["events"].([]any)
	e := events[0].(map[string]any)
	if e["until"].(float64)-e["at"].(float64) < 900 || e["displayUntil"].(float64)-e["until"].(float64) < 500 {
		t.Fatal("test clock changed the real animation durations", e)
	}
	deadline := a.room()["turnDeadline"].(float64)
	if remaining := deadline - float64(time.Now().UnixMilli()); remaining < 118000 || remaining > 121000 {
		t.Fatal("test acceleration shortened planning window", remaining)
	}
	tb.restart()
	if data(a.room())["round"].(float64) != 2 {
		t.Fatal("restart lost accelerated state")
	}
	if err := tb.trace.Sync(); err != nil {
		t.Fatal(err)
	}
	verifyReplay(t, filepath.Join(tb.out, "replay.jsonl"))
}
