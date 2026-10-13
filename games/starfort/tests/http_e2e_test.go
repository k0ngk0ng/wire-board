package starfort_test

// Failure scenarios, declared before the platform integration: a new game
// appears without administrator publication; solo/four-player tables rejected;
// invalid or test-only options accepted; spectators/other seats spend gold;
// duplicate nonces or concurrent same-pad builds charge twice; stale epoch
// crosses wave boundary; timer needs a browser action; pause advances the game;
// restart changes paused state; bot control stalls; cooperative results mint
// competitive points or invent winners; archives duplicate across rematch.
// Retained JSON is HTTP evidence, not a claim that each scenario is a full game.
import (
	"bytes"
	"encoding/json"
	"fmt"
	starfort "github.com/k0ngk0ng/wire-board/games/starfort/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
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
)

type client struct {
	t      *testing.T
	http   *http.Client
	base   string
	serial int
}
type table struct {
	t         *testing.T
	host      *server.Server
	http      *httptest.Server
	cfg       server.Config
	people    []*client
	spectator *client
	trace     []any
}

func (c *client) requestStatus(method, path string, body any) (int, map[string]any) {
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
		c.t.Fatal(err)
	}
	return res.StatusCode, v
}
func (c *client) request(method, path string, body any, want int) map[string]any {
	c.t.Helper()
	status, value := c.requestStatus(method, path, body)
	if status != want {
		c.t.Fatalf("%s %s = %d want %d: %v", method, path, status, want, value)
	}
	return value
}
func (c *client) state() map[string]any { return c.request("GET", "/api/state", nil, 200) }
func (c *client) room() map[string]any {
	c.t.Helper()
	v := c.state()["room"]
	if v == nil {
		c.t.Fatal("missing current room")
	}
	return v.(map[string]any)
}
func data(r map[string]any) map[string]any {
	return r["game"].(map[string]any)["data"].(map[string]any)
}
func (c *client) command(kind string, body map[string]any, want int) map[string]any {
	c.t.Helper()
	r := c.room()
	c.serial++
	req := map[string]any{"type": kind, "version": r["version"], "nonce": fmt.Sprintf("starfort-%012d", c.serial)}
	for k, v := range body {
		req[k] = v
	}
	return c.request("POST", "/api/rooms/"+r["id"].(string), req, want)
}
func (c *client) act(action map[string]any, want int) {
	c.t.Helper()
	if _, ok := action["epoch"]; !ok {
		action["epoch"] = data(c.room())["epoch"]
	}
	c.command("action", map[string]any{"action": action}, want)
}
func gold(r map[string]any, i int) float64 {
	return data(r)["players"].([]any)[i].(map[string]any)["gold"].(float64)
}
func (tb *table) record(label string) {
	tb.trace = append(tb.trace, map[string]any{"label": label, "room": tb.people[0].room()})
}
func (tb *table) restart() {
	tb.t.Helper()
	saved := tb.people[0].room()
	tb.http.Close()
	tb.host.Close()
	s, err := server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Starfort HTTP")}})
	if err != nil {
		tb.t.Fatal(err)
	}
	tb.host = s
	tb.http = httptest.NewServer(s.Handler())
	for _, c := range append(tb.people, tb.spectator) {
		c.base = tb.http.URL
	}
	if !reflect.DeepEqual(saved, tb.people[0].room()) {
		tb.t.Fatal("restart changed paused state")
	}
	tb.record("restarted persisted paused room")
}
func newHost(t *testing.T, n int, module gameapi.Module) *table {
	t.Helper()
	tb := &table{t: t, cfg: server.Config{DataDir: t.TempDir(), InviteCode: "starfort-test", AdminUsername: "星垒0", AdminPassword: "test-password-123", Modules: []gameapi.Module{module}}}
	var err error
	tb.host, err = server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Starfort HTTP")}})
	if err != nil {
		t.Fatal(err)
	}
	tb.http = httptest.NewServer(tb.host.Handler())
	t.Cleanup(func() {
		tb.http.Close()
		tb.host.Close()
		out := filepath.Join("..", "..", "..", ".local", "starfort", "http")
		if e := os.MkdirAll(out, 0755); e != nil {
			t.Error(e)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "real HTTP sessions and persistence; fixture scope labelled in trace", "trace": tb.trace}, "", "  ")
		if e := os.WriteFile(filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); e != nil {
			t.Error(e)
		}
	})
	for i := 0; i < n+1; i++ {
		jar, _ := cookiejar.New(nil)
		c := &client{t: t, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}, base: tb.http.URL}
		if i == 0 {
			c.request("POST", "/api/login", map[string]any{"name": "星垒0", "password": "test-password-123"}, 200)
		} else {
			c.request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("星垒%d", i), "password": "test-password-123", "invite": "starfort-test"}, 200)
		}
		if i < n {
			tb.people = append(tb.people, c)
		} else {
			tb.spectator = c
		}
	}
	return tb
}
func (tb *table) start(options map[string]any) {
	tb.t.Helper()
	if options == nil {
		options = map[string]any{}
	}
	a := tb.people[0]
	a.request("POST", "/api/admin/games/starfort", map[string]bool{"enabled": true}, 200)
	r := a.request("POST", "/api/rooms", map[string]any{"kind": "starfort", "name": "星垒验收", "capacity": len(tb.people), "options": options}, 201)
	id := r["id"].(string)
	for _, c := range tb.people[1:] {
		c.serial++
		c.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": a.room()["version"], "nonce": fmt.Sprintf("starfort-%012d", c.serial)}, 200)
	}
	for _, c := range tb.people {
		c.command("ready", nil, 200)
	}
	a.command("start", nil, 200)
	tb.spectator.request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
	tb.record("started")
}
func contains(v any, s string) bool {
	for _, x := range v.([]any) {
		if x == s {
			return true
		}
	}
	return false
}
func wait(t *testing.T, label string, check func() bool) {
	t.Helper()
	for until := time.Now().Add(8 * time.Second); time.Now().Before(until); {
		if check() {
			return
		}
		time.Sleep(60 * time.Millisecond)
	}
	t.Fatal("timed out", label)
}

func TestStarfortPublicationAndCapacityHTTP(t *testing.T) {
	for _, n := range []int{1, 2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tb := newHost(t, n, starfort.Module())
			a := tb.people[0]
			if contains(a.state()["availableGames"], "starfort") {
				t.Fatal("new game published by default")
			}
			a.request("POST", "/api/rooms", map[string]any{"kind": "starfort", "name": "默认下架", "capacity": n}, 403)
			tb.spectator.request("POST", "/api/admin/games/starfort", map[string]bool{"enabled": true}, 403)
			a.request("POST", "/api/admin/games/starfort", map[string]bool{"enabled": true}, 200)
			for _, opts := range []map[string]any{{"scenario": "battle"}, {"mode": "invalid"}, {"waves": 3}, {"difficulty": "impossible"}, {"prepSeconds": 1}, {"gold": 99999}} {
				a.request("POST", "/api/rooms", map[string]any{"kind": "starfort", "name": "非法设置", "capacity": n, "options": opts}, 400)
			}
			for _, capacity := range []int{0, 5} {
				a.request("POST", "/api/rooms", map[string]any{"kind": "starfort", "name": "非法人数", "capacity": capacity}, 400)
			}
			tb.start(map[string]any{"mode": "campaign", "waves": 20, "difficulty": "easy", "prepSeconds": 45})
			if len(data(a.room())["players"].([]any)) != n {
				t.Fatal("incorrect seats")
			}
			tb.spectator.act(map[string]any{"type": "build", "pad": 0, "tower": "arrow"}, 400)
			if data(tb.spectator.room())["controls"].(map[string]any)["canAct"] != false {
				t.Fatal("spectator enabled")
			}
			a.command("options", map[string]any{"options": map[string]any{"mode": "endless"}}, 400)
			for _, c := range tb.people {
				c.act(map[string]any{"type": "pause"}, 200)
			}
			tb.restart()
			if !contains(a.state()["availableGames"], "starfort") {
				t.Fatal("publication lost on restart")
			}
			tb.record("capacity/publication/auth validated")
		})
	}
}

func TestStarfortBuildConcurrencyHTTP(t *testing.T) {
	tb := newHost(t, 2, starfort.Module())
	tb.start(nil)
	a, b := tb.people[0], tb.people[1]
	before := a.room()
	oldGold := gold(before, 0)
	a.serial++
	req := map[string]any{"type": "action", "version": before["version"], "nonce": fmt.Sprintf("starfort-%012d", a.serial), "action": map[string]any{"type": "build", "pad": 0, "tower": "arrow", "epoch": data(before)["epoch"]}}
	path := "/api/rooms/" + before["id"].(string)
	a.request("POST", path, req, 200)
	after := a.room()
	a.request("POST", path, req, 200)
	if !reflect.DeepEqual(after, a.room()) || gold(after, 0) >= oldGold {
		t.Fatal("duplicate charge or no charge")
	}
	for _, action := range []map[string]any{{"type": "build", "pad": 0, "tower": "arrow"}, {"type": "build", "pad": 999, "tower": "arrow"}, {"type": "build", "pad": 1, "tower": "invalid"}, {"type": "gift", "target": 1, "amount": 999999}, {"type": "build", "pad": 1, "tower": "arrow", "actor": 1}} {
		saved := a.room()
		a.act(action, 400)
		if !reflect.DeepEqual(saved, a.room()) {
			t.Fatal("invalid action changed state", action)
		}
	}
	id := data(after)["towers"].([]any)[0].(map[string]any)["id"]
	b.act(map[string]any{"type": "sell", "towerId": id}, 400)
	snap := a.room()
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, c := range []*client{a, b} {
		c.serial++
		body := map[string]any{"type": "action", "version": snap["version"], "nonce": fmt.Sprintf("starfort-%012d", c.serial), "action": map[string]any{"type": "build", "pad": 1, "tower": "arrow", "epoch": data(snap)["epoch"]}}
		wg.Add(1)
		go func(c *client, body any) {
			defer wg.Done()
			status, _ := c.requestStatus("POST", path, body)
			codes <- status
		}(c, body)
	}
	wg.Wait()
	close(codes)
	success, failure := 0, 0
	for status := range codes {
		if status == 200 {
			success++
		} else if status == 400 {
			failure++
		} else {
			t.Fatal("unexpected conflict status", status)
		}
	}
	if success != 1 || failure != 1 || len(data(a.room())["towers"].([]any)) != 2 {
		t.Fatal("same-pad atomicity")
	}
	tb.record("nonce and competing builders")
}

func TestStarfortTimerPauseAndResumeHTTP(t *testing.T) {
	tb := newHost(t, 2, starfort.Module())
	tb.start(nil)
	a, b := tb.people[0], tb.people[1]
	a.act(map[string]any{"type": "pause"}, 200)
	if data(a.room())["phase"] != "prepare" {
		t.Fatal("one player paused team")
	}
	b.act(map[string]any{"type": "pause"}, 200)
	saved := a.room()
	time.Sleep(1200 * time.Millisecond)
	if !reflect.DeepEqual(saved, a.room()) {
		t.Fatal("paused timer moved")
	}
	tb.restart()
	a.act(map[string]any{"type": "resume"}, 200)
	if data(a.room())["phase"] != "paused" {
		t.Fatal("one player resumed team")
	}
	b.act(map[string]any{"type": "resume"}, 200)
	a.act(map[string]any{"type": "ready"}, 200)
	b.act(map[string]any{"type": "ready"}, 200)
	battle := a.room()
	if data(battle)["phase"] != "battle" {
		t.Fatal("all ready did not start")
	}
	a.act(map[string]any{"type": "build", "pad": 2, "tower": "arrow", "epoch": data(saved)["epoch"]}, 400)
	first := data(battle)["tick"].(float64)
	wait(t, "server advances without player actions", func() bool { return data(a.room())["tick"].(float64) > first })
	if a.room()["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
		t.Fatal("system tick enabled autoplay")
	}
	tb.record("real system timer, no browser driving simulation")
}

func TestStarfortOutcomesArchiveHTTP(t *testing.T) {
	for _, scenario := range []string{"victory-ready", "defeat-ready"} {
		for _, n := range []int{1, 4} {
			t.Run(fmt.Sprintf("%s-%d", scenario, n), func(t *testing.T) {
				tb := newHost(t, n, scenarioModule())
				tb.start(map[string]any{"scenario": scenario})
				a := tb.people[0]
				wait(t, "timer resolves terminal battle", func() bool { return a.room()["status"] == "finished" })
				r := a.room()
				result := r["result"].(map[string]any)
				win := scenario == "victory-ready"
				if result["rated"] != false || result["cooperative"] != true {
					t.Fatal("cooperative scoring", result)
				}
				winners := r["game"].(map[string]any)["winners"].([]any)
				if (win && len(winners) != n) || (!win && len(winners) != 0) {
					t.Fatal("incorrect shared outcome", winners)
				}
				for _, v := range result["players"].([]any) {
					p := v.(map[string]any)
					if p["won"] != win || p["ratingDelta"] != nil {
						t.Fatal("incorrect record", p)
					}
				}
				tb.record("terminal real-timer outcome from explicit final-battle fixture")
				tb.restart()
				a.command("rematch", nil, 200)
				if a.room()["status"] != "waiting" || a.room()["game"] != nil {
					t.Fatal("rematch retained battle")
				}
				for _, c := range tb.people {
					uid := c.state()["user"].(map[string]any)["id"].(string)
					profile := c.request("GET", "/api/players/"+uid, nil, 200)
					stats := profile["stats"].(map[string]any)["starfort"].(map[string]any)
					wins := float64(0)
					if win {
						wins = 1
					}
					if stats["played"] != float64(1) || stats["wins"] != wins {
						t.Fatal("incorrect profile history", stats)
					}
					rating := profile["rating"].(map[string]any)
					if rating["points"] != float64(1000) || rating["played"] != float64(0) {
						t.Fatal("coop changed rating", rating)
					}
					tb.trace = append(tb.trace, map[string]any{"profile": profile})
				}
			})
		}
	}
}

// A system timer must not continually push BotAt into the future. During an
// active real-time battle, autoplay has to invest even while ticks keep firing.
func TestStarfortBattleAutoplayHTTP(t *testing.T) {
	tb := newHost(t, 1, scenarioModule())
	tb.start(map[string]any{"scenario": "battle"})
	a := tb.people[0]
	// Join an already-ticking battle. The first pre-timer scheduling gap can
	// otherwise hide starvation by allowing exactly one bot action.
	wait(t, "battle timer has begun", func() bool { return data(a.room())["tick"].(float64) >= 5 })
	original := len(data(a.room())["towers"].([]any))
	a.command("autoplay", map[string]any{"enabled": true}, 200)
	wait(t, "autoplay builds during battle despite recurring system timers", func() bool {
		r := a.room()
		return data(r)["phase"] == "battle" && len(data(r)["towers"].([]any)) > original
	})
	tb.record("autoplay invested while system battle ticks continue")
	a.command("autoplay", map[string]any{"enabled": false}, 200)
	if a.room()["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
		t.Fatal("unable to reclaim realtime seat")
	}
}

func TestStarfortCountdownAndBattleRestartHTTP(t *testing.T) {
	m := starfort.Module()
	start := m.Start
	// The shorter preparation clock belongs only to this test wrapper. All
	// transitions and subsequent one-second simulation ticks are production code.
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		match, err := start(ctx, raw)
		if err != nil {
			return match, err
		}
		var s starfort.State
		if err = json.Unmarshal(match.Data, &s); err != nil {
			return match, err
		}
		s.NextAt = ctx.Now.Add(time.Second).UnixMilli()
		return s.Envelope()
	}
	tb := newHost(t, 1, m)
	tb.start(nil)
	a := tb.people[0]
	wait(t, "preparation deadline automatically starts wave", func() bool { return data(a.room())["phase"] == "battle" })
	wait(t, "battle advances", func() bool { return data(a.room())["tick"].(float64) > 0 })
	before := data(a.room())["tick"].(float64)
	tb.restart()
	wait(t, "persisted battle timer advances after process restart", func() bool { return data(a.room())["tick"].(float64) > before })
	tb.record("short test preparation clock then real running battle restart")
}

func TestStarfortPausedAutoplayIsIdleHTTP(t *testing.T) {
	tb := newHost(t, 2, starfort.Module())
	tb.start(nil)
	a, b := tb.people[0], tb.people[1]
	a.act(map[string]any{"type": "pause"}, 200)
	b.act(map[string]any{"type": "pause"}, 200)
	a.command("autoplay", map[string]any{"enabled": true}, 200)
	before := a.room()
	time.Sleep(2300 * time.Millisecond)
	after := a.room()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("paused autoplay persisted idle actions")
	}
	tb.record("paused autoplay produces no room updates")
	b.act(map[string]any{"type": "resume"}, 200)
	wait(t, "autoplay responds to teammate resume vote", func() bool { return data(b.room())["phase"] != "paused" })
	tb.record("autoplay resumed only after teammate vote")
}
