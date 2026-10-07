package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func TestCatanExplorerCityHTTPGoldLedger(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newExplorerCityHTTP(t, n, "roll")
			s.mu.Lock()
			r := s.rooms[id]
			g, p := r.Game.Catan, r.Game.Turn
			e, k := g.Explorer.Economy, g.CitiesKnights
			// Controlled shortage: move existing coins to another player, plus
			// one real Alchemy card from its deck so HTTP rolls exactly two.
			e.Gold[(p+1)%n] += e.GoldBank
			e.GoldBank = 0
			card := slices.Index(k.ProgressDecks[0], 0)
			if card < 0 {
				t.Fatal("missing Alchemy")
			}
			k.ProgressDecks[0] = append(k.ProgressDecks[0][:card], k.ProgressDecks[0][card+1:]...)
			k.Players[p].Progress = append(k.Players[p].Progress, 0)
			gold := slices.Clone(e.Gold)
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			body := map[string]any{"type": "action", "version": r.Version, "nonce": "ledger-production-request", "action": map[string]any{"type": "catan_progress", "card": 0, "tokens": []int{1, 1}, "prompt": g.TurnSerial}}
			clients[p].post("/api/rooms/"+id, body, 200)
			r = s.rooms[id]
			e = r.Game.Catan.Explorer.Economy
			issued := 0
			for seat, amount := range e.Gold {
				issued += amount - gold[seat]
			}
			if issued <= 0 || e.GoldIssued != issued || e.GoldBank != 0 || r.Game.Phase != "catan_turn" {
				t.Fatal("production shortage did not complete with ledger credits")
			}
			before, _ := json.Marshal(r)
			clients[p].post("/api/rooms/"+id, body, 200)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("replayed production paid twice")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			clients[p].post("/api/rooms/"+id, body, 200)
			assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
			for _, c := range clients {
				v := current(c)["game"].(map[string]any)["catan"].(map[string]any)["explorer"].(map[string]any)["economy"].(map[string]any)
				if v["goldRule"] != "ledger" || v["goldIssued"] != float64(issued) {
					t.Fatal("supplemental rule absent from player/spectator view")
				}
			}
			// Real purchase returns credits to the bank, persists them, and still
			// consumes one of the two purchases allowed in this action stage.
			clients[p].command(current(clients[p]), "action", map[string]any{"type": "catan_explorer_bank", "color": -1, "target": 0, "prompt": s.rooms[id].Game.Catan.TurnSerial}, 200)
			e = s.rooms[id].Game.Catan.Explorer.Economy
			if e.GoldBank != 2 || e.GoldIssued != issued || e.Turn.Bought != 1 {
				t.Fatal("payment did not recycle ledger coins")
			}
			_, _ = restartRiversHTTP(t, s, ts, clients, id)
		})
	}
}
