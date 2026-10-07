package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

// Explicit middle-game placement fixtures. Formal room recipes and natural
// full-game acceptance remain separate from this response/restart matrix.
func TestCatanTwoCaravansHTTPPlacementsClocksRestart(t *testing.T) {
	for _, tie := range []bool{false, true} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("tie=%v/%s", tie, mode), func(t *testing.T) {
				s, ts, clients, id := newTwoFullTable(t)
				state, e := game.NewCatanTwoCaravans(2, game.CatanOptions{})
				if e != nil {
					t.Fatal(e)
				}
				state.Turn, state.Catan.StartPlayer = 0, 0
				state.Catan.SetupStep = 4
				state.Phase = "catan_turn"
				state.Catan.Two.Rolls = []int{2, 12}
				state.Catan.Caravans.Built = true
				bids := []int{2, 1}
				if tie {
					bids[0] = 1
				}
				for p, n := range bids {
					state.Catan.Players[p].Resources[2] = n
					state.Catan.Bank[2] -= n
				}
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if e = s.save(r); e != nil {
					t.Fatal(e)
				}
				s.mu.Unlock()
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_end"}, 200)
				for step := 0; step < 4; step++ {
					r = s.rooms[id]
					actor := r.Game.CatanPendingActor()
					phase := r.Game.Phase
					if actor < 0 || r.Game.Turn != 0 {
						t.Fatal("premature transition")
					}
					if left := r.TurnDeadline - time.Now().UnixMilli(); left < 119000 || left > 120000 {
						t.Fatal("missing response clock", step, left)
					}
					before, _ := json.Marshal(r)
					for _, p := range []int{1 - actor, 2} {
						clients[p].command(current(clients[p]), "action", game.Action{Type: phase}, 400)
					}
					clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_end"}, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("invalid action changed state/clock")
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					r = s.rooms[id]
					// Escrow must survive both first-placement save and viewer reconstruction.
					if step == 3 && (len(r.Game.Catan.Caravans.Wagons) != 1 || r.Game.Catan.Bank[2] != 19-bids[0]-bids[1]) {
						t.Fatal("first placement or escrow lost")
					}
					for p, c := range clients {
						view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
						pub := view["caravans"].(map[string]any)
						if pub["canAct"] != (p == actor) {
							t.Fatal("response privacy")
						}
						for owner, seat := range view["players"].([]any) {
							if owner != p && seat.(map[string]any)["resources"] != nil {
								t.Fatal("hand leak")
							}
						}
					}
					if step < 2 {
						clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, bids[actor], 0, 0}}, 200)
						continue
					}
					// Prove that even one winning bidder's two placements receive separate
					// deadlines; do not mistake a still-fresh first timer for a reset.
					s.mu.Lock()
					r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
					s.save(r)
					s.mu.Unlock()
					switch mode {
					case "manual":
						a, e := r.Game.BotAction(actor)
						if e != nil {
							t.Fatal(e)
						}
						clients[actor].command(current(clients[actor]), "action", a, 200)
					case "autoplay":
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(time.Now())
						s.mu.Unlock()
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					case "timeout":
						s.mu.Lock()
						s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
						s.expireSetups(time.Now())
						s.mu.Unlock()
						reclaimTimeoutHumans(t, s, clients, id)
					}
				}
				r = s.rooms[id]
				if r.Game.CatanPendingActor() != -1 || len(r.Game.Catan.Caravans.Wagons) != 2 || r.Game.Catan.Bank[2] != 19 || r.Game.Turn != 1 || r.Game.Phase != "catan_roll" || len(r.Game.Catan.Two.Rolls) != 0 {
					t.Fatal("two-wagon completion")
				}
				if left := r.TurnDeadline - time.Now().UnixMilli(); left < 119000 || left > 120000 {
					t.Fatal("new turn clock", left)
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
			})
		}
	}
}
