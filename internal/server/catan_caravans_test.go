package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanCaravansHTTPVotingClocksAndRestart(t *testing.T) {
	for _, n := range []int{3, 5, 6} {
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
				state, err := game.NewCatanCaravans(n, game.CatanOptions{FiveSix: n > 4})
				if err != nil {
					t.Fatal(err)
				}
				state.Turn, state.Catan.StartPlayer = 0, 0
				if state.Catan.Paired != nil {
					state.Catan.Paired.Primary, state.Catan.Paired.Secondary = 0, 3
				}
				state.Catan.SetupStep = state.Catan.SetupLimit()
				state.Phase = "catan_turn"
				state.Catan.Caravans.Built = true
				// Two equal public bidders ensure the negotiation phase is exercised.
				for _, p := range []int{0, 1} {
					state.Catan.Players[p].Resources[2] = 1
					state.Catan.Bank[2]--
				}
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_end"}, 200)
				steps := 0
				for s.rooms[id].Game.CatanPendingActor() >= 0 && steps < 2*n+4 {
					r = s.rooms[id]
					actor := r.Game.CatanPendingActor()
					phase := r.Game.Phase
					left := r.TurnDeadline - time.Now().UnixMilli()
					if left < 119000 || left > 120000 || r.Game.Turn != 0 {
						t.Fatal("response clock or owner", phase, actor, left)
					}
					before, _ := json.Marshal(r)
					clients[n].command(current(clients[n]), "action", game.Action{Type: phase, Tokens: []int{0, 0, 0, 0, 0}}, 400)
					clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", game.Action{Type: phase, Tokens: []int{0, 0, 0, 0, 0}}, 400)
					clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_end"}, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("invalid vote changed room/clock")
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					r = s.rooms[id]
					// The first two bids are deliberate manual choices in every path;
					// later responders use the selected automatic/manual mechanism.
					if phase == "catan_caravan_bid" && actor < 2 {
						clients[actor].command(current(clients[actor]), "action", game.Action{Type: phase, Tokens: []int{0, 0, 1, 0, 0}}, 200)
					} else {
						switch mode {
						case "manual":
							a, err := r.Game.BotAction(actor)
							if err != nil {
								t.Fatal(err)
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
						}
					}
					steps++
					if s.rooms[id].Game.CatanPendingActor() == actor && s.rooms[id].Game.Phase == phase {
						t.Fatal("responder did not advance", phase, mode)
					}
				}
				r = s.rooms[id]
				if r.Game.CatanPendingActor() >= 0 || len(r.Game.Catan.Caravans.Wagons) != 1 || r.Game.Catan.Bank[2] != 19 && n == 3 || r.Game.Catan.Bank[2] != 24 && n > 4 {
					t.Fatal("vote not settled", r.Game.Phase)
				}
				if n > 4 {
					if r.Game.Turn != 3 || !r.Game.Catan.Paired.Second || r.Game.Phase != "catan_turn" {
						t.Fatal("paired second action")
					}
				} else if r.Game.Turn != 1 || r.Game.Phase != "catan_roll" {
					t.Fatal("next turn")
				}
				if left := r.TurnDeadline - time.Now().UnixMilli(); left < 119000 || left > 120000 {
					t.Fatal("new action restored old 45 seconds", left)
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if n > 4 {
					// A second action's independent vote ends the production pair.
					s.mu.Lock()
					s.rooms[id].Game.Catan.Caravans.Built = true
					if err = s.save(s.rooms[id]); err != nil {
						t.Fatal(err)
					}
					s.mu.Unlock()
					clients[3].command(current(clients[3]), "action", game.Action{Type: "catan_end"}, 200)
					if s.rooms[id].Game.CatanPendingActor() != 3 || s.rooms[id].Game.Turn != 3 {
						t.Fatal("secondary response owner")
					}
					for step := 0; s.rooms[id].Game.CatanPendingActor() >= 0 && step < n+2; step++ {
						s.mu.Lock()
						s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
						s.expireSetups(time.Now())
						s.mu.Unlock()
					}
					r = s.rooms[id]
					if r.Game.CatanPendingActor() >= 0 || r.Game.Catan.Paired.Second || r.Game.Turn != 1 || r.Game.Phase != "catan_roll" || len(r.Game.Catan.Caravans.Wagons) != 2 {
						t.Fatal("secondary vote transition")
					}
				}
			})
		}
	}
}
