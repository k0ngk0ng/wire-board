package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanExplorerTwoKnightsNaturalHTTP(t *testing.T) {
	for i, scenario := range []string{"land-ho", "pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
		t.Run(scenario, func(t *testing.T) {
			helpers, events, fish := i%2 == 0, i%2 == 1 || i == 4, i >= 2
			s, ts, clients, id := newPublicExplorerOptionsHTTP(t, 2, scenario, true, events, game.CatanOptions{Helpers: helpers, AllHelpers: helpers}, fish, fish)
			restored, timedout, auto := map[string]bool{}, false, false
			seen := map[string]int{}
			for step := 0; step < 16000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				state := r.Game
				p := state.CatanPendingActor()
				if p < 0 {
					p = explorerHTTPActor(state)
				}
				if state.Catan.Two != nil || state.Catan.Paired != nil || state.Catan.Explorer.Board.TwoKnights != game.CatanExplorerTwoKnightsRules {
					t.Fatal("wrong controller or recipe")
				}
				label := state.Phase
				if q := state.Catan.CitiesKnights.Pending; q != nil {
					label += "/" + q.Kind
				}
				if !restored[label] {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored[label] = true
					r = s.rooms[id]
					state = r.Game
				}
				if !timedout && state.Phase == "catan_roll" && state.Catan.RollID > 0 {
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(r.TurnDeadline))
					s.mu.Unlock()
					if !s.rooms[id].Seats[p].TimeoutAutoPlay {
						t.Fatal("timeout not persistent")
					}
					reclaimTimeoutHumans(t, s, clients, id)
					timedout = true
					continue
				}
				if !auto && state.Phase == "catan_explorer_move" {
					setAutoPlay(clients[p], current(clients[p]), true, 200)
					version := s.rooms[id].Version
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("autoplay stalled")
					}
					setAutoPlay(clients[p], current(clients[p]), false, 200)
					auto = true
					continue
				}
				a, err := state.BotAction(p)
				if err != nil {
					t.Fatal(step, state.Phase, err)
				}
				req := map[string]any{"type": "action", "action": a, "version": r.Version, "nonce": randomID(12)}
				clients[p].post("/api/rooms/"+id, req, 200)
				seen[a.Type]++
				if step%151 == 0 {
					assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || len(r.Game.Winners) != 1 || !timedout || !auto || seen["catan_explorer_setup"] != 12 || seen["catan_roll"] == 0 || seen["catan_explorer_sail"] == 0 {
				t.Fatal("incomplete full game", r.Game.Round, seen, timedout, auto)
			}
			assertPublicExplorerHistory(t, s, clients, id)
			_, profile := clients[2].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			rules := profile["history"].([]any)[0].(map[string]any)["catanExpansionRules"].(map[string]any)
			if rules["explorer_two_knights"] != game.CatanExplorerTwoKnightsRules {
				t.Fatal("missing history supplement")
			}
			t.Log("natural complete", r.Game.Round, "rounds", len(restored), "restart states", seen)
		})
	}
}

func TestCatanExplorerTwoKnightsPublicConfiguration(t *testing.T) {
	for _, capacity := range []int{2, 6} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, g := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("双人探索骑士房主")
			g.register("双人探索骑士客人")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "双人探索骑士", "capacity": capacity, "catanScenario": "land-ho", "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "catanOptions": game.CatanOptions{Helpers: true}}, 201)
			id := raw["id"].(string)
			g.command(current(h), "join", nil, 200)
			change := func(c *testClient, value any, status int) {
				c.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			h.command(current(h), "ready", nil, 200)
			g.command(current(g), "ready", nil, 200)
			before, _ := json.Marshal(s.rooms[id])
			change(g, nil, 400)
			change(h, game.CatanCitiesKnightsSetup{Layout: "fixed"}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid toggle mutated room")
			}
			change(h, game.CatanCitiesKnightsSetup{}, 200)
			if !s.rooms[id].Seats[1].Ready {
				t.Fatal("same options reset readiness")
			}
			change(h, nil, 200)
			if s.rooms[id].Seats[1].Ready {
				t.Fatal("new options retained readiness")
			}
			change(h, game.CatanCitiesKnightsSetup{}, 200)
			h.command(current(h), "ready", nil, 200)
			g.command(current(g), "ready", nil, 200)
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, g}, id)
			h.command(current(h), "start", nil, 200)
			board := s.rooms[id].Game.Catan.Explorer.Board
			if board.Players != 2 || board.TwoKnights != game.CatanExplorerTwoKnightsRules || s.rooms[id].Game.Catan.Paired != nil {
				t.Fatal("used capacity instead of actual seats")
			}
			change(h, nil, 400)
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			id = current(h)["id"].(string)
			if s.rooms[id].CatanCitiesKnights == nil {
				t.Fatal("rematch lost city setting")
			}
			change(h, nil, 200)
			h.command(current(h), "ready", nil, 200)
			g.command(current(g), "ready", nil, 200)
			h.command(current(h), "start", nil, 200)
			state := s.rooms[id].Game
			if state.Catan.CitiesKnights != nil || state.Catan.Explorer.Board.TwoKnights != "" || state.Catan.Explorer.Board.Target != 8 || state.Catan.Explorer.Board.Layout != "fixed" {
				t.Fatal("base isolation")
			}
		})
	}
}
