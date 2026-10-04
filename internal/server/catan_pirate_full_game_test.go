package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// The room selector intentionally remains closed. Inject only the initial
// official scenario; all subsequent moves use real authenticated HTTP requests,
// the production autoplay dispatcher or the production timeout handler.
func TestCatanPirateFullHTTPGames(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, helpers := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/helpers=%v", n, helpers), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for i := range clients {
					clients[i] = newClient(t, ts.URL)
					clients[i].register(fmt.Sprintf("远征玩家%d", i))
				}
				options := game.CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}
				r := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "海盗群岛联机验证", "capacity": n, "catanOptions": options}, 201)
				id := r["id"].(string)
				for i := 1; i < n; i++ {
					clients[i].command(current(clients[0]), "join", nil, 200)
				}
				for i := 0; i < n; i++ {
					clients[i].command(current(clients[0]), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				scenario, err := game.NewCatanPirateIslands(n, options)
				if err != nil {
					t.Fatal(err)
				}
				s.mu.Lock()
				s.rooms[id].Game = scenario
				s.rooms[id].startTurnClock(time.Now())
				err = s.save(s.rooms[id])
				s.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				// Reopen the same database while keeping every client's login cookie.
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
						t.Fatal("saved naval state/clock changed on restart")
					}
					s, ts = next, nextHTTP
					for _, c := range clients {
						c.base = ts.URL
					}
				}
				restartedBattle, restartedReward := false, false
				steps, automatic, timeouts, rewards := 0, 0, 0, 0
				for ; steps < 12000 && !s.rooms[id].Game.Finished; steps++ {
					room := s.rooms[id]
					state := room.Game
					g := state.Catan
					if g.Seafarers.PirateIslands.Battle != nil && !restartedBattle {
						restart()
						restartedBattle = true
						room = s.rooms[id]
						state = room.Game
						g = state.Catan
					}
					if state.Phase == "catan_fleet_reward" {
						rewards++
						if !restartedReward {
							restart()
							restartedReward = true
							room = s.rooms[id]
							state = room.Game
							g = state.Catan
						}
					}
					actor := state.Turn
					if pending := state.CatanPendingActor(); pending >= 0 {
						actor = pending
					} else if state.Phase == "catan_discard" {
						for i, due := range g.DiscardDue {
							if due > 0 {
								actor = i
								break
							}
						}
					}
					if steps%61 == 0 {
						for _, viewer := range []int{actor, n} {
							v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
							if _, leak := v["devDeck"]; leak {
								t.Fatal("deck order leaked")
							}
							for i, raw := range v["players"].([]any) {
								p := raw.(map[string]any)
								_, hand := p["resources"]
								_, dev := p["dev"]
								if hand != (viewer == i) || dev != (viewer == i) {
									t.Fatal("private hand leaked", viewer, i)
								}
							}
						}
					}
					if steps%29 == 0 && (g.SetupStep < g.SetupLimit() || state.CatanPendingActor() >= 0 || state.Phase == "catan_discard") {
						version := room.Version
						s.mu.Lock()
						room.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
						s.expireSetups(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("timeout failed to advance", state.Phase)
						}
						timeouts++
						continue
					}
					if steps%17 == 0 {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						version := s.rooms[id].Version
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("autoplay failed to advance", state.Phase, actor)
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
				room := s.rooms[id]
				g := room.Game.Catan
				if !room.Game.Finished || room.Status != "finished" || len(room.Game.Winners) != 1 {
					t.Fatal("game failed to finish", steps, room.Game.Phase)
				}
				winner := room.Game.Winners[0]
				if g.Players[winner].Score < 10 || g.Seafarers.PirateIslands.Fortresses[winner].Strength != 0 {
					t.Fatal("invalid victory")
				}
				if !restartedBattle || automatic == 0 || timeouts == 0 {
					t.Fatal("missing integration coverage")
				}
				if rewards > 0 && !restartedReward {
					t.Fatal("reward restart missing")
				}
				uid := room.Seats[winner].ID
				code, profile := clients[n].request("GET", "/api/players/"+uid, nil)
				if code != 200 {
					t.Fatal(code)
				}
				stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
				if stats["wins"] != float64(1) || stats["played"] != float64(1) {
					t.Fatal("history result missing", stats)
				}
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
				t.Logf("steps=%d autoplay=%d timeouts=%d rewards=%d battles=%d", steps, automatic, timeouts, rewards, g.Seafarers.PirateIslands.Battle.ID)
			})
		}
	}
}
