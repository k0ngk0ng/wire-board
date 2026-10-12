package monopoly_test

// Failure scenarios, written before implementation: unaffordable or duplicate
// purchase; missing mandatory auction; ineligible prompts; equal/late/stale bids;
// double-spending reserved bids; restart losing bids/debt/estate queues; treating
// illiquidity as bankruptcy; illegal mortgage or redemption; player/bank estates
// mixed up; transfer fees lost; exited identities/ranks lost; private RNG leaked.
// These are HTTP E2E with explicit mid-game fixtures, not full-game evidence.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
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
	t         *testing.T
	host      *server.Server
	http      *httptest.Server
	cfg       server.Config
	people    []*client
	spectator *client
	trace     []any
}

func (c *client) request(method, path string, body any, want int) map[string]any {
	c.t.Helper()
	status, value := c.requestStatus(method, path, body)
	if status != want {
		c.t.Fatalf("%s %s = %d, want %d: %v", method, path, status, want, value)
	}
	return value
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
	var value map[string]any
	if err = json.NewDecoder(res.Body).Decode(&value); err != nil {
		c.t.Fatal(err)
	}
	return res.StatusCode, value
}
func (c *client) state() map[string]any { return c.request("GET", "/api/state", nil, 200) }
func (c *client) room() map[string]any  { c.t.Helper(); return c.state()["room"].(map[string]any) }
func data(room map[string]any) map[string]any {
	return room["game"].(map[string]any)["data"].(map[string]any)
}
func (c *client) command(kind string, body map[string]any, want int) map[string]any {
	c.t.Helper()
	r := c.room()
	c.serial++
	request := map[string]any{"type": kind, "nonce": fmt.Sprintf("e2e-%012d", c.serial), "version": r["version"]}
	for k, v := range body {
		request[k] = v
	}
	return c.request("POST", "/api/rooms/"+r["id"].(string), request, want)
}
func (c *client) act(action map[string]any, want int) {
	c.t.Helper()
	if _, ok := action["window"]; !ok {
		action["window"] = data(c.room())["window"]
	}
	c.command("action", map[string]any{"action": action}, want)
}
func (tb *table) record(label string) {
	tb.trace = append(tb.trace, map[string]any{"label": label, "room": tb.people[1].room()})
}
func (tb *table) restart() {
	tb.t.Helper()
	saved := tb.people[1].room()
	tb.http.Close()
	tb.host.Close()
	s, err := server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Monopoly E2E")}})
	if err != nil {
		tb.t.Fatal(err)
	}
	tb.host = s
	tb.http = httptest.NewServer(s.Handler())
	for _, c := range append(tb.people, tb.spectator) {
		c.base = tb.http.URL
	}
	if !reflect.DeepEqual(saved, tb.people[1].room()) {
		tb.t.Fatal("restart changed public state")
	}
	tb.record("restarted")
}
func newTable(t *testing.T, n int, fixture func(*monopoly.State)) *table {
	return newTableWithModule(t, n, fixture, nil)
}
func newTableWithModule(t *testing.T, n int, fixture func(*monopoly.State), tune func(*gameapi.Module)) *table {
	t.Helper()
	scope := "HTTP mid-game fixture"
	if fixture == nil {
		scope = "HTTP natural start with a short test auction timer"
	}
	m := monopoly.Module()
	start := m.Start
	m.Start = func(ctx gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
		match, err := start(ctx, options)
		if err != nil {
			return match, err
		}
		var s monopoly.State
		if err = json.Unmarshal(match.Data, &s); err != nil {
			return match, err
		}
		s.Options.AuctionSeconds = 2 // Test-owned fixture, never accepted from production clients.
		if fixture != nil {
			s.Turn = 0
			s.Phase = "end"
			s.NextPhase = "end"
			s.Deadline = ctx.Now.Add(120 * time.Second).UnixMilli()
			fixture(&s)
		}
		s.BankNet = 0
		for _, p := range s.Players {
			s.BankNet -= p.Cash
		}
		return s.Envelope()
	}
	if tune != nil {
		tune(&m)
	}
	tb := &table{t: t, cfg: server.Config{DataDir: t.TempDir(), InviteCode: "monopoly-test", AdminUsername: "强手棋0", AdminPassword: "test-password-123", Modules: []gameapi.Module{m}}}
	var err error
	tb.host, err = server.New(tb.cfg, fstest.MapFS{"index.html": {Data: []byte("Monopoly E2E")}})
	if err != nil {
		t.Fatal(err)
	}
	tb.http = httptest.NewServer(tb.host.Handler())
	t.Cleanup(func() {
		tb.http.Close()
		tb.host.Close()
		out := filepath.Join("..", "..", "..", ".local", "monopoly", "acceptance", "economy")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": scope, "trace": tb.trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, strings.ReplaceAll(t.Name(), "/", "-")+".json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	for i := 0; i < n+1; i++ {
		jar, _ := cookiejar.New(nil)
		c := &client{t: t, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}, base: tb.http.URL}
		if i == 0 {
			c.request("POST", "/api/login", map[string]any{"name": "强手棋0", "password": "test-password-123"}, 200)
		} else {
			c.request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("强手棋%d", i), "password": "test-password-123", "invite": "monopoly-test"}, 200)
		}
		if i < n {
			tb.people = append(tb.people, c)
		} else {
			tb.spectator = c
		}
	}
	tb.people[0].request("POST", "/api/admin/games/monopoly", map[string]bool{"enabled": true}, 200)
	r := tb.people[0].request("POST", "/api/rooms", map[string]any{"kind": "monopoly", "name": "经济验收", "capacity": n}, 201)
	id := r["id"].(string)
	for _, c := range tb.people[1:] {
		c.serial++
		c.request("POST", "/api/rooms/"+id, map[string]any{"type": "join", "version": tb.people[0].room()["version"], "nonce": fmt.Sprintf("e2e-%012d", c.serial)}, 200)
	}
	for _, c := range tb.people {
		c.command("ready", nil, 200)
	}
	tb.people[0].command("start", nil, 200)
	tb.spectator.request("POST", "/api/rooms/"+id+"/watch", map[string]any{}, 200)
	tb.record("started: " + scope)
	return tb
}
func money(r map[string]any, p int) int {
	return int(data(r)["players"].([]any)[p].(map[string]any)["cash"].(float64))
}
func property(r map[string]any, p int) map[string]any {
	return data(r)["properties"].([]any)[p].(map[string]any)
}
func (tb *table) waitPhase(phase string) {
	tb.t.Helper()
	for until := time.Now().Add(6 * time.Second); time.Now().Before(until); {
		if data(tb.people[1].room())["phase"] == phase {
			tb.record("reached " + phase)
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	tb.t.Fatal("phase not reached", phase, data(tb.people[1].room()))
}

func TestMonopolyAuctionHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { s.Phase = "purchase"; s.Players[0].Position = 6 })
	a, b, c := tb.people[0], tb.people[1], tb.people[2]
	a.act(map[string]any{"type": "auction"}, 200)
	r := a.room()
	old := r
	window := data(r)["window"]
	if len(r["game"].(map[string]any)["attention"].([]any)) != 0 {
		t.Fatal("auction prompts entire table")
	}
	b.act(map[string]any{"type": "bid", "amount": 100}, 200)
	c.act(map[string]any{"type": "bid", "amount": 100}, 400)
	// Concurrent bids from the same auction may use the old room version.
	c.serial++
	body := map[string]any{"type": "action", "version": old["version"], "nonce": fmt.Sprintf("e2e-%012d", c.serial), "action": map[string]any{"type": "bid", "window": window, "amount": 150}}
	c.request("POST", "/api/rooms/"+r["id"].(string), body, 200)
	c.request("POST", "/api/rooms/"+r["id"].(string), body, 200)
	tb.restart()
	tb.waitPhase("end")
	r = a.room()
	if money(r, 2) != 1350 || property(r, 6)["owner"] != float64(2) {
		t.Fatal("auction settlement", r)
	}
	if money(r, 1) != 1500 {
		t.Fatal("losing bidder charged")
	}
	for _, seat := range r["seats"].([]any) {
		if seat.(map[string]any)["autoPlay"] == true {
			t.Fatal("auction enabled autoplay")
		}
	}
	b.act(map[string]any{"type": "bid", "amount": 160, "window": window}, 400)
	before := a.room()
	a.act(map[string]any{"type": "roll", "dice": []int{6, 6}}, 400)
	if !reflect.DeepEqual(before, a.room()) {
		t.Fatal("illegal action mutated state")
	}
	raw, _ := json.Marshal(tb.spectator.room())
	if bytes.Contains(raw, []byte("random")) || bytes.Contains(raw, []byte("seed")) || bytes.Contains(raw, []byte("decks")) {
		t.Fatal("private randomness leaked")
	}
}

func TestMonopolyPurchaseAndEmptyAuctionHTTP(t *testing.T) {
	for _, buy := range []bool{true, false} {
		t.Run(fmt.Sprint(buy), func(t *testing.T) {
			tb := newTable(t, 2, func(s *monopoly.State) { s.Phase = "purchase"; s.Players[0].Position = 9 })
			a := tb.people[0]
			if buy {
				a.act(map[string]any{"type": "buy"}, 200)
				a.act(map[string]any{"type": "buy"}, 400)
				if money(a.room(), 0) != 1380 || property(a.room(), 9)["owner"] != float64(0) {
					t.Fatal("purchase wrong")
				}
			} else {
				a.act(map[string]any{"type": "auction"}, 200)
				tb.waitPhase("end")
				if property(a.room(), 9)["owner"] != float64(-1) || money(a.room(), 0) != 1500 {
					t.Fatal("empty auction charged/assigned")
				}
			}
			tb.record("purchase or flow-through")
		})
	}
}

func TestMonopolyMortgageDebtHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Phase = "debt"
		s.Players[0].Cash = 10
		s.Properties[5].Owner = 0
		s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 1, Amount: 75, Reason: "租金"}
	})
	a := tb.people[0]
	a.act(map[string]any{"type": "bankrupt"}, 400) // Cash shortage is not insolvency.
	a.act(map[string]any{"type": "mortgage", "property": 6}, 400)
	tb.restart()
	a.act(map[string]any{"type": "mortgage", "property": 5}, 200)
	r := a.room()
	if money(r, 0) != 35 || money(r, 1) != 1575 || data(r)["phase"] != "end" || property(r, 5)["mortgaged"] != true {
		t.Fatal("debt did not settle atomically", r)
	}
	a.act(map[string]any{"type": "unmortgage", "property": 5}, 400)
	tb.record("funded debt settled")
}

func TestMonopolyBankruptcyToPlayerHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Phase = "debt"
		s.Players[0].Cash = 20
		s.Properties[5].Owner = 0
		s.Properties[5].Mortgaged = true
		s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 1, Amount: 550, Reason: "租金"}
	})
	a, b := tb.people[0], tb.people[1]
	a.act(map[string]any{"type": "bankrupt"}, 200)
	r := b.room()
	if data(r)["phase"] != "mortgage_transfer" || money(r, 1) != 1520 || property(r, 5)["owner"] != float64(1) {
		t.Fatal("estate not transferred", r)
	}
	tb.restart()
	b.act(map[string]any{"type": "accept_mortgage", "choice": "keep"}, 200)
	r = b.room()
	if money(r, 1) != 1510 || property(r, 5)["mortgaged"] != true {
		t.Fatal("missing transfer interest", r)
	}
	a.command("leave", nil, 200)
	a.request("POST", "/api/rooms", map[string]any{"kind": "monopoly", "name": "破产后另一局", "capacity": 2}, 201)
	if len(b.room()["seats"].([]any)) != 3 {
		t.Fatal("original roster lost")
	}
	tb.record("bankrupt departed")
}

func TestMonopolyBankruptcyToBankHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Phase = "debt"
		s.Players[0].Cash = 20
		s.Properties[5].Owner = 0
		s.Properties[5].Mortgaged = true
		s.Properties[6].Owner = 0
		s.Debt = &monopoly.Debt{Debtor: 0, Creditor: -1, Amount: 550, Reason: "税款"}
	})
	a, b := tb.people[0], tb.people[1]
	a.act(map[string]any{"type": "bankrupt"}, 200)
	r := b.room()
	if data(r)["phase"] != "auction" || property(r, 5)["mortgaged"] == true {
		t.Fatal("bank did not clear mortgage and auction", r)
	}
	b.act(map[string]any{"type": "bid", "amount": 100}, 200)
	tb.restart()
	// The second bank-owned property must also be auctioned, even if the first gets no further bids.
	for until := time.Now().Add(6 * time.Second); time.Now().Before(until); {
		r = b.room()
		if data(r)["phase"] == "auction" && data(r)["auction"].(map[string]any)["property"] == float64(6) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if data(r)["phase"] != "auction" || data(r)["auction"].(map[string]any)["property"] != float64(6) {
		t.Fatal("lost estate auction queue", r)
	}
	b.act(map[string]any{"type": "bid", "amount": 60}, 200)
	tb.waitPhase("roll")
	r = b.room()
	if money(r, 1) != 1340 || property(r, 5)["owner"] != float64(1) || property(r, 6)["owner"] != float64(1) {
		t.Fatal("estate auction settlement", r)
	}
}

func TestMonopolyFinalBankruptcyHTTP(t *testing.T) {
	tb := newTable(t, 2, func(s *monopoly.State) {
		s.Phase = "debt"
		s.Players[0].Cash = 20
		s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 1, Amount: 550, Reason: "租金"}
	})
	tb.people[0].act(map[string]any{"type": "bankrupt"}, 200)
	r := tb.people[1].room()
	if r["status"] != "finished" {
		t.Fatal("last survivor not declared", r)
	}
	players := r["result"].(map[string]any)["players"].([]any)
	if players[0].(map[string]any)["rank"] != float64(2) || players[1].(map[string]any)["rank"] != float64(1) {
		t.Fatal("wrong survival ranking", players)
	}
	tb.restart()
	tb.record("final result survived restart")
}

func TestMonopolyStartHTTP(t *testing.T) {
	for _, n := range []int{2, 4, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tb := newTable(t, n, nil)
			r := tb.people[0].room()
			view := data(r)
			if view["phase"] != "roll" || len(view["board"].([]any)) != 40 || len(view["players"].([]any)) != n {
				t.Fatal("bad initial game", view)
			}
			for p := range tb.people {
				if money(r, p) != 1500 || player(r, p)["position"] != float64(0) {
					t.Fatal("bad initial player", p)
				}
			}
			for i := 0; i < 40; i++ {
				if property(r, i)["owner"] != float64(-1) {
					t.Fatal("initial property already owned", i)
				}
			}
			if view["houses"] != float64(32) || view["hotels"] != float64(12) {
				t.Fatal("wrong building inventory")
			}
			tb.spectator.command("action", map[string]any{"action": map[string]any{"type": "roll", "window": view["window"]}}, 400)
			tb.restart()
			tb.record("natural start restored")
		})
	}
}

// Fractional 10% fees must not be truncated in the integer money ledger.
// Site convention: round an individual mortgage fee up to the next unit.
func TestMonopolyMortgageRoundingHTTP(t *testing.T) {
	for _, transfer := range []bool{false, true} {
		t.Run(fmt.Sprint(transfer), func(t *testing.T) {
			tb := newTable(t, 3, func(s *monopoly.State) {
				s.Properties[37].Owner = 0
				s.Properties[37].Mortgaged = true
				if transfer {
					s.Phase = "mortgage_transfer"
					s.Transfer = &monopoly.Transfer{Player: 0, Property: 37}
				}
			})
			a := tb.people[0]
			if transfer {
				a.act(map[string]any{"type": "accept_mortgage", "choice": "keep"}, 200)
				if money(a.room(), 0) != 1482 {
					t.Fatal("transfer fee not rounded up")
				}
			}
			a.act(map[string]any{"type": "unmortgage", "property": 37}, 200)
			want := 1307
			if transfer {
				want -= 18
			}
			if money(a.room(), 0) != want || property(a.room(), 37)["mortgaged"] == true {
				t.Fatal("redemption fee truncated", a.room())
			}
			tb.record("rounded mortgage fee")
		})
	}
}
