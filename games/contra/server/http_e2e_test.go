package contra

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	hostserver "github.com/k0ngk0ng/wire-board/internal/server"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func httpView(t *testing.T, c *gametest.Client) View {
	t.Helper()
	raw, _ := json.Marshal(gametest.Data(c.Room()))
	var v View
	if e := json.Unmarshal(raw, &v); e != nil {
		t.Fatal(e)
	}
	return v
}
func awaitHTTP(t *testing.T, c *gametest.Client, p func(View) bool) View {
	t.Helper()
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) {
		v := httpView(t, c)
		if p(v) {
			return v
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for authority")
	return View{}
}
func stream(t *testing.T, c *gametest.Client) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ws, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(c.Base, "http")+"/api/ws?stream=1", &websocket.DialOptions{HTTPClient: c.HTTP})
	if e != nil {
		t.Fatal(e)
	}
	ws.SetReadLimit(8 << 20)
	t.Cleanup(func() { ws.CloseNow() })
	return ws
}
func packet(t *testing.T, ws *websocket.Conn, kind string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for i := 0; i < 100; i++ {
		_, raw, e := ws.Read(ctx)
		if e != nil {
			t.Fatal(e)
		}
		var v map[string]any
		if e = json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		if v["type"] == kind {
			return v
		}
	}
	t.Fatal("missing stream packet", kind)
	return nil
}
func send(t *testing.T, ws *websocket.Conn, v any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, _ := json.Marshal(v)
	if e := ws.Write(ctx, websocket.MessageText, raw); e != nil {
		t.Fatal(e)
	}
}
func TestContraHTTPLoadingStreamAndSQLiteRestore(t *testing.T) {
	h := gametest.NewHTTP(t, 2, testModule(t), "contra")
	h.Config.ClientLoading = true
	// Restart the test host before a room exists; harness.Restart records a room.
	h.Server.Close()
	h.Host.Close()
	var err error
	h.Host, err = hostserver.New(h.Config, fstest.MapFS{"index.html": {Data: []byte("Contra HTTP acceptance")}})
	if err != nil {
		t.Fatal(err)
	}
	h.Server = httptest.NewServer(h.Host.Handler())
	for _, client := range append(append([]*gametest.Client{}, h.People...), h.Spectator) {
		client.Base = h.Server.URL
	}
	h.Start(map[string]any{})
	a, b := h.People[0], h.People[1]
	v := httpView(t, a)
	gate := a.Room()["loading"].(map[string]any)
	time.Sleep(160 * time.Millisecond)
	if httpView(t, a).Snapshot.Frame != 0 {
		t.Fatal("platform initial gate did not block")
	}
	for i, c := range h.People {
		fields := map[string]any{"loadToken": gate["token"], "loadBuild": gate["build"], "loadClient": fmt.Sprintf("contra-test-%d", i)}
		c.Command("load_begin", fields, 200)
		if i == 0 {
			c.Command("load_ready", fields, 200)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if httpView(t, a).Snapshot.Frame != 0 {
		t.Fatal("only one human unblocked")
	}
	b.Command("load_ready", map[string]any{"loadToken": gate["token"], "loadBuild": gate["build"], "loadClient": "contra-test-1"}, 200)
	v = awaitHTTP(t, a, func(v View) bool { return v.Snapshot.Frame > 0 })
	b.Command("autoplay", map[string]any{"enabled": true}, 200)
	v = awaitHTTP(t, a, func(v View) bool { return v.Inputs[1].Automated })
	ws := stream(t, a)
	state := packet(t, ws, "state")["state"].(map[string]any)
	room := state["room"].(map[string]any)
	var first map[string]any
	for n := 1; n <= 4; n++ {
		p := map[string]any{"type": "input", "roomId": room["id"], "version": room["version"], "serverInstance": state["serverInstance"], "nonce": fmt.Sprintf("contra-ws-%d", n), "action": map[string]any{"type": "input", "epoch": v.Epoch, "sequence": n, "held": n % 2}}
		if n == 1 {
			first = p
		}
		send(t, ws, p)
	}
	for n := 0; n < 4; n++ {
		if ack := packet(t, ws, "ack"); ack["status"] != float64(200) {
			t.Fatal("input rejected", ack)
		}
	}
	send(t, ws, first)
	if ack := packet(t, ws, "ack"); ack["status"] != float64(200) {
		t.Fatal("retry rejected", ack)
	}
	v = httpView(t, a)
	if v.Inputs[0].Sequence != 4 || v.Inputs[0].Held != 0 {
		t.Fatal("duplicate resurrected input")
	}
	sync := a.Request("GET", "/api/rooms/"+room["id"].(string)+"/sync", nil, 200)
	if sync == nil {
		t.Fatal("missing explicit checkpoint")
	}
	h.Record("all-human loading barrier, authenticated ordered WS input and nonce retry")
	ws.CloseNow()
	a.Action(map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 5, "held": 1}, 200)
	before := httpView(t, a)
	h.Restart()
	after := httpView(t, a)
	if after.Epoch != before.Epoch || after.Snapshot.Frame < before.Snapshot.Frame || after.Inputs[0].Held != 0 || !after.Inputs[1].Automated {
		t.Fatal("restore changed campaign or kept transport input")
	}
	ws = stream(t, a)
	next := packet(t, ws, "state")["state"].(map[string]any)
	if next["serverInstance"] == state["serverInstance"] {
		t.Fatal("instance did not rotate")
	}
	first["version"] = next["room"].(map[string]any)["version"]
	first["nonce"] = "contra-old-instance"
	first["action"] = map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 6, "held": 1}
	send(t, ws, first)
	if ack := packet(t, ws, "ack"); ack["status"] == float64(200) {
		t.Fatal("old server instance accepted")
	}
	watcher := stream(t, h.Spectator)
	packet(t, watcher, "state")
	first["serverInstance"] = next["serverInstance"]
	first["nonce"] = "contra-spectator"
	send(t, watcher, first)
	if ack := packet(t, watcher, "ack"); ack["status"] == float64(200) {
		t.Fatal("spectator controlled player")
	}
	b.Command("autoplay", map[string]any{"enabled": false}, 200)
	awaitHTTP(t, b, func(v View) bool { return !v.Inputs[1].Automated })
	h.Record("SQLite restore preserves campaign and autoplay; releases human controls; rejects old instance and spectator")
}

func TestContraHTTPPausePresentationRequiresManualResume(t *testing.T) {
	h := gametest.NewHTTP(t, 2, testModule(t), "contra")
	h.Start(map[string]any{})
	a, b := h.People[0], h.People[1]
	v := awaitHTTP(t, a, func(v View) bool { return v.Snapshot.Phase == "intro" })
	a.Action(map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 1, "held": int(engine.ButtonStart)}, 200)
	// Query the recovered button constant through the engine rather than assuming
	// a UI mapper can inject pause; B/A mappings are unrelated to Start authority.
	awaitHTTP(t, a, func(v View) bool { return v.Snapshot.Phase == "playing" })
	a.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": 2}, 200)
	time.Sleep(70 * time.Millisecond)
	a.Action(map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 3, "held": int(engine.ButtonStart)}, 200)
	v = awaitHTTP(t, a, func(v View) bool { return v.Snapshot.Paused })
	frame, revision, audio := v.Snapshot.Frame, v.Revision, v.Snapshot.AudioTick
	time.Sleep(180 * time.Millisecond)
	v = httpView(t, a)
	if !v.Snapshot.Paused || v.Snapshot.Frame != frame || v.Revision <= revision || v.Snapshot.AudioTick <= audio {
		t.Fatal("paused host did not separate presentation and rules")
	}
	a.Action(map[string]any{"type": "input", "epoch": v.Epoch, "sequence": 4, "held": int(engine.ButtonStart)}, 200)
	time.Sleep(70 * time.Millisecond)
	if !httpView(t, a).Snapshot.Paused {
		t.Fatal("held Start heartbeat resumed")
	}
	b.Action(map[string]any{"type": "resume", "epoch": v.Epoch, "sequence": 1}, 400)
	h.Record("real host paused timer advances presentation while rule frame remains fixed")
	h.Restart()
	v = httpView(t, a)
	if !v.Snapshot.Paused || v.Snapshot.Frame != frame {
		t.Fatal("restart resumed paused game")
	}
	a.Action(map[string]any{"type": "resume", "epoch": v.Epoch, "sequence": 5}, 200)
	v = awaitHTTP(t, a, func(v View) bool { return !v.Snapshot.Paused && v.Snapshot.Frame > frame })
	if v.Epoch == "" {
		t.Fatal("lost match identity")
	}
	h.Record("SQLite retains pause; only explicit P1 Apply resumes rules")
}

// A trusted test-only Start wrapper places the standard Session immediately
// before the original transition boundary. This proves module -> Finalize ->
// platform archive/ledger atomicity, not a naturally cleared eight-stage run.
func TestContraHTTPBoundaryCooperativeLedger(t *testing.T) {
	for _, players := range []int{1, 2} {
		for _, win := range []bool{true, false} {
			t.Run(fmt.Sprintf("players-%d-win-%t", players, win), func(t *testing.T) {
				m := testModule(t)
				start := m.Start
				m.Start = func(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
					g, e := start(c, raw)
					if e != nil {
						return g, e
					}
					g = boundary(t, g, 7)
					if !win {
						s := saved(t, g)
						s.Session.World.Active.Combat.World.BossDefeated = 0
						s.Session.World.Campaign.Continues = 0
						g.Data, e = json.Marshal(s)
					}
					return g, e
				}
				h := gametest.NewHTTP(t, players, m, "contra")
				h.Start(map[string]any{})
				a := h.People[0]
				v := awaitHTTP(t, a, func(v View) bool { return v.Snapshot.Finalized })
				if !v.Snapshot.Finalized || len(v.Frames) != 2 || v.Frames[1].Kind != "terminal" {
					t.Fatal("archive preceded terminal display")
				}
				delta, outcome := -10, "failed"
				if win {
					delta, outcome = 20, "cleared"
				}
				check := func() string {
					t.Helper()
					r := a.Room()
					raw, _ := json.Marshal(r["result"])
					var record hostserver.MatchRecord
					if e := json.Unmarshal(raw, &record); e != nil {
						t.Fatal(e)
					}
					if !record.Rated || !record.Cooperative || record.Outcome != outcome || record.RatingPolicy != gameapi.RatingCoopCampaignV1 || len(record.Players) != players {
						t.Fatal("invalid cooperative archive", string(raw))
					}
					for _, p := range record.Players {
						if p.RatingDelta != delta || p.Won != win || p.Rank != 1 {
							t.Fatal("unequal team settlement", p)
						}
					}
					db, e := sql.Open("sqlite", filepath.Join(h.Config.DataDir, "wire-board.db"))
					if e != nil {
						t.Fatal(e)
					}
					defer db.Close()
					var count, sum int
					if e = db.QueryRow("SELECT COUNT(*),COALESCE(SUM(delta),0) FROM rating_ledger WHERE match_id=?", record.ID).Scan(&count, &sum); e != nil {
						t.Fatal(e)
					}
					if count != players || sum != players*delta {
						t.Fatal("incorrect durable rating ledger", count, sum)
					}
					return record.ID
				}
				id := check()
				h.Record("controlled original terminal boundary finalizes before equal cooperative ledger entries")
				h.Restart()
				if check() != id {
					t.Fatal("restart created new match archive")
				}
				h.Record("SQLite restart preserves one archived outcome and one ledger entry per participant")
			})
		}
	}
}
