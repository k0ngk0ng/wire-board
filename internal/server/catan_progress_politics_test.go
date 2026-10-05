package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanProgressPoliticsHTTPPrivateAndQueuedResponses(t *testing.T) {
	for _, kind := range []string{"espionage", "wedding", "sabotage", "treason", "diplomacy"} {
		for _, mode := range []string{"manual", "timeout", "autoplay"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				state, err := game.NewCatanCitiesKnights(3, game.CatanOptions{})
				if err != nil {
					t.Fatal(err)
				}
				g := state.Catan
				g.SetupStep = 6
				g.TurnSerial = 1
				state.Turn, state.Phase = 0, "catan_turn"
				g.Ports = nil
				g.Vertices = []game.CatanVertex{{ID: 0, Owner: 0, Level: 1}, {ID: 1, Owner: -1}, {ID: 2, Owner: -1}, {ID: 3, Owner: -1}, {ID: 4, Owner: 1, Level: 2}, {ID: 5, Owner: 2, Level: 2}}
				g.Edges = []game.CatanEdge{{ID: 0, A: 0, B: 1, Owner: 0}, {ID: 1, A: 1, B: 2, Owner: 0}, {ID: 2, A: 2, B: 3, Owner: -1}, {ID: 3, A: 3, B: 4, Owner: 1}}
				g.Tiles = []game.CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0, 1, 2, 3, 4, 5}}}
				g.Players[0].Score, g.Players[1].Score, g.Players[2].Score = 1, 2, 2
				g.Players[1].Resources[0] = 3
				g.Players[2].Resources[5] = 3
				g.Bank[0] -= 3
				g.Bank[5] -= 3
				card := 18
				switch kind {
				case "espionage":
					cityProgressGive(t, g, 1, 4)
				case "wedding":
					card = 24
				case "sabotage":
					card = 20
				case "treason":
					card = 22
					g.CitiesKnights.Knights = []game.CatanKnight{{Owner: 1, Vertex: 3, Strength: 3, Active: true, ActivatedAt: 0}}
				case "diplomacy":
					card = 16
				}
				cityProgressGive(t, g, 0, card)
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = state
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_progress", "card": card, "target": 1, "edge": 1}, 200)
				savedTime := s.rooms[id].CatanTimeLeft
				if savedTime < 44000 || savedTime > 45000 {
					t.Fatal("card response failed to pause turn")
				}
				steps := 0
				for s.rooms[id].Game.CatanPendingActor() >= 0 {
					r = s.rooms[id]
					actor := r.Game.CatanPendingActor()
					phase := r.Game.Phase
					if steps > 2 {
						t.Fatal("response loop")
					}
					remain := r.TurnDeadline - time.Now().UnixMilli()
					// Timeout-mode subsequent deadlines follow our synthetic future clock.
					if mode != "timeout" && (remain < 119000 || remain > 120000) {
						t.Fatal("response did not receive fresh 120 seconds", phase, remain)
					}
					before, _ := json.Marshal(r)
					response := map[string]any{"type": phase}
					switch phase {
					case "catan_espionage":
						response["card"] = 4
					case "catan_wedding", "catan_sabotage":
						give := make([]int, 8)
						n := 2
						if phase == "catan_sabotage" {
							n = 1
						}
						if actor == 1 {
							give[0] = n
						} else {
							give[5] = n
						}
						response["give"] = give
					case "catan_treason_remove":
						response["vertex"] = 3
					case "catan_treason_place":
						response["vertex"] = 1
						response["color"] = 3
					case "catan_diplomacy":
						response["edge"] = 1
					default:
						t.Fatal("unexpected response", phase)
					}
					for _, p := range []int{0, 1, 2, 3} {
						if p != actor {
							clients[p].command(current(clients[p]), "action", response, 400)
						}
					}
					clients[actor].command(current(clients[actor]), "action", map[string]any{"type": phase, "card": 999, "vertex": 99, "edge": 99, "give": []int{0, 0, 0, 0, 0, 0, 0, 0}}, 400)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("invalid politics response changed state")
					}
					for viewer, c := range clients {
						view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
						k := view["citiesKnights"].(map[string]any)
						_, peek := k["pending"].(map[string]any)["progress"]
						if peek != (phase == "catan_espionage" && viewer == 0) {
							t.Fatal("private spy response leaked")
						}
						for p, raw := range k["players"].([]any) {
							_, ok := raw.(map[string]any)["progress"]
							if ok != (viewer == p) {
								t.Fatal("progress hand leaked")
							}
						}
						for p, raw := range view["players"].([]any) {
							_, ok := raw.(map[string]any)["resources"]
							if ok != (viewer == p) {
								t.Fatal("resource hand leaked")
							}
						}
					}
					ts.Close()
					s.Close()
					next, err := New(s.cfg, s.files)
					if err != nil {
						t.Fatal(err)
					}
					stopBotTicker(next)
					s = next
					defer next.Close()
					after, _ = json.Marshal(next.rooms[id])
					if string(before) != string(after) {
						t.Fatal("politics continuation lost on restart", phase)
					}
					ts = httptest.NewServer(next.Handler())
					defer ts.Close()
					for _, c := range clients {
						c.base = ts.URL
					}
					var resolvedAt time.Time
					switch mode {
					case "manual":
						clients[actor].command(current(clients[actor]), "action", response, 200)
						resolvedAt = time.Now()
					case "timeout":
						s.mu.Lock()
						resolvedAt = time.UnixMilli(s.rooms[id].TurnDeadline)
						s.expireSetups(resolvedAt)
						s.mu.Unlock()
					case "autoplay":
						s.mu.Lock()
						s.rooms[id].Seats[actor].AutoPlay = true
						s.rooms[id].BotAt = 0
						resolvedAt = time.Now()
						s.runBots(resolvedAt)
						s.mu.Unlock()
					}
					r = s.rooms[id]
					if r.Game.CatanPendingActor() >= 0 {
						if r.CatanTimeLeft != savedTime || r.TurnDeadline-resolvedAt.UnixMilli() < 119000 || r.TurnDeadline-resolvedAt.UnixMilli() > 120000 {
							t.Fatal("next actor did not receive independent clock", phase)
						}
					} else {
						remain = r.TurnDeadline - resolvedAt.UnixMilli()
						if remain < savedTime-1000 || remain > savedTime {
							t.Fatal("politics did not restore original action clock", remain, savedTime)
						}
					}
					steps++
				}
				r = s.rooms[id]
				g = r.Game.Catan
				if r.Game.Turn != 0 || r.Game.Phase != "catan_turn" {
					t.Fatal("politics did not return to original turn")
				}
				switch kind {
				case "espionage":
					if len(g.CitiesKnights.Players[0].Progress) != 1 || len(g.CitiesKnights.Players[1].Progress) != 0 {
						t.Fatal("spy transfer")
					}
				case "wedding":
					if g.Players[0].Resources[0] != 2 || g.Players[0].Resources[5] != 2 || steps != 2 {
						t.Fatal("wedding sequence")
					}
				case "sabotage":
					if g.Players[1].Resources[0] != 2 || g.Players[2].Resources[5] != 2 || steps != 2 {
						t.Fatal("sabotage sequence")
					}
				case "treason":
					if len(g.CitiesKnights.Knights) != 1 || g.CitiesKnights.Knights[0].Owner != 0 || g.CitiesKnights.Knights[0].Strength != 3 || !g.CitiesKnights.Knights[0].Active || steps != 2 {
						t.Fatal("treason replacement state")
					}
				case "diplomacy":
					if g.Edges[1].Owner != 0 || steps != 1 {
						t.Fatal("diplomacy relocation")
					}
				}
			})
		}
	}
}
