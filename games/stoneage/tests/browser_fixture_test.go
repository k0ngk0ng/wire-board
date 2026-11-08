package stoneage_test

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	stoneage "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestStoneageBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_STONEAGE_BROWSER") != "1" {
		t.Skip("explicit browser acceptance")
	}
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(root, ".local/stoneage/browser")
	if e = os.MkdirAll(out, 0755); e != nil {
		t.Fatal(e)
	}
	module := stoneage.Module()
	if os.Getenv("WIRE_BOARD_STONEAGE_FORCE_COMBO") == "1" {
		start := module.Start
		module.Start = func(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
			m, err := start(c, options)
			if err != nil {
				return m, err
			}
			var state stoneage.State
			if err = json.Unmarshal(m.Data, &state); err != nil {
				return m, err
			}
			for i := range state.Pool.Pets {
				state.Pool.Pets[i].Loyalty = 100
			}
			m.Data, err = json.Marshal(state)
			return m, err
		}
		apply := module.Apply
		module.Apply = func(c gameapi.Context, m gameapi.Match, action json.RawMessage) (gameapi.Match, error) {
			var previous stoneage.State
			json.Unmarshal(m.Data, &previous)
			next, err := apply(c, m, action)
			if err != nil || previous.Phase != "arming" {
				return next, err
			}
			var state stoneage.State
			if err = json.Unmarshal(next.Data, &state); err != nil {
				return next, err
			}
			if state.Phase == "planning" {
				state.Random = 1 // ninth draw starts the two-member combination.
				for i := range state.Players {
					speed := 1000
					if i != 0 {
						speed = 10000
					}
					state.Players[i].Body.Speed, state.Players[i].Pet.Speed, state.Players[i].Ride.Speed = speed, speed, speed
				}
				next.Data, err = json.Marshal(state)
			}
			return next, err
		}
		bot := module.BotAction
		module.BotAction = func(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
			var state stoneage.State
			if err := json.Unmarshal(m.Data, &state); err != nil {
				return nil, err
			}
			if state.Phase == "planning" && c.Actor != 0 {
				return json.Marshal(stoneage.Action{Type: "submit", Session: state.Session, Window: state.Window, Plan: &stoneage.Plan{Player: stoneage.Command{Kind: "guard", Target: c.Actor * 2}, Pet: stoneage.Command{Kind: "guard", Target: c.Actor*2 + 1}}})
			}
			return bot(c, m)
		}
	}
	if os.Getenv("WIRE_BOARD_STONEAGE_FORCE_ESCAPE") == "1" {
		original := module.Start
		module.Start = func(c gameapi.Context, options json.RawMessage) (gameapi.Match, error) {
			m, err := original(c, options)
			if err != nil {
				return m, err
			}
			var state stoneage.State
			if err = json.Unmarshal(m.Data, &state); err != nil {
				return m, err
			}
			for i := range state.Pool.Pets {
				state.Pool.Pets[i].Loyalty = 0
			}
			state.Random = 1 // third SplitMix draw is >=60: first pet flees.
			m.Data, err = json.Marshal(state)
			return m, err
		}
		bot := module.BotAction
		module.BotAction = func(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
			var state stoneage.State
			if err := json.Unmarshal(m.Data, &state); err != nil {
				return nil, err
			}
			if state.Phase == "planning" && c.Actor == 1 {
				return json.Marshal(stoneage.Action{Type: "submit", Session: state.Session, Window: state.Window, Plan: &stoneage.Plan{Player: stoneage.Command{Kind: "guard", Target: 2}, Pet: stoneage.Command{Kind: "wait", Target: 3}}})
			}
			return bot(c, m)
		}
	}
	s, e := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "stoneage-browser", AdminUsername: "石器验收", AdminPassword: "stoneage-browser-only", Modules: []gameapi.Module{module}}, os.DirFS(filepath.Join(root, "web/dist")))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer srv.Close()
	go srv.Serve(l)
	if e = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+l.Addr().String()), 0600); e != nil {
		t.Fatal(e)
	}
	for start := time.Now(); time.Since(start) < 30*time.Minute; {
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			if e = os.Remove(filepath.Join(out, "stop")); e != nil {
				t.Fatal(e)
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("browser fixture timed out")
}
