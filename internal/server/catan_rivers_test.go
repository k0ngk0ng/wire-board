package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func restartRiversHTTP(t *testing.T, s *Server, ts *httptest.Server, clients []*testClient, id string) (*Server, *httptest.Server) {
	t.Helper()
	before, _ := json.Marshal(s.rooms[id])
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	stopBotTicker(next)
	t.Cleanup(func() { next.Close() })
	nextTS := httptest.NewServer(next.Handler())
	t.Cleanup(nextTS.Close)
	for _, c := range clients {
		c.base = nextTS.URL
	}
	after, _ := json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rivers room changed on restart")
	}
	return next, nextTS
}

func TestCatanRiversHTTPStartClockAndRestart(t *testing.T) {
	for _, n := range []int{3, 5, 6} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
				state, err := game.NewCatanRivers(n, game.CatanOptions{FiveSix: n > 4})
				if err != nil {
					t.Fatal(err)
				}
				state.Turn, state.Catan.StartPlayer = 0, 0
				if state.Catan.Paired != nil {
					state.Catan.Paired.Primary, state.Catan.Paired.Secondary = 0, 3
				}
				now := time.Now()
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = now.Add(45 * time.Second).UnixMilli()
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				before, _ := json.Marshal(r)
				choice := state.Catan.Rivers.Map.Swamps[1]
				for _, p := range []int{1, n} {
					clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_rivers_start", Tile: choice}, 400)
				}
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_rivers_start", Tile: 0}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid start changed room")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				at := time.Now()
				switch mode {
				case "manual":
					clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_rivers_start", Tile: choice}, 200)
				case "autoplay":
					setAutoPlay(clients[0], current(clients[0]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(at)
					s.mu.Unlock()
					setAutoPlay(clients[0], current(clients[0]), false, 200)
				case "timeout":
					at = time.UnixMilli(s.rooms[id].TurnDeadline)
					s.mu.Lock()
					s.expireSetups(at)
					s.mu.Unlock()
				}
				r = s.rooms[id]
				left := r.TurnDeadline - at.UnixMilli()
				if r.Game.Phase != "catan_setup_settlement" || r.Game.Catan.SetupStep != 0 || r.Game.Turn != 0 || r.Game.CatanPendingActor() != -1 || left < 120000 || left > 121000 || !slices.Contains(r.Game.Catan.Rivers.Map.Swamps, r.Game.Catan.Robber) {
					t.Fatal("start transition", mode, left, r.Game.Phase)
				}
				if mode == "manual" && r.Game.Catan.Robber != choice {
					t.Fatal("manual swamp ignored")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_rivers_start", Tile: choice}, 400)
			})
		}
	}
}

func TestCatanRiversHTTPBridgeCoinsTradeAndRestore(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
			state, err := game.NewCatanRivers(n, game.CatanOptions{FiveSix: n > 4})
			if err != nil {
				t.Fatal(err)
			}
			state.Turn, state.Catan.StartPlayer, state.Phase = 0, 0, "catan_turn"
			g := state.Catan
			g.SetupStep, g.TurnSerial, g.Robber = g.SetupLimit(), 1, g.Rivers.Map.Swamps[0]
			if g.Paired != nil {
				g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
			}
			bridge := g.Rivers.Map.Bridges[0]
			v := g.Edges[bridge].A
			g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
			g.Players[0].Resources[0], g.Players[0].Resources[1] = 1, 2
			g.Bank[0]--
			g.Bank[1] -= 2
			g.Players[1].Resources[0] = 2
			g.Bank[0] -= 2
			g.Rivers.Gold[0] = 10
			g.Rivers.Bank -= 10
			// Exhausted physical supply must not block bridge rewards.
			g.Rivers.Gold[2], g.Rivers.Bank = g.Rivers.Bank, 0
			deadline := time.Now().Add(45 * time.Second).UnixMilli()
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = state
			r.TurnDeadline = deadline
			if err = s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			for _, a := range []game.Action{{Type: "catan_bridge", Edge: bridge}, {Type: "catan_coin_buy", Color: 0}, {Type: "catan_coin_buy", Color: 4}} {
				before, _ := json.Marshal(s.rooms[id])
				for _, p := range []int{1, n} {
					clients[p].command(current(clients[p]), "action", a, 400)
				}
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("unauthorized action changed room")
				}
				clients[0].command(current(clients[0]), "action", a, 200)
				if s.rooms[id].TurnDeadline != deadline {
					t.Fatal("ordinary action refreshed clock")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
			}
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_coin_buy", Color: 1}, 400)
			g = s.rooms[id].Game.Catan
			if g.Rivers.GoldIssued != 3 || g.Rivers.Bought != 2 || g.Rivers.Gold[0] != 9 || !g.Edges[bridge].Bridge || !slices.Equal(g.Players[0].Resources, []int{1, 0, 0, 0, 1}) {
				t.Fatal("bridge payment/coin restoration")
			}
			for p, c := range clients {
				view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
				public := view["rivers"].(map[string]any)
				if int(public["gold"].([]any)[0].(float64)) != 9 {
					t.Fatal("public coins")
				}
				for seat, raw := range view["players"].([]any) {
					_, visible := raw.(map[string]any)["resources"]
					if visible != (seat == p) {
						t.Fatal("resource privacy")
					}
				}
				if p != 0 {
					if raw := view["legal"].(map[string]any)["bridges"]; raw != nil && len(raw.([]any)) != 0 {
						t.Fatal("spectator/non-actor bridge controls")
					}
				}
			}
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_trade_offer", Give: make([]int, 5), Take: []int{2, 0, 0, 0, 0}, GoldGive: 3}, 200)
			offer := s.rooms[id].Game.Catan.Trade.ID
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			clients[1].command(current(clients[1]), "action", game.Action{Type: "catan_trade_accept", Offer: offer}, 200)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_trade_complete", Offer: offer, Target: 1}, 200)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			g = s.rooms[id].Game.Catan
			if g.Rivers.Gold[0] != 6 || g.Rivers.Gold[1] != 3 || g.Players[0].Resources[0] != 3 || g.Players[1].Resources[0] != 0 || s.rooms[id].TurnDeadline != deadline {
				t.Fatal("coin trade settlement")
			}
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_trade_complete", Offer: offer, Target: 1}, 400)
			if n > 4 {
				at := time.Now()
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_end"}, 200)
				r = s.rooms[id]
				left := r.TurnDeadline - at.UnixMilli()
				if r.Game.Turn != 3 || r.Game.Phase != "catan_turn" || !r.Game.Catan.Paired.Second || r.Game.Catan.Rivers.Bought != 0 || left < 120000 || left > 121000 {
					t.Fatal("secondary cap/clock")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
			}
		})
	}
}
