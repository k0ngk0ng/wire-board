package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

// Inject only the official initial state while the room picker is closed.
// All subsequent moves use normal HTTP actions, autoplay or timeout handling.
func TestCatanNewWorldFullHTTPGames(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, helpers := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/helpers=%v", n, helpers), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for i := range clients {
					clients[i] = newClient(t, ts.URL)
					clients[i].register(fmt.Sprintf("世界玩家%d", i))
				}
				options := game.CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}
				raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "新世界联机验证", "capacity": n, "catanOptions": options}, 201)
				id := raw["id"].(string)
				for i := 1; i < n; i++ {
					clients[i].command(current(clients[0]), "join", nil, 200)
				}
				for i := 0; i < n; i++ {
					clients[i].command(current(clients[0]), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				scenario, err := game.NewCatanNewWorld(n, options)
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
				restarted := false
				steps, automatic, timeouts := 0, 0, 0
				for ; steps < 10000 && !s.rooms[id].Game.Finished; steps++ {
					room := s.rooms[id]
					if room.Game.Catan.Seafarers.NewWorld.Index == 2 && !restarted {
						before, _ := json.Marshal(room)
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
							t.Fatal("port order and clock changed on restart")
						}
						s, ts, room = next, nextHTTP, next.rooms[id]
						for _, c := range clients {
							c.base = ts.URL
						}
						restarted = true
					}
					state, g := room.Game, room.Game.Catan
					if steps%31 == 0 {
						for _, viewer := range []int{state.Turn, n} {
							view := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
							world := view["seafarers"].(map[string]any)["newWorld"].(map[string]any)
							if _, ok := world["ports"]; ok {
								t.Fatal("hidden port order exposed")
							}
							for i, raw := range view["players"].([]any) {
								p := raw.(map[string]any)
								_, hand := p["resources"]
								_, dev := p["dev"]
								if hand != (viewer == i) || dev != (viewer == i) {
									t.Fatal("private hand leaked")
								}
							}
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
					if steps%29 == 0 && (g.SetupStep < g.SetupLimit() || state.CatanPendingActor() >= 0 || state.Phase == "catan_discard") {
						version := room.Version
						s.mu.Lock()
						room.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
						s.expireSetups(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("timeout did not advance", state.Phase)
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
							t.Fatal("autoplay did not advance", state.Phase)
						}
						if !s.rooms[id].Game.Finished {
							setAutoPlay(clients[actor], current(clients[actor]), false, 200)
						}
						automatic++
						continue
					}
					a, err := state.BotAction(actor)
					if err != nil {
						t.Fatal(steps, state.Phase, err)
					}
					clients[actor].command(current(clients[actor]), "action", a, 200)
				}
				room := s.rooms[id]
				if !room.Game.Finished || room.Status != "finished" || len(room.Game.Winners) != 1 || !restarted || automatic == 0 || timeouts == 0 {
					t.Fatal("full game coverage incomplete", steps, restarted, automatic, timeouts)
				}
				winner := room.Game.Winners[0]
				if room.Game.Catan.Players[winner].Score < 12 || room.Game.Catan.Seafarers.NewWorld.Index != len(room.Game.Catan.Ports) {
					t.Fatal("new world victory or completed ports")
				}
				code, profile := clients[n].request("GET", "/api/players/"+room.Seats[winner].ID, nil)
				if code != 200 || profile["stats"].(map[string]any)["catan"].(map[string]any)["wins"] != float64(1) {
					t.Fatal("new world victory absent from history")
				}
				t.Logf("steps=%d autoplay=%d timeouts=%d winner=%d", steps, automatic, timeouts, winner)
			})
		}
	}
}

func TestCatanNewWorldPortPermissionAndFreshClock(t *testing.T) {
	s, _, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	scenario, err := game.NewCatanNewWorld(3, game.CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	scenario.Turn, scenario.Catan.StartPlayer = 2, 2
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = scenario
	deadline := time.Now().Add(40 * time.Second).UnixMilli()
	r.TurnDeadline = deadline
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	a, err := scenario.BotAction(2)
	if err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []int{0, 3} {
		clients[viewer].command(current(clients[viewer]), "action", a, 400)
	}
	clients[2].command(current(clients[2]), "action", game.Action{Type: "catan_world_port", Edge: -1}, 400)
	if s.rooms[id].TurnDeadline != deadline {
		t.Fatal("invalid action reset clock")
	}
	clients[2].command(current(clients[2]), "action", a, 200)
	r = s.rooms[id]
	left := r.TurnDeadline - time.Now().UnixMilli()
	if r.Game.Turn != 0 || r.Game.Catan.SetupStep != 0 || r.Game.Catan.Seafarers.NewWorld.Index != 1 || left < 119000 || left > 120000 {
		t.Fatal("port handoff clock", left)
	}
	// Every timeout places one port, with a fresh clock; the final port returns
	// to the original first player without also placing their first settlement.
	s.mu.Lock()
	defer s.mu.Unlock()
	for index := 1; index < 10; index++ {
		now := time.Now()
		r.TurnDeadline = now.Add(-time.Second).UnixMilli()
		s.expireSetups(now)
		r = s.rooms[id]
		if r.Game.Catan.Seafarers.NewWorld.Index != index+1 || r.Game.Catan.SetupStep != 0 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
			t.Fatal("timeout skipped a port/setup step", index)
		}
	}
	if r.Game.Phase != "catan_setup_settlement" || r.Game.Turn != 2 {
		t.Fatal("wrong first settler")
	}
	for _, v := range r.Game.Catan.Vertices {
		if v.Level > 0 {
			t.Fatal("timeout also built settlement")
		}
	}
}
