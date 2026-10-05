package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// The creation catalog remains closed. Provision the waiting-room setting,
// then use formal readiness/start and real actions/autoplay/timeouts through victory.
func TestCatanCitiesKnightsConfiguredFullHTTPGames(t *testing.T) {
	testCatanCitiesKnightsConfiguredFullHTTPGames(t, "")
}

func TestCatanCitiesKnightsSeafarersConfiguredFullHTTPGames(t *testing.T) {
	for _, scenario := range []string{"shores", "islands", "fog", "desert", "new_world", "wonders"} {
		t.Run(scenario, func(t *testing.T) { testCatanCitiesKnightsConfiguredFullHTTPGames(t, scenario) })
	}
}

func testCatanCitiesKnightsConfiguredFullHTTPGames(t *testing.T, scenario string) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, n+1)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("骑士玩家%d", p))
			}
			options := game.CatanOptions{FiveSix: n > 4}
			r := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "内部城市骑士验证", "capacity": n, "catanOptions": options}, 201)
			id := r["id"].(string)
			for p := 1; p < n; p++ {
				clients[p].command(current(clients[0]), "join", nil, 200)
			}
			if scenario != "" {
				provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: scenario})
			}
			provisionCatanCitiesKnights(t, s, id)
			for p := 0; p < n; p++ {
				clients[p].command(current(clients[0]), "ready", nil, 200)
			}
			clients[0].command(current(clients[0]), "start", nil, 200)
			state := s.rooms[id].Game
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
					t.Fatal("live city state changed on restart")
				}
				s, ts = next, nextHTTP
				for _, c := range clients {
					c.base = ts.URL
				}
			}
			restart()
			restartedRoll, restartedResponse := false, false
			steps, automatic, timeouts := 0, 0, 0
			for ; steps < 8000 && !s.rooms[id].Game.Finished; steps++ {
				room := s.rooms[id]
				state = room.Game
				g := state.Catan
				if (g.RollID >= 4 && !restartedRoll) || (state.CatanPendingActor() >= 0 && !restartedResponse) {
					if g.RollID >= 4 {
						restartedRoll = true
					}
					if state.CatanPendingActor() >= 0 {
						restartedResponse = true
					}
					restart()
					room = s.rooms[id]
					state = room.Game
					g = state.Catan
				}
				actor := state.Turn
				if p := state.CatanPendingActor(); p >= 0 {
					actor = p
				} else if state.Phase == "catan_discard" {
					for p, due := range g.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				if steps%53 == 0 {
					for _, viewer := range []int{actor, n} {
						v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
						k := v["citiesKnights"].(map[string]any)
						if _, ok := k["progressDecks"]; ok {
							t.Fatal("hidden deck leaked")
						}
						for p, raw := range k["players"].([]any) {
							_, ok := raw.(map[string]any)["progress"]
							if ok != (p == viewer) {
								t.Fatal("progress hand leaked")
							}
						}
						for p, raw := range v["players"].([]any) {
							_, ok := raw.(map[string]any)["resources"]
							if ok != (p == viewer) {
								t.Fatal("resource hand leaked")
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
						t.Fatal("autoplay failed to advance", state.Phase)
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
			if !room.Game.Finished || room.Status != "finished" || len(room.Game.Winners) != 1 || !restartedRoll || automatic == 0 || timeouts == 0 {
				t.Fatal("incomplete city game", steps, room.Game.Phase)
			}
			winner := room.Game.Winners[0]
			target := 13
			if sea := room.Game.Catan.Seafarers; sea != nil {
				target = sea.VictoryPoints
			}
			if (scenario != "wonders" && room.Game.Catan.Players[winner].Score < target) || room.Game.Catan.CitiesKnights.Invasions == 0 {
				t.Fatal("wrong expansion victory")
			}
			if scenario == "wonders" {
				level, rival := 0, 0
				for _, card := range room.Game.Catan.Seafarers.Wonders.Cards {
					if card.Owner == winner {
						level = card.Level
					} else if card.Owner >= 0 {
						rival = max(rival, card.Level)
					}
				}
				if target != 12 || (level != 4 && (level <= rival || room.Game.Catan.Players[winner].Score < target)) {
					t.Fatal("wrong combined wonder victory")
				}
			}
			code, profile := clients[n].request("GET", "/api/players/"+room.Seats[winner].ID, nil)
			if code != 200 {
				t.Fatal(code)
			}
			stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
			history := profile["history"].([]any)[0].(map[string]any)
			if sea := room.Game.Catan.Seafarers; sea != nil {
				versions := history["catanExpansionRules"].(map[string]any)
				if history["catanScenario"] != sea.Scenario || history["catanLayout"] != sea.Layout || history["catanRules"] != sea.Rules || versions["seafarers"] != sea.Rules || versions["cities_knights"] != room.Game.Catan.CitiesKnightsSetup().Rules || len(history["catanExpansions"].([]any)) != 2 {
					t.Fatal("missing combined frozen identity", history)
				}
			} else if history["catanLayout"] != "variable" || history["catanRules"] != room.Game.Catan.CitiesKnightsSetup().Rules || history["catanExpansions"].([]any)[0] != "cities_knights" {
				t.Fatal("missing frozen expansion identity")
			}
			if stats["wins"] != float64(1) || stats["played"] != float64(1) {
				t.Fatal("missing history result")
			}
			clients[n].post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
			clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
			t.Logf("steps=%d autoplay=%d timeout=%d responseRestart=%v", steps, automatic, timeouts, restartedResponse)
		})
	}
}
