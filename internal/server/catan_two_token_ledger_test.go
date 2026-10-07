package server

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Real opening followed by an explicit depleted-token/played-knight fixture.
// Actions, persistence, retries and autoplay all use the production server.
func TestCatanTwoHTTPTokenLedger(t *testing.T) {
	for _, scenario := range []string{"", "rivers", "caravans"} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(scenario+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := newTwoScenarioFullTable(t, scenario)
				state := s.rooms[id].Game
				for state.Catan.SetupStep < state.Catan.SetupLimit() {
					a, err := state.BotAction(state.Turn)
					if err != nil {
						t.Fatal(err)
					}
					if err = state.Apply(state.Turn, a); err != nil {
						t.Fatal(err)
					}
				}
				g, p := state.Catan, state.Turn
				q := g.Two
				q.Bank, q.Tokens[p], q.Tokens[1-p] = 0, 0, 20
				knight := slices.Index(g.DevDeck, 0)
				if knight < 0 {
					t.Fatal("missing knight card")
				}
				g.DevDeck = slices.Delete(g.DevDeck, knight, knight+1)
				g.DevDiscard = append(g.DevDiscard, 0)
				g.Players[p].Knights++
				// Prevent optional trades in follow-up observations; only the knight
				// exchange is driven automatically in this fixture.
				for seat := range g.Players {
					for c, n := range g.Players[seat].Resources {
						g.Bank[c] += n
						g.Players[seat].Resources[c] = 0
					}
				}
				deadline := time.Now().Add(45 * time.Second).UnixMilli()
				s.mu.Lock()
				r := s.rooms[id]
				r.TurnDeadline = deadline
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				before, _ := json.Marshal(r)
				action := game.Action{Type: "catan_two_knight"}
				for _, wrong := range []int{1 - p, 2} {
					clients[wrong].command(current(clients[wrong]), "action", action, 400)
				}
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("wrong seat mutated ledger")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				request := map[string]any{"type": "action", "version": s.rooms[id].Version, "nonce": "two-token-ledger-knight", "action": action}
				switch mode {
				case "manual":
					clients[p].post("/api/rooms/"+id, request, 200)
				case "autoplay":
					setAutoPlay(clients[p], current(clients[p]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					setAutoPlay(clients[p], current(clients[p]), false, 200)
				case "timeout":
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(deadline))
					s.mu.Unlock()
					setAutoPlay(clients[p], current(clients[p]), false, 200)
				}
				r = s.rooms[id]
				q = r.Game.Catan.Two
				if q.Bank != 0 || q.TokensIssued != 2 || q.Tokens[p] != 2 || !q.KnightExchanged || r.Game.Catan.Players[p].Knights != 0 || r.Game.Phase != "catan_roll" {
					t.Fatal("exchange failed", mode, q, r.Game.Phase)
				}
				if mode != "timeout" && r.TurnDeadline != deadline {
					t.Fatal("exchange renewed clock")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				before, _ = json.Marshal(s.rooms[id])
				if mode == "manual" {
					clients[p].post("/api/rooms/"+id, request, 200)
				}
				clients[p].command(current(clients[p]), "action", action, 400)
				after, _ = json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("repeat request paid twice")
				}
				assertTwoHTTPPrivacy(t, clients, s.rooms[id].Game)
				assertTwoHTTPInventory(t, s.rooms[id].Game)
				for _, c := range clients {
					public := current(c)["game"].(map[string]any)["catan"].(map[string]any)["two"].(map[string]any)
					if public["tokenRule"] != "ledger" || public["tokensIssued"] != float64(2) || public["canExchangeKnight"] != false {
						t.Fatal("public token ledger/action flags")
					}
				}
			})
		}
	}
}
