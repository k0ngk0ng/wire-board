package monopoly_test

// Explicit opt-in fixtures for accept_ui.py. Fixture selection exists only in
// this test binary; the production module never accepts the scenario option.
import (
	"encoding/json"
	"errors"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestMonopolyBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_MONOPOLY_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local", "monopoly", "acceptance", "browser")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	m := monopoly.Module()
	configure, start := m.Configure, m.Start
	split := func(raw json.RawMessage) (string, json.RawMessage, error) {
		v := map[string]json.RawMessage{}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &v); err != nil {
				return "", nil, err
			}
		}
		scenario := "natural"
		if value, ok := v["scenario"]; ok {
			if err := json.Unmarshal(value, &scenario); err != nil {
				return "", nil, err
			}
			delete(v, "scenario")
		}
		if scenario != "natural" && scenario != "management" && scenario != "auction" && scenario != "debt" && scenario != "bankrupt" && scenario != "finish" && scenario != "full-bots" {
			return "", nil, errors.New("unknown test scenario")
		}
		body, err := json.Marshal(v)
		return scenario, body, err
	}
	m.Configure = func(n int, raw json.RawMessage) (json.RawMessage, error) {
		name, body, err := split(raw)
		if err != nil {
			return nil, err
		}
		value, err := configure(n, body)
		if err != nil {
			return nil, err
		}
		var options map[string]any
		_ = json.Unmarshal(value, &options)
		options["scenario"] = name
		return json.Marshal(options)
	}
	m.Start = func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		name, body, err := split(raw)
		if err != nil {
			return gameapi.Match{}, err
		}
		match, err := start(ctx, body)
		if err != nil {
			return match, err
		}
		var s monopoly.State
		if err = json.Unmarshal(match.Data, &s); err != nil {
			return match, err
		}
		s.Turn = 0
		s.Phase = "roll"
		s.NextPhase = "end"
		s.Random = monopoly.Random{}
		s.Log = []string{"浏览器验收：" + name}
		s.Deadline = ctx.Now.Add(300 * time.Second).UnixMilli()
		s.Options.TurnSeconds = 300
		s.Options.AuctionSeconds = 30
		switch name {
		case "full-bots":
			s.Random.Key[31] = 19
			rng := rand.New(rand.NewSource(402))
			for d := range s.Decks {
				for i := range s.Decks[d] {
					s.Decks[d][i] = i
				}
				rng.Shuffle(16, func(i, j int) { s.Decks[d][i], s.Decks[d][j] = s.Decks[d][j], s.Decks[d][i] })
			}
		case "finish":
			s.Phase = "debt"
			s.Players[0].Cash = 0
			s.Players[2].Cash = 0
			s.Players[2].Bankrupt = true
			s.Eliminated = []int{2}
			s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 1, Amount: 550, Reason: "租金"}
		case "management":
			s.Phase = "end"
			group(&s, 0, []int{6, 8, 9}, []int{0, 0, 0})
			s.Properties[5].Owner = 0
			s.Properties[37].Owner = 1
			s.Properties[37].Mortgaged = true
		case "auction":
			s.Phase = "purchase"
			s.Players[0].Position = 6
			if len(s.Players) > 2 {
				s.Players[2].Cash = 0
			}
		case "debt":
			s.Phase = "debt"
			s.Players[0].Cash = 10
			s.Properties[5].Owner = 0
			s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 1, Amount: 75, Reason: "租金"}
		case "bankrupt":
			s.Phase = "debt"
			s.Players[0].Cash = 0
			s.Properties[5].Owner = 0
			s.Properties[5].Mortgaged = true
			s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 1, Amount: 550, Reason: "租金"}
		}
		s.BankNet = 0
		for _, p := range s.Players {
			s.BankNet -= p.Cash
		}
		return s.Envelope()
	}
	s, err := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "monopoly-browser", AdminUsername: "上架管理员", AdminPassword: "monopoly-browser-test", AssetsBaseURL: os.Getenv("WIRE_BOARD_MONOPOLY_ASSETS"), Modules: []gameapi.Module{m}}, os.DirFS(filepath.Join(root, "web", "dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	httpServer := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer httpServer.Close()
	go httpServer.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for begin := time.Now(); time.Since(begin) < 15*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
