package pvz_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	pvz "github.com/k0ngk0ng/wire-board/games/pvz/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestPVZBackendDeadlineBoundaryAndNaturalOutcome(t *testing.T) {
	t.Run("boundary-inclusive-breach", func(t *testing.T) {
		s := backendCombatState(t)
		backendEngineAct(t, &s, 1, engine.Action{Type: "spawn", Kind: "zombie", Row: 0})
		s.Mowers[0].State = "spent"
		s.Mowers[0].X = 10300
		s.Zombies[0].X = 5
		s.PhaseEndTick = s.Tick + 1
		_ = engine.Step(&s, 1)
		if len(s.Rounds) != 1 || !s.Rounds[0].Breached {
			t.Fatal("last-tick house breach was discarded by timeout")
		}
	})
	t.Run("natural-one-team-attacks", func(t *testing.T) {
		b := backendNew(t, 2)
		for frames := 0; frames < 8200 && b.match.Result == nil; frames++ {
			v := b.view()
			if v.Players[1].Side == "zombies" {
				raw, e := b.m.BotAction(b.ctx(1), b.match)
				if e != nil {
					t.Fatal(e)
				}
				if len(raw) > 0 {
					b.match, e = b.m.Apply(b.ctx(1), b.match, raw)
					if e != nil {
						t.Fatal(e)
					}
				}
			}
			b.advance(50)
		}
		if b.match.Result == nil || b.match.Result.Draw || len(b.match.Result.Winners) != 1 || b.match.Result.Winners[0] != 1 {
			t.Fatalf("wrong asymmetric natural outcome: %+v", b.match.Result)
		}
		v := b.view()
		if !v.Rounds[0].Breached || v.Rounds[1].Breached {
			t.Fatalf("unexpected natural rounds: %+v", v.Rounds)
		}
		b.trace = append(b.trace, map[string]any{"kind": "one-attacking-team-natural-complete", "result": b.match.Result, "rounds": v.Rounds})
	})
}

// backendClock is deliberately local to test registration. It accelerates only
// callback time and maps SystemDeadline back to the real scheduler. No game
// resources, positions, bots, inputs, or outcomes are injected.
type backendClock struct {
	mu     sync.Mutex
	now    time.Time
	real   time.Time
	factor int
}

func (c *backendClock) with(ctx gameapi.Context, advance bool) (gameapi.Context, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	real := ctx.Now
	if c.now.IsZero() {
		c.now = real
		c.real = real
	}
	if advance {
		delta := real.Sub(c.real)
		if delta < 0 {
			delta = 0
		}
		c.now = c.now.Add(delta * time.Duration(c.factor))
		c.real = real
	}
	ctx.Now = c.now
	return ctx, real
}
func backendClockModule(original gameapi.Module, c *backendClock) gameapi.Module {
	m := original
	mapDeadline := func(match gameapi.Match, real, virtual time.Time) gameapi.Match {
		if match.SystemDeadline != 0 {
			match.SystemDeadline = real.UnixMilli() + max(int64(1), match.SystemDeadline-virtual.UnixMilli())
		}
		return match
	}
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		ctx, real := c.with(ctx, false)
		match, e := original.Start(ctx, raw)
		return mapDeadline(match, real, ctx.Now), e
	}
	m.Apply = func(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		ctx, real := c.with(ctx, false)
		next, e := original.Apply(ctx, match, raw)
		return mapDeadline(next, real, ctx.Now), e
	}
	m.OnTimer = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		ctx, real := c.with(ctx, true)
		next, e := original.OnTimer(ctx, match)
		return mapDeadline(next, real, ctx.Now), e
	}
	m.BotAction = func(ctx gameapi.Context, match gameapi.Match) (json.RawMessage, error) {
		ctx, _ = c.with(ctx, false)
		return original.BotAction(ctx, match)
	}
	m.Reclaim = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		ctx, real := c.with(ctx, false)
		next, e := original.Reclaim(ctx, match)
		return mapDeadline(next, real, ctx.Now), e
	}
	m.Restore = func(ctx gameapi.Context, match gameapi.Match) (gameapi.Match, error) {
		ctx, real := c.with(ctx, false)
		next, e := original.Restore(ctx, match)
		return mapDeadline(next, real, ctx.Now), e
	}
	return m
}

type backendHTTPRun struct {
	t      *testing.T
	host   *server.Server
	http   *httptest.Server
	client *integrationClient
	trace  []any
	clock  *backendClock
}

func backendNewHTTP(t *testing.T, n int) *backendHTTPRun {
	t.Helper()
	b := &backendHTTPRun{t: t, clock: &backendClock{factor: 20}}
	var e error
	b.host, e = server.New(server.Config{DataDir: t.TempDir(), InviteCode: "pvz-clock", AdminUsername: "BackendRoot", AdminPassword: "pvz-backend-test-pass", Modules: []gameapi.Module{backendClockModule(pvz.Module(), b.clock)}}, fstest.MapFS{"index.html": {Data: []byte("PvZ accelerated production HTTP acceptance")}})
	if e != nil {
		t.Fatal(e)
	}
	b.http = httptest.NewServer(b.host.Handler())
	jar, _ := cookiejar.New(nil)
	b.client = &integrationClient{t: t, base: b.http.URL, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
	b.client.request("POST", "/api/login", map[string]any{"name": "BackendRoot", "password": "pvz-backend-test-pass"}, 200)
	mode := "duel"
	if n == 4 {
		mode = "team"
	}
	b.client.request("POST", "/api/rooms", map[string]any{"kind": "pvz", "name": "PvZ生产电脑整局", "capacity": n, "options": map[string]any{"mode": mode, "roundSeconds": 180, "prepSeconds": 10}}, 201)
	for i := 1; i < n; i++ {
		b.client.command("add_bot", nil, 200)
	}
	b.client.command("ready", nil, 200)
	b.client.command("start", nil, 200)
	t.Cleanup(func() {
		b.http.Close()
		b.host.Close()
		out := filepath.Join("..", "..", "..", ".local", "pvz", "backend-http")
		if e := os.MkdirAll(out, 0755); e != nil {
			t.Error(e)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "players": n, "clockFactor": b.clock.factor, "scope": "real HTTP sessions, SQLite, production module strategies and rules; only callback clock is accelerated; no injected money/entities/results", "trace": b.trace}, "", "  ")
		if e := os.WriteFile(filepath.Join(out, fmt.Sprintf("%s.json", filepath.Base(t.Name()))), raw, 0600); e != nil {
			t.Error(e)
		}
	})
	return b
}
func TestPVZBackendBotsAndManualControlHTTP(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprintf("players-%d", n), func(t *testing.T) {
			b := backendNewHTTP(t, n)
			c := b.client
			initial := c.room()
			b.trace = append(b.trace, map[string]any{"kind": "natural-start", "room": initial})
			startSeq := integrationData(initial)["players"].([]any)[0].(map[string]any)["seq"]
			time.Sleep(600 * time.Millisecond)
			idle := c.room()
			seat := idle["seats"].([]any)[0].(map[string]any)
			if seat["autoPlay"] == true || seat["timeoutAutoPlay"] == true || integrationData(idle)["players"].([]any)[0].(map[string]any)["seq"] != startSeq {
				t.Fatal("idle human was automatically controlled")
			}
			b.trace = append(b.trace, map[string]any{"kind": "idle-human-still-manual", "room": idle})
			c.command("autoplay", map[string]any{"enabled": true}, 200)
			time.Sleep(500 * time.Millisecond)
			controlled := c.room()
			if controlled["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
				t.Fatal("explicit autoplay not enabled")
			}
			c.command("autoplay", map[string]any{"enabled": false}, 200)
			manual := c.room()
			seq := integrationData(manual)["players"].([]any)[0].(map[string]any)["seq"]
			time.Sleep(250 * time.Millisecond)
			stillManual := c.room()
			if stillManual["seats"].([]any)[0].(map[string]any)["autoPlay"] == true || integrationData(stillManual)["players"].([]any)[0].(map[string]any)["seq"] != seq {
				t.Fatal("cancelled autoplay continued buying")
			}
			b.trace = append(b.trace, map[string]any{"kind": "manual-takeover-preserved-seat", "room": stillManual})
			c.command("autoplay", map[string]any{"enabled": true}, 200)
			rounds := map[float64]bool{}
			until := time.Now().Add(50 * time.Second)
			var final map[string]any
			for time.Now().Before(until) {
				r := c.room()
				d := integrationData(r)
				rounds[d["round"].(float64)] = true
				if r["game"].(map[string]any)["finished"] == true {
					final = r
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if final == nil {
				t.Fatal("production computer match did not finish under clock scope")
			}
			d := integrationData(final)
			if len(d["rounds"].([]any)) != 2 || len(rounds) != 2 || d["phase"] != "finished" {
				t.Fatalf("incomplete double-round match: %v", d["rounds"])
			}
			b.trace = append(b.trace, map[string]any{"kind": "natural-two-round-finish", "room": final})
		})
	}
}
