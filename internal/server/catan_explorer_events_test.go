package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanExplorerEventsNaturalHTTPGames(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		n        int
		city     bool
	}{
		{"land-ho", 2, false}, {"land-ho", 4, false},
		{"pirate-lairs", 2, false}, {"pirate-lairs", 5, false}, {"pirate-lairs", 3, true},
		{"fish-for-catan", 2, false}, {"fish-for-catan", 6, true},
		{"spices-for-catan", 6, false}, {"spices-for-catan", 3, true},
		{"explorers-and-pirates", 2, false}, {"explorers-and-pirates", 6, true},
	} {
		t.Run(fmt.Sprintf("%s/%d/city=%v", tc.scenario, tc.n, tc.city), func(t *testing.T) {
			s, ts, clients, id := newPublicExplorerRecipeHTTP(t, tc.n, tc.scenario, tc.city, true)
			seen := map[string]int{}
			restored := map[string]bool{}
			timedout, automatic := false, false
			for step := 0; step < 14000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				state := r.Game
				p := state.CatanPendingActor()
				if p < 0 {
					p = explorerHTTPActor(state)
				}
				if state.Phase == "catan_roll" && state.Catan.RollID > 0 && !timedout {
					before := state.Catan.RollID
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(r.TurnDeadline))
					s.mu.Unlock()
					if !s.rooms[id].Seats[p].AutoPlay || s.rooms[id].Game.Catan.RollID != before+1 {
						t.Fatal("timeout did not persist takeover and draw once")
					}
					reclaimTimeoutHumans(t, s, clients, id)
					timedout = true
					continue
				}
				if state.Phase == "catan_roll" && timedout && !automatic {
					before := state.Catan.RollID
					setAutoPlay(clients[p], current(clients[p]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Game.Catan.RollID != before+1 {
						t.Fatal("autoplay did not produce once")
					}
					setAutoPlay(clients[p], current(clients[p]), false, 200)
					automatic = true
					continue
				}
				a, err := state.BotAction(p)
				if err != nil {
					t.Fatal(step, state.Phase, err)
				}
				request := map[string]any{"type": "action", "action": a, "version": r.Version, "nonce": randomID(12)}
				clients[p].post("/api/rooms/"+id, request, 200)
				seen[a.Type]++
				r = s.rooms[id]
				if !restored[a.Type] && (a.Type == "catan_roll" || a.Type == "catan_explorer_begin_move" || a.Type == "catan_pillage" || a.Type == "catan_aqueduct") {
					before, _ := json.Marshal(r)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					clients[p].post("/api/rooms/"+id, request, 200)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("restart/replay repeated production or movement")
					}
					restored[a.Type] = true
				}
				if step%131 == 0 && !s.rooms[id].Game.Finished {
					assertPublicEventsPrivacy(t, clients)
					if tc.city {
						assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
					} else {
						assertExplorerHTTPPrivacy(t, clients)
					}
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || r.Status != "finished" || r.TurnDeadline != 0 || !timedout || !automatic || !restored["catan_roll"] || seen["catan_explorer_sail"] == 0 {
				t.Fatal("incomplete natural game", r.Game.Phase, seen)
			}
			if r.Game.Catan.Players[r.Game.Winners[0]].Score < r.Game.Catan.Explorer.Board.Target {
				t.Fatal("premature victory")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			assertPublicExplorerHistory(t, s, clients, id)
			_, profile := clients[tc.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			rules := profile["history"].([]any)[0].(map[string]any)["catanExpansionRules"].(map[string]any)
			if rules["event_cards"] != game.CatanEventCatalogue || rules["event_explorer"] != game.CatanEventExplorerRules {
				t.Fatal("event combination history missing", rules)
			}
			t.Log("natural complete game", "rounds", r.Game.Round, "rolls", r.Game.Catan.RollID, "actions", seen)
		})
	}
}

func TestCatanExplorerEventsConfigurationAndBaseIsolation(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("探险事件房主")
	guest.register("探险事件朋友")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "capacity": 3, "name": "探险事件设置", "catanScenario": "land-ho", "catanEvents": game.CatanEventCatalogue}, 201)
	id := raw["id"].(string)
	guest.command(current(host), "join", nil, 200)
	change := func(c *testClient, typ, key string, value any, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": typ, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	change(guest, "catan_events", "enabled", false, 400)
	change(host, "catan_events", "enabled", true, 200)
	if !s.rooms[id].Seats[0].Ready {
		t.Fatal("no-op reset ready")
	}
	change(host, "catan_events", "enabled", false, 200)
	if s.rooms[id].Seats[0].Ready || s.rooms[id].Seats[1].Ready {
		t.Fatal("changed option kept ready")
	}
	change(host, "catan_events", "enabled", true, 200)
	for _, scene := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates", "land-ho"} {
		change(host, "catan_scenario", "catanScenario", scene, 200)
		if s.rooms[id].CatanEvents != game.CatanEventCatalogue {
			t.Fatal("mission switch lost event preference")
		}
	}
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	if s.rooms[id].Game.Catan.EventDeck == nil || len(s.rooms[id].Game.Catan.Players) != 2 {
		t.Fatal("actual count/event configuration lost")
	}
	change(host, "catan_events", "enabled", false, 400)
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	change(host, "catan_events", "enabled", false, 200)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game
	if g.Catan.EventDeck != nil || g.Catan.Two != nil {
		t.Fatal("plain explorer polluted")
	}
	p := g.Turn
	a, err := g.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	([]*testClient{host, guest})[p].command(current(([]*testClient{host, guest})[p]), "action", a, 200)
	g = s.rooms[id].Game
	if g.Catan.Dice[0] < 1 || g.Catan.Dice[1] < 1 || g.Catan.RevealedEvent != nil {
		t.Fatal("disabled event replaced original dice")
	}
}
