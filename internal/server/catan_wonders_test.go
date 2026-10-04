package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanWondersHTTPClaimsClockPrivacyAndRestart(t *testing.T) {
	s, ts, clients, id := newCatanTable(t)
	clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	scenario, err := game.NewCatanWonders(3, game.CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g := scenario.Catan
	g.SetupStep, scenario.Turn, scenario.Phase = g.SetupLimit(), 0, "catan_turn"
	// A built settlement at a printed wall site qualifies without any cost.
	for _, m := range g.Seafarers.Wonders.Markers {
		if m.Card == 4 {
			g.Vertices[m.Vertex].Owner, g.Vertices[m.Vertex].Level = 0, 1
			break
		}
	}
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = scenario
	deadline := time.Now().Add(45 * time.Second).UnixMilli()
	r.TurnDeadline = deadline
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	for _, viewer := range []int{1, 3} {
		clients[viewer].command(current(clients[viewer]), "action", game.Action{Type: "catan_wonder_claim", Card: 4}, 400)
	}
	clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_wonder_claim", Card: 4}, 200)
	clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_wonder_claim", Card: 3}, 400)
	clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_wonder_build", Card: 4}, 400)
	r = s.rooms[id]
	if r.TurnDeadline != deadline || r.Game.Catan.Seafarers.Wonders.Cards[4].Owner != 0 || r.Game.Catan.Seafarers.Wonders.Cards[4].Level != 0 {
		t.Fatal("claim or rejected action changed phase/clock/level")
	}
	before, _ := json.Marshal(r)
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	after, _ := json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("unbuilt claimed wonder/clock lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	for _, c := range clients {
		c.base = ts2.URL
	}
	for level := 1; level <= 4; level++ {
		next.mu.Lock()
		g = next.rooms[id].Game.Catan
		for color, amount := range []int{1, 3, 0, 1, 0} {
			g.Bank[color] -= amount
			g.Players[0].Resources[color] += amount
		}
		next.mu.Unlock()
		for _, viewer := range []int{0, 1, 3} {
			v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
			if (len(v["wonderBuilds"].([]any)) > 0) != (viewer == 0) {
				t.Fatal("private build availability leaked", viewer)
			}
			for player, raw := range v["players"].([]any) {
				_, resources := raw.(map[string]any)["resources"]
				if resources != (viewer == player) {
					t.Fatal("private resources leaked")
				}
			}
		}
		clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_wonder_build", Card: 4}, 200)
		r = next.rooms[id]
		if r.Game.Catan.Seafarers.Wonders.Cards[4].Level != level || r.Game.Finished != (level == 4) || (level < 4 && r.TurnDeadline != deadline) {
			t.Fatal("building reset clock or failed immediate victory", level)
		}
	}
	if next.rooms[id].Status != "finished" || next.rooms[id].Game.Winners[0] != 0 {
		t.Fatal("wonder completion not reflected in room status")
	}
}

// Only inject the official initial scenario while public room selection is
// closed. Every subsequent move uses HTTP, real autoplay, or timeout handling.
func TestCatanWondersFullHTTPGames(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, helpers := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/helpers=%v", n, helpers), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for i := range clients {
					clients[i] = newClient(t, ts.URL)
					clients[i].register(fmt.Sprintf("奇迹玩家%d", i))
				}
				options := game.CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}
				raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "卡坦奇迹联机验证", "capacity": n, "catanOptions": options}, 201)
				id := raw["id"].(string)
				for i := 1; i < n; i++ {
					clients[i].command(current(clients[0]), "join", nil, 200)
				}
				for i := 0; i < n; i++ {
					clients[i].command(current(clients[0]), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				scenario, err := game.NewCatanWonders(n, options)
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
					built := false
					for _, c := range room.Game.Catan.Seafarers.Wonders.Cards {
						built = built || c.Level > 0
					}
					if built && !restarted {
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
							t.Fatal("partial wonder and clock changed on restart")
						}
						s, ts, room = next, nextHTTP, next.rooms[id]
						for _, c := range clients {
							c.base = ts.URL
						}
						restarted = true
					}
					state, g := room.Game, room.Game.Catan
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
				var own game.CatanWonder
				for _, c := range room.Game.Catan.Seafarers.Wonders.Cards {
					if c.Owner == winner {
						own = c
					}
				}
				if own.Owner != winner || own.Level == 0 {
					t.Fatal("winner has no built wonder")
				}
				if own.Level < 4 {
					if room.Game.Catan.Players[winner].Score < 10 {
						t.Fatal("unfinished wonder winner below ten points")
					}
					for _, other := range room.Game.Catan.Seafarers.Wonders.Cards {
						if other.Owner >= 0 && other.Owner != winner && other.Level >= own.Level {
							t.Fatal("winning wonder does not lead")
						}
					}
				}
				code, profile := clients[n].request("GET", "/api/players/"+room.Seats[winner].ID, nil)
				if code != 200 || profile["stats"].(map[string]any)["catan"].(map[string]any)["wins"] != float64(1) {
					t.Fatal("wonder victory absent from history")
				}
				t.Logf("steps=%d autoplay=%d timeouts=%d winner=%d wonder=%d level=%d", steps, automatic, timeouts, winner, own.ID, own.Level)
			})
		}
	}
}
