package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Explicit exhausted-bank midgames. Normal complete-game coverage lives in
// catan_rivers_full_test.go; these cases exercise HTTP clocks and persistence.
func TestCatanRiversHTTPGoldLedger(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
				state, err := game.NewCatanRivers(n, game.CatanOptions{FiveSix: n > 4})
				if err != nil {
					t.Fatal(err)
				}
				state.Turn, state.Catan.StartPlayer, state.Phase = 0, 0, "catan_turn"
				g := state.Catan
				g.SetupStep, g.TurnSerial, g.Robber = g.SetupLimit(), 1, g.Rivers.Map.Swamps[0]
				if g.Paired != nil {
					g.Paired.Primary, g.Paired.Secondary = 0, 3
				}
				g.Rivers.Gold[1], g.Rivers.Bank = g.Rivers.Bank, 0
				g.Players[0].Resources[0], g.Bank[0] = 8, g.Bank[0]-8
				deadline := time.Now().Add(45 * time.Second).UnixMilli()
				s.mu.Lock()
				r := s.rooms[id]
				r.Game, r.TurnDeadline = state, deadline
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				sale := game.Action{Type: "catan_coin_sell", Color: 0}
				before, _ := json.Marshal(r)
				for _, seat := range []int{1, n} {
					clients[seat].command(current(clients[seat]), "action", sale, 400)
				}
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("unauthorized sale mutated room")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				r = s.rooms[id]
				request := map[string]any{"type": "action", "version": r.Version, "nonce": "river-ledger-sale", "action": sale}
				switch mode {
				case "manual":
					clients[0].post("/api/rooms/"+id, request, 200)
				case "autoplay":
					setAutoPlay(clients[0], current(clients[0]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					setAutoPlay(clients[0], current(clients[0]), false, 200)
				case "timeout":
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(deadline))
					s.mu.Unlock()
				}
				r = s.rooms[id]
				coins := r.Game.Catan.Rivers
				if coins.GoldIssued != 1 || coins.Bank != 0 || coins.Gold[0] != 1 || r.Game.Catan.Players[0].Resources[0] != 4 {
					t.Fatal("empty supply blocked sale", mode, coins, r.Game.Phase)
				}
				if mode != "timeout" && r.TurnDeadline != deadline {
					t.Fatal("ordinary sale refreshed clock")
				}
				if mode == "timeout" {
					setAutoPlay(clients[0], current(clients[0]), false, 200)
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if mode == "manual" {
					before, _ = json.Marshal(s.rooms[id])
					clients[0].post("/api/rooms/"+id, request, 200)
					after, _ = json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("replay issued gold twice")
					}
				}
				for seat, c := range clients {
					view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					public := view["rivers"].(map[string]any)
					if public["goldRule"] != "ledger" || public["goldIssued"] != float64(1) {
						t.Fatal("missing rule/issued amount")
					}
					for p, raw := range view["players"].([]any) {
						_, visible := raw.(map[string]any)["resources"]
						if visible != (seat == p) {
							t.Fatal("private hand exposed")
						}
					}
				}
				clients[0].command(current(clients[0]), "action", sale, 200)
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_coin_buy", Color: 0}, 200)
				coins = s.rooms[id].Game.Catan.Rivers
				if coins.GoldIssued != 2 || coins.Bank != 2 || coins.Gold[0] != 0 || coins.Bought != 1 {
					t.Fatal("payment not returned to bank")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_end"}, 200)
				if s.rooms[id].Game.Catan.Rivers.Bought != 0 {
					t.Fatal("purchase cap not reset on handoff")
				}
			})
		}
	}
}
