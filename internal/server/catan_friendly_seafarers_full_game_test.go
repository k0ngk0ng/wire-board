package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCatanFriendlySeaConfiguredFullHTTPGames(t *testing.T) {
	for _, scenario := range []string{"shores", "desert", "wonders", "cloth", "pirate_islands"} {
		small := 3
		if scenario == "shores" {
			small = 4
		}
		for _, n := range []int{small, 6} {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for i := range clients {
					clients[i] = newClient(t, ts.URL)
					clients[i].register(fmt.Sprintf("友善航海%d", i))
				}
				raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "友善航海整局", "capacity": n, "catanOptions": game.CatanOptions{FiveSix: n > 4}}, 201)
				id := raw["id"].(string)
				for i := 1; i < n; i++ {
					clients[i].command(current(clients[0]), "join", nil, 200)
				}
				provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: scenario})
				provisionCatanFriendlyRobber(t, s, id)
				if n == 6 {
					provisionCatanHarbors(t, s, id)
				}
				for i := 0; i < n; i++ {
					clients[i].command(current(clients[i]), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				restart := func() {
					before, _ := json.Marshal(s.rooms[id])
					ts.Close()
					s.Close()
					next, e := New(s.cfg, s.files)
					if e != nil {
						t.Fatal(e)
					}
					stopBotTicker(next)
					nextHTTP := httptest.NewServer(next.Handler())
					t.Cleanup(func() { nextHTTP.Close(); next.Close() })
					after, _ := json.Marshal(next.rooms[id])
					if string(before) != string(after) {
						t.Fatal("sea state/clock lost on restart")
					}
					s, ts = next, nextHTTP
					for _, c := range clients {
						c.base = ts.URL
					}
				}
				restart()
				steps, automatic, timeouts := 0, 0, 0
				pairedRestart, pendingRestart := false, false
				for ; steps < 12000 && !s.rooms[id].Game.Finished; steps++ {
					r := s.rooms[id]
					state := r.Game
					g := state.Catan
					pending := state.CatanPendingActor() >= 0 || state.Phase == "catan_discard"
					paired := g.Paired != nil && g.Paired.Second
					if (paired && !pairedRestart) || (pending && !pendingRestart) {
						restart()
						pairedRestart = pairedRestart || paired
						pendingRestart = pendingRestart || pending
						r = s.rooms[id]
						state = r.Game
						g = state.Catan
					}
					if pending && (timeouts == 0 || steps%43 == 0) {
						version := r.Version
						s.mu.Lock()
						r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
						s.expireSetups(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("pending timeout failed", state.Phase)
						}
						timeouts++
						continue
					}
					actor := state.Turn
					if p := state.CatanPendingActor(); p >= 0 {
						actor = p
					} else if state.Phase == "catan_discard" {
						for i, due := range g.DiscardDue {
							if due > 0 {
								actor = i
								break
							}
						}
					}
					if steps%97 == 0 {
						for _, viewer := range []int{actor, n} {
							v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
							if v["friendlyRobber"] == nil || v["seafarers"].(map[string]any)["scenario"] != scenario {
								t.Fatal("combined view missing")
							}
							for i, raw := range v["players"].([]any) {
								p := raw.(map[string]any)
								for _, key := range []string{"dev", "resources"} {
									if _, visible := p[key]; visible != (i == viewer) {
										t.Fatal("private hand leaked")
									}
								}
							}
						}
					}
					if steps%19 == 0 {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						version := s.rooms[id].Version
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("autoplay stalled", state.Phase)
						}
						if !s.rooms[id].Game.Finished {
							setAutoPlay(clients[actor], current(clients[actor]), false, 200)
						}
						automatic++
						continue
					}
					action, e := state.BotAction(actor)
					if e != nil {
						t.Fatal(steps, state.Phase, e)
					}
					clients[actor].command(current(clients[actor]), "action", action, 200)
				}
				r := s.rooms[id]
				if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) == 0 || automatic == 0 || n == 6 && !pairedRestart {
					t.Fatal("incomplete game/coverage", steps, r.Game.Phase)
				}
				for _, winner := range r.Game.Winners {
					code, profile := clients[n].request("GET", "/api/players/"+r.Seats[winner].ID, nil)
					if code != 200 {
						t.Fatal(code)
					}
					record := profile["history"].([]any)[0].(map[string]any)
					rules := record["catanExpansionRules"].(map[string]any)
					if rules["friendly_robber"] != game.CatanFriendlyRobberRules || rules["seafarers"] != game.CatanSeafarersRules || record["catanScenario"] != scenario || n == 6 && rules["harbors"] != game.CatanHarborsRules {
						t.Fatal("combined history wrong", record)
					}
					stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
					if stats["wins"] != float64(1) || stats["played"] != float64(1) {
						t.Fatal("winner not recorded", stats)
					}
				}
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
				t.Logf("steps=%d autoplay=%d timeouts=%d pendingRestart=%v pairedRestart=%v", steps, automatic, timeouts, pendingRestart, pairedRestart)
			})
		}
	}
}
