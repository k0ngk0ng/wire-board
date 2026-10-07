package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanAttackLastPieceHTTPBuildingReplayAndRestart(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
			state := attackHTTPFixture(t, n)
			g, actor := state.Catan, state.Turn
			a := g.Attack
			// Valid controlled midgame stock, not a naturally reached capture count.
			left := a.Map.Barbarians - 1
			for _, amount := range a.Barbarians {
				left -= amount
			}
			for i := 0; i < left; i++ {
				a.Prisoners[i%n]++
			}
			for p := range g.Players {
				g.Players[p].Score += a.Prisoners[p] / 2
			}
			vertex := -1
			for _, v := range g.Vertices {
				if v.Owner == actor && v.Level == 1 {
					vertex = v.ID
					break
				}
			}
			if vertex < 0 {
				t.Fatal("missing starting settlement")
			}
			for c, want := range []int{0, 0, 0, 2, 3} {
				g.Bank[c] += g.Players[actor].Resources[c] - want
				g.Players[actor].Resources[c] = want
			}
			beforeCounts := slices.Clone(a.Barbarians)
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = state
			r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
			deadline := r.TurnDeadline
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			body := map[string]any{"type": "action", "version": r.Version, "nonce": "last-piece-city", "action": game.Action{Type: "catan_city", Vertex: vertex}}
			clients[actor].post("/api/rooms/"+id, body, 200)
			r = s.rooms[id]
			g, a = r.Game.Catan, r.Game.Catan.Attack
			if g.Vertices[vertex].Level != 2 || !slices.Equal(g.Players[actor].Resources, []int{0, 0, 0, 0, 0}) || a.Sequence != 1 || len(a.Landing.Rolls) != 1 {
				t.Fatal("building failed or landing did not finish once")
			}
			changed := 0
			for tile, amount := range a.Barbarians {
				if amount != beforeCounts[tile] {
					changed++
					if amount != beforeCounts[tile]+1 {
						t.Fatal("wrong last-piece count")
					}
				}
			}
			if changed != 1 || r.TurnDeadline != deadline || r.Game.Phase != "catan_turn" || r.Game.Turn != actor {
				t.Fatal("landing changed multiple tiles or lost original action clock")
			}
			before, _ := json.Marshal(r)
			clients[actor].post("/api/rooms/"+id, body, 200)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			clients[actor].post("/api/rooms/"+id, body, 200)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("replay or restart rerolled last-piece allocation")
			}
			for viewer, c := range clients {
				view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
				pub := view["attack"].(map[string]any)
				if pub["landingSupplyRule"] != "random-last" || pub["supply"] != float64(0) || pub["deck"] != nil {
					t.Fatal("public shortage rule, stock, or secret deck")
				}
				for owner, raw := range view["players"].([]any) {
					if owner != viewer && raw.(map[string]any)["resources"] != nil {
						t.Fatal("opponent hand leaked")
					}
				}
			}
		})
	}
}

func TestCatanAttackGoldLedgerHTTPRewardReplayAndRestart(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
			state := attackHTTPFixture(t, n)
			prepareAttackHTTPCard(t, state, "treason")
			g, actor := state.Catan, state.Turn
			a := g.Attack
			a.Gold[(actor+1)%n] += a.GoldBank
			a.GoldBank = 0
			g.Bank[0] -= 4
			g.Players[actor].Resources[0] += 4
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = state
			r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_buy_dev"}, 200)
			r = s.rooms[id]
			choice, err := r.Game.BotAction(actor)
			if err != nil {
				t.Fatal(err)
			}
			body := map[string]any{"type": "action", "version": r.Version, "nonce": "empty-bank-treason", "action": choice}
			clients[actor].post("/api/rooms/"+id, body, 200)
			r = s.rooms[id]
			a = r.Game.Catan.Attack
			if a.GoldIssued != 2 || a.Gold[actor] != 2 || a.GoldBank != 0 || r.Game.Phase != "catan_turn" {
				t.Fatal("treason did not finish with exact ledger reward")
			}
			before, _ := json.Marshal(r)
			clients[actor].post("/api/rooms/"+id, body, 200)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			clients[actor].post("/api/rooms/"+id, body, 200)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("replay minted more coins or repeated treason")
			}
			for _, c := range clients {
				pub := current(c)["game"].(map[string]any)["catan"].(map[string]any)["attack"].(map[string]any)
				if pub["goldRule"] != "ledger" || pub["goldIssued"] != float64(2) {
					t.Fatal("supplemental rule missing")
				}
			}
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_coin_buy", Color: 1}, 200)
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_coin_sell", Color: 0}, 200)
			a = s.rooms[id].Game.Catan.Attack
			if a.GoldIssued != 2 || a.GoldBank != 1 || a.Gold[actor] != 1 || a.Bought != 1 {
				t.Fatal("bank payment was not reused")
			}
			_, _ = restartRiversHTTP(t, s, ts, clients, id)
		})
	}
}
