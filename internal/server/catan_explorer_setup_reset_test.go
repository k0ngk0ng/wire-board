package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestCatanExplorerCityHTTPSetupReset(t *testing.T) {
	for _, n := range []int{4, 6} {
		for _, mode := range []string{"manual", "timeout", "autoplay"} {
			t.Run(fmt.Sprintf("%d/%s", n, mode), func(t *testing.T) {
				s, ts, clients, id := newExplorerCityHTTP(t, n, "blocked")
				p := s.rooms[id].Game.Turn
				setAutoPlay(clients[p], current(clients[p]), true, 200)
				s.mu.Lock()
				// A legacy blocked room may have an expired nonzero deadline.
				s.rooms[id].TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
				before, _ := json.Marshal(s.rooms[id])
				s.expireSetups(time.Now().Add(time.Hour))
				s.runBots(time.Now().Add(time.Hour))
				after, _ := json.Marshal(s.rooms[id])
				s.mu.Unlock()
				if string(before) != string(after) {
					t.Fatal("blocked timer/bot mutated room")
				}
				clients[1].command(current(clients[1]), "catan_explorer_reset", nil, 400)
				clients[n].command(current(clients[n]), "catan_explorer_reset", nil, 400)
				clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_explorer_reset"}, 400)
				oldRoom := current(clients[0])
				request := map[string]any{"type": "catan_explorer_reset", "version": oldRoom["version"], "nonce": "reset-opening-nonce"}
				clients[0].post("/api/rooms/"+id, request, 200)
				r := s.rooms[id]
				if r.Game.Catan.Explorer.Setup.Step != 0 || r.Game.Catan.Explorer.Setup.PromptBase == 0 || r.Game.Turn != r.Game.Catan.StartPlayer || r.TurnDeadline < time.Now().Add(119*time.Second).UnixMilli() {
					t.Fatal("reset missed prompt, order or clock")
				}
				before, _ = json.Marshal(r)
				clients[0].post("/api/rooms/"+id, request, 200)
				clients[0].command(oldRoom, "catan_explorer_reset", nil, 409)
				clients[0].command(current(clients[0]), "catan_explorer_reset", nil, 400)
				after, _ = json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("reset replay/rejection mutated snapshot")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				clients[0].post("/api/rooms/"+id, request, 200)
				a, err := s.rooms[id].Game.BotAction(s.rooms[id].Game.Turn)
				if err != nil {
					t.Fatal(err)
				}
				a.Prompt = 1
				clients[s.rooms[id].Game.Turn].command(current(clients[0]), "action", a, 400)
				for seat := 0; seat < n; seat++ {
					setAutoPlay(clients[seat], current(clients[seat]), mode == "autoplay", 200)
				}
				for step := 0; s.rooms[id].Game.Catan.Explorer.Setup != nil; step++ {
					if step >= 4*n {
						t.Fatal("reset opening stalled")
					}
					r = s.rooms[id]
					assertExplorerCityHTTPPrivacy(t, clients, r.Game)
					switch mode {
					case "manual":
						a, err := r.Game.BotAction(r.Game.Turn)
						if err != nil {
							t.Fatal(err)
						}
						clients[r.Game.Turn].command(current(clients[0]), "action", a, 200)
					case "timeout":
						s.mu.Lock()
						s.expireSetups(time.UnixMilli(r.TurnDeadline + 1))
						s.mu.Unlock()
					case "autoplay":
						s.mu.Lock()
						s.runBots(time.UnixMilli(r.BotAt + 1000))
						s.mu.Unlock()
					}
					if step%n == 0 {
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
					}
				}
				if s.rooms[id].Game.Phase != "catan_roll" {
					t.Fatal("reset opening did not reach production")
				}
				clients[0].command(current(clients[0]), "catan_explorer_reset", nil, 400)
			})
		}
	}
}
