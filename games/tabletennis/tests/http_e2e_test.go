package tabletennis_test

// Real host, HTTP/WS authentication, actual clocks and SQLite. No game-state
// injection. Failure scenarios were recorded in CONTRACT.md before implementation.
import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	game "github.com/k0ngk0ng/wire-board/games/tabletennis/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"github.com/k0ngk0ng/wire-board/internal/server"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func httpView(t *testing.T, c *gametest.Client) game.View {
	t.Helper()
	var v game.View
	if e := json.Unmarshal(raw(gametest.Data(c.Room())), &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func awaitView(t *testing.T, c *gametest.Client, within time.Duration, predicate func(game.View) bool) game.View {
	t.Helper()
	until := time.Now().Add(within)
	var v game.View
	for time.Now().Before(until) {
		v = httpView(t, c)
		if predicate(v) {
			return v
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("state not reached: phase=%s score=%v shot=%d expected=%d", v.Phase, v.Score, v.Shot, v.Expected)
	return v
}
func stroke(v game.View) map[string]any {
	at := time.Now().UnixMilli()
	racketX := 0.
	if v.Phase == "rally" && v.Flight != nil {
		contact := at
		if contact < v.Flight.ContactStart {
			contact = v.Flight.ContactStart
		}
		segment := v.Flight.Segments[0]
		for _, candidate := range v.Flight.Segments {
			if candidate.At <= contact {
				segment = candidate
			} else {
				break
			}
		}
		seconds := float64(contact-segment.At) / 1000
		x := segment.P.X + segment.V.X*seconds + .5*segment.A.X*seconds*seconds
		dir := 1.
		if v.Options.Teams[v.Expected]^v.Ends == 1 {
			dir = -1
		}
		racketX = math.Max(-1, math.Min(1, x*dir/.9))
	}
	return map[string]any{"type": "swing", "epoch": v.Epoch, "rally": v.Rally, "shot": v.Shot, "seq": v.Controls.Seq + 1, "kind": "drive", "aim": 0, "power": .55, "racketX": racketX, "at": at}
}

func TestTableTennisHTTPAdmissionAndWebSocketIsolation(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "tabletennis")
	h.AssertPublished()
	a := h.People[0]
	for _, o := range []map[string]any{{"capacity": 3}, {"capacity": 2, "options": map[string]any{"teams": []int{0, 0}}}, {"capacity": 2, "options": map[string]any{"court": "unknown"}}} {
		o["kind"] = "tabletennis"
		o["name"] = "invalid"
		a.Request("POST", "/api/rooms", o, 400)
	}
	h.Start(map[string]any{})
	initial := httpView(t, a)
	if initial.Phase != "countdown" || initial.Options.Court != "club" {
		t.Fatal("invalid initial room")
	}
	if httpView(t, h.Spectator).Controls.CanSwing {
		t.Fatal("spectator can swing")
	}
	// Independent cancellation sequences may share an old room revision.
	version := a.Room()["version"]
	for _, c := range h.People {
		v := httpView(t, c)
		r := c.Room()
		c.Request("POST", "/api/rooms/"+r["id"].(string), map[string]any{"type": "action", "version": version, "nonce": fmt.Sprintf("cancel-%d", c.Index), "action": map[string]any{"type": "cancel", "epoch": v.Epoch, "rally": v.Rally, "shot": v.Shot, "seq": 1}}, 200)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	conn, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(a.Base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: a.HTTP})
	if e != nil {
		t.Fatal(e)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(2 << 20)
	state := a.State()
	r := a.Room()
	v := httpView(t, a)
	request := map[string]any{"type": "input", "serverInstance": state["serverInstance"], "roomId": r["id"], "version": r["version"], "nonce": "tt-stream-once", "action": map[string]any{"type": "cancel", "epoch": v.Epoch, "rally": v.Rally, "shot": v.Shot, "seq": 2}}
	for attempt := 0; attempt < 2; attempt++ {
		if e = conn.Write(ctx, websocket.MessageText, raw(request)); e != nil {
			t.Fatal(e)
		}
		for {
			_, b, err := conn.Read(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var msg map[string]any
			_ = json.Unmarshal(b, &msg)
			if msg["type"] == "ack" && msg["nonce"] == "tt-stream-once" {
				if msg["status"] != float64(200) {
					t.Fatal(msg)
				}
				break
			}
		}
	}
	if httpView(t, a).Controls.Seq != 2 {
		t.Fatal("WS duplicate changed sequence")
	}
	before := httpView(t, a)
	h.Restart()
	after := httpView(t, a)
	if before.Epoch != after.Epoch || before.Rally != after.Rally || before.Score != after.Score || after.Controls.Seq != 2 {
		t.Fatal("restore lost acknowledged input")
	}
	h.Record("default publication, admission, independent stale revisions, real WS duplicate and SQLite restore")
}
func TestTableTennisHTTPCompleteBotsAndManualHandoff(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			h := gametest.NewHTTP(t, 1, game.Module(), "tabletennis")
			host := h.People[0]
			host.Request("POST", "/api/rooms", map[string]any{"kind": "tabletennis", "name": "乒乓球完整比赛", "capacity": n, "options": map[string]any{"difficulty": "easy", "bestOf": 1}}, 201)
			for i := 1; i < n; i++ {
				host.Command("add_bot", nil, 200)
			}
			host.Command("ready", nil, 200)
			host.Command("start", nil, 200)
			id := host.Room()["id"].(string)
			h.Spectator.Request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
			v := awaitView(t, host, 8*time.Second, func(v game.View) bool {
				now := time.Now().UnixMilli()
				return v.Controls.CanSwing && (v.Phase == "serve" || (v.Flight != nil && now >= v.Controls.OpenAt+60 && now < v.Controls.CloseAt-100))
			})
			host.Action(stroke(v), 200)
			h.Record("human performed a real stroke against computer seats")
			host.Command("autoplay", map[string]any{"enabled": true}, 200)
			v = awaitView(t, host, 12*time.Second, func(v game.View) bool { return v.Rally > 1 || v.Shot > 4 })
			host.Command("autoplay", map[string]any{"enabled": false}, 200)
			after := httpView(t, host)
			if after.Epoch != v.Epoch || after.Rally < v.Rally || host.Room()["seats"].([]any)[0].(map[string]any)["autoPlay"] == true {
				t.Fatal("handoff changed identity or failed")
			}
			host.Command("autoplay", map[string]any{"enabled": true}, 200)
			before := httpView(t, host)
			h.Restart()
			restored := httpView(t, host)
			if restored.Epoch != before.Epoch || restored.Rally < before.Rally {
				t.Fatal("active restart reset match")
			}
			h.Record("autoplay, reclaim and real SQLite recovery preserved active match")
			until := time.Now().Add(8 * time.Minute)
			lastRally := 0
			var final game.View
			for time.Now().Before(until) {
				final = httpView(t, host)
				if final.Rally != lastRally {
					h.Record("natural point")
					lastRally = final.Rally
				}
				if final.Phase == "finished" {
					break
				}
				time.Sleep(250 * time.Millisecond)
			}
			if final.Phase != "finished" || len(final.Games) != 1 {
				t.Fatal("real natural match did not finish")
			}
			g := final.Games[0]
			if g.Score[g.Winner] < 11 || g.Score[g.Winner]-g.Score[1-g.Winner] < 2 {
				t.Fatal("invalid natural score")
			}
			if len(host.Room()["game"].(map[string]any)["winners"].([]any)) != n/2 {
				t.Fatal("incorrect winner seats")
			}
			if host.Room()["result"].(map[string]any)["rated"] != false {
				t.Fatal("computer match was rated")
			}
			h.Record("natural full match ended through ordinary computer/human inputs")
			h.Restart()
			if !reflect.DeepEqual(final, httpView(t, host)) {
				t.Fatal("finished state changed after recovery")
			}
		})
	}
}
func TestTableTennisBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_TABLETENNIS_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	root, e := filepath.Abs(filepath.Join("..", "..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local", "tabletennis", "browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	module := game.Module()
	accept := module.AcceptStale
	rejected, e := os.Create(filepath.Join(out, "rejected-inputs.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	defer rejected.Close()
	module.AcceptStale = func(ctx gameapi.Context, m gameapi.Match, action json.RawMessage) bool {
		ok := accept(ctx, m, action)
		if !ok {
			view, _ := module.View(m, ctx.Actor)
			_ = json.NewEncoder(rejected).Encode(map[string]any{"at": ctx.Now.UnixMilli(), "actor": ctx.Actor, "action": action, "view": view})
		}
		return ok
	}
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "tabletennis-browser", AdminUsername: "乒乓验收管理员", AdminPassword: "tabletennis-browser-test", ClientLoading: true, Modules: []gameapi.Module{module}}, os.DirFS(filepath.Join(root, "web", "dist")))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	hs := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer hs.Close()
	go hs.Serve(ln)
	_ = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+networkProxy(t, ln.Addr().String(), filepath.Join(out, "network-delay"))), 0600)
	for begin := time.Now(); time.Since(begin) < 30*time.Minute; {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}

func TestTableTennisConcurrentInputsAndRecovery(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "tabletennis")
	h.Start(map[string]any{})
	version := h.People[0].Room()["version"]
	type result struct {
		status int
		err    error
	}
	done := make(chan result, 2)
	for _, c := range h.People {
		v := httpView(t, c)
		r := c.Room()
		body := raw(map[string]any{"type": "action", "version": version, "nonce": fmt.Sprintf("tt-concurrent-%d", c.Index), "action": map[string]any{"type": "cancel", "epoch": v.Epoch, "rally": v.Rally, "shot": v.Shot, "seq": 1}})
		go func(c *gametest.Client, id string, body []byte) {
			req, e := http.NewRequest("POST", c.Base+"/api/rooms/"+id, strings.NewReader(string(body)))
			if e != nil {
				done <- result{err: e}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			response, e := c.HTTP.Do(req)
			if e != nil {
				done <- result{err: e}
				return
			}
			response.Body.Close()
			done <- result{status: response.StatusCode}
		}(c, r["id"].(string), body)
	}
	for i := 0; i < 2; i++ {
		r := <-done
		if r.err != nil || r.status != 200 {
			t.Fatalf("concurrent input failed: %+v", r)
		}
	}
	for _, c := range h.People {
		if httpView(t, c).Controls.Seq != 1 {
			t.Fatal("concurrent seat input was lost")
		}
	}
	v := httpView(t, h.Spectator)
	h.Spectator.Action(map[string]any{"type": "cancel", "epoch": v.Epoch, "rally": v.Rally, "shot": v.Shot, "seq": 1}, 400)
	h.Restart()
	for _, c := range h.People {
		if httpView(t, c).Controls.Seq != 1 {
			t.Fatal("durable concurrent input lost on restart")
		}
	}
	h.Record("concurrent stale revisions, observer rejection and durable recovery")
}

// Real-time ownership stays manual, matching the shared platform policy.
// The game-specific serve clock awards a fault; explicit autoplay still works.
func TestTableTennisHTTPServeTimeoutAndReclaim(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "tabletennis")
	h.Start(map[string]any{})
	v := awaitView(t, h.People[0], 5*time.Second, func(v game.View) bool { return v.Phase == "serve" })
	owner := h.People[v.Expected]
	epoch := v.Epoch
	team := v.Options.Teams[v.Expected]
	after := awaitView(t, owner, 19*time.Second, func(x game.View) bool { return x.Score[1-team] == 1 })
	if owner.Room()["seats"].([]any)[v.Expected].(map[string]any)["autoPlay"] == true {
		t.Fatal("serve clock silently transferred human control")
	}
	if after.Epoch != epoch || after.Score[team] != 0 {
		t.Fatal("serve fault changed identity or awarded wrong side")
	}
	h.Record("fifteen-second serve clock awarded one point without forced autoplay")
	owner.Command("autoplay", map[string]any{"enabled": true}, 200)
	awaitView(t, owner, 5*time.Second, func(x game.View) bool { return x.Phase == "serve_toss" || x.Phase == "rally" })
	owner.Command("autoplay", map[string]any{"enabled": false}, 200)
	if httpView(t, owner).Epoch != epoch || owner.Room()["seats"].([]any)[v.Expected].(map[string]any)["autoPlay"] == true {
		t.Fatal("explicit autoplay/reclaim reset or retained control")
	}
	h.Record("explicit platform autoplay and manual reclaim preserved match")
}

func TestTableTennisHTTPTwoHumanPartnersAndComputers(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "tabletennis")
	host, partner := h.People[0], h.People[1]
	created := host.Request("POST", "/api/rooms", map[string]any{"kind": "tabletennis", "name": "真人搭档对电脑", "capacity": 4, "options": map[string]any{"teams": []int{0, 0, 1, 1}, "difficulty": "easy", "bestOf": 1}}, 201)
	id := created["id"].(string)
	partner.Request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": host.Room()["version"], "nonce": "tt-human-partner-join"}, 200)
	host.Command("add_bot", nil, 200)
	host.Command("add_bot", nil, 200)
	host.Command("ready", nil, 200)
	partner.Command("ready", nil, 200)
	host.Command("start", nil, 200)
	h.Spectator.Request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
	for _, person := range h.People {
		v := awaitView(t, person, 25*time.Second, func(v game.View) bool {
			now := time.Now().UnixMilli()
			return v.Controls.CanSwing && (v.Phase == "serve" || (v.Flight != nil && now >= v.Controls.OpenAt+60 && now < v.Controls.CloseAt-100))
		})
		person.Action(stroke(v), 200)
		person.Command("autoplay", map[string]any{"enabled": true}, 200)
	}
	h.Record("both real teammates submitted ordinary strokes against two computer seats")
	finished := awaitView(t, host, 8*time.Minute, func(v game.View) bool { return v.Phase == "finished" })
	if len(finished.Games) != 1 || !reflect.DeepEqual(finished.Options.Teams, []int{0, 0, 1, 1}) {
		t.Fatal("cooperative team setup changed")
	}
	result := host.Room()["result"].(map[string]any)
	if result["rated"] != false {
		t.Fatal("computer game awarded competitive ratings")
	}
	winners := host.Room()["game"].(map[string]any)["winners"].([]any)
	if len(winners) != 2 {
		t.Fatal("team result lost a partner")
	}
	h.Record("natural doubles outcome, both winners retained and computer match unrated")
}
