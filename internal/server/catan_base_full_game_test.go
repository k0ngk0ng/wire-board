package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

// Only the waiting selection is provisioned. Start, manual moves, autoplay and
// timeout handling are production paths; no running game state is substituted.
func TestCatanBaseFixedFullHTTPGames(t *testing.T)   { testCatanBaseFullHTTPGames(t, false) }
func TestCatanHarborsBaseFullHTTPGames(t *testing.T) { testCatanBaseFullHTTPGames(t, true) }
func testCatanBaseFullHTTPGames(t *testing.T, harbors bool) {
	players := []int{5, 6}
	if harbors {
		players = []int{3, 6}
	}
	for _, n := range players {
		layout, target, neutralWant := "fixed", 10, (6-n)*2
		rules := "catan-base-5-6-2025"
		if n < 5 {
			layout, rules, neutralWant = "variable", "catan-base-2025", 0
		}
		if harbors {
			target++
		}
		for _, helpers := range []bool{false, true} {
			if harbors && helpers {
				continue
			}
			t.Run(fmt.Sprintf("%d/helpers=%v", n, helpers), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for i := range clients {
					clients[i] = newClient(t, ts.URL)
					clients[i].register(fmt.Sprintf("固定布局%d", i))
				}
				raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "固定布局整局", "capacity": n, "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}}, 201)
				id := raw["id"].(string)
				for i := 1; i < n; i++ {
					clients[i].command(current(clients[0]), "join", nil, 200)
				}
				provisionCatanBase(t, s, id, layout)
				if harbors {
					provisionCatanHarbors(t, s, id)
				}
				for i := 0; i < n; i++ {
					clients[i].command(current(clients[i]), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				if layout == "fixed" && s.rooms[id].Game.Phase != "catan_roll" {
					t.Fatal("fixed layout started manual setup")
				}
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
						t.Fatal("fixed colors, neutral villages or clock lost on restart")
					}
					s, ts = next, nextHTTP
					for _, c := range clients {
						c.base = ts.URL
					}
				}
				restart()
				steps, automatic, timeouts := 0, 0, 0
				restartedPaired, restartedPending := false, false
				for ; steps < 12000 && !s.rooms[id].Game.Finished; steps++ {
					room := s.rooms[id]
					state := room.Game
					g := state.Catan
					if g.Paired != nil && g.Paired.Second && !restartedPaired {
						restart()
						restartedPaired = true
						room = s.rooms[id]
						state = room.Game
						g = state.Catan
					}
					if state.CatanPendingActor() >= 0 || state.Phase == "catan_discard" {
						if !restartedPending {
							restart()
							restartedPending = true
							room = s.rooms[id]
							state = room.Game
							g = state.Catan
						}
						if timeouts == 0 || steps%29 == 0 {
							version := room.Version
							s.mu.Lock()
							room.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
							s.expireSetups(time.Now())
							s.mu.Unlock()
							if s.rooms[id].Version <= version {
								t.Fatal("pending timeout failed", state.Phase)
							}
							timeouts++
							continue
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
							if v["baseSetup"].(map[string]any)["layout"] != layout {
								t.Fatal("public layout disappeared")
							}
							for i, raw := range v["players"].([]any) {
								p := raw.(map[string]any)
								_, hand := p["resources"]
								_, dev := p["dev"]
								if hand != (viewer == i) || dev != (viewer == i) {
									t.Fatal("hand leaked", viewer, i)
								}
							}
						}
					}
					if steps%17 == 0 {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						version := s.rooms[id].Version
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("autoplay failed", state.Phase)
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
				if !room.Game.Finished || room.Status != "finished" || len(room.Game.Winners) != 1 {
					t.Fatal("fixed game did not finish", steps, room.Game.Phase)
				}
				winner := room.Game.Winners[0]
				if room.Game.Catan.Players[winner].Score < target || automatic == 0 || (n > 4 && !restartedPaired) {
					t.Fatal("victory or paired/autoplay coverage missing")
				}
				if restartedPending && timeouts == 0 {
					t.Fatal("pending timeout coverage missing")
				}
				neutral := 0
				for _, v := range room.Game.Catan.Vertices {
					if v.Owner < 0 && v.Level > 0 {
						neutral++
					}
				}
				if neutral != neutralWant {
					t.Fatal("neutral villages changed")
				}
				code, profile := clients[n].request("GET", "/api/players/"+room.Seats[winner].ID, nil)
				if code != 200 {
					t.Fatal(code)
				}
				match := profile["history"].([]any)[0].(map[string]any)
				stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
				if stats["wins"] != float64(1) || stats["played"] != float64(1) || match["catanLayout"] != layout || match["catanRules"] != rules {
					t.Fatal("fixed results not archived")
				}
				if harbors && (room.Game.Catan.Harbors == nil || match["catanExpansionRules"].(map[string]any)["harbors"] != game.CatanHarborsRules) {
					t.Fatal("harbor rules missing from game/history")
				}
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
				t.Logf("steps=%d autoplay=%d timeouts=%d pendingRestart=%v", steps, automatic, timeouts, restartedPending)
			})
		}
	}
}
