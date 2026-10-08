package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func newHelpersKnightsHTTP(t *testing.T, n int, scenario string, events bool, fishing ...bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("骑士助手玩家%d", p))
	}
	recipe := map[string]any{"kind": "catan", "name": "骑士助手", "capacity": n, "catanScenario": scenario, "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: true}}
	if scenario != "cities-knights" {
		recipe["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
	}
	if len(fishing) > 0 && fishing[0] && scenario != "fishing" {
		recipe["catanFishing"] = true
	}
	if events {
		recipe["catanEvents"] = game.CatanEventCatalogue
	}
	raw := clients[0].post("/api/rooms", recipe, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[p]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	ordered := make([]*testClient, n+1)
	for _, c := range clients[:n] {
		ordered[int(current(c)["you"].(float64))] = c
	}
	ordered[n] = clients[n]
	ordered[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, ordered, id
}

func TestCatanHelpersKnightsHTTPConfiguration(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("骑士助手房主")
			guest.register("骑士助手客人")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "助手组合设置", "capacity": n, "catanScenario": "cities-knights", "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: true}}, 201)
			id := raw["id"].(string)
			guest.command(raw, "join", nil, 200)
			change := func(c *testClient, typ, key string, value any, status int) {
				c.post("/api/rooms/"+id, map[string]any{"type": typ, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			for _, scene := range []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "pirate_islands", "wonders", "new_world", "cities-knights"} {
				change(h, "catan_scenario", "catanScenario", scene, 200)
				if scene != "cities-knights" {
					change(h, "catan_cities_knights", "catanCitiesKnights", game.CatanCitiesKnightsSetup{}, 200)
				}
				h.command(current(h), "ready", nil, 200)
				guest.command(current(guest), "ready", nil, 200)
				before, _ := json.Marshal(s.rooms[id])
				change(guest, "catan_options", "catanOptions", game.CatanOptions{FiveSix: n > 4}, 400)
				if scene == "pirate_islands" {
					change(h, "catan_fishing", "enabled", true, 400)
				}
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid recipe mutated state")
				}
				change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: n > 4}, 200)
				if s.rooms[id].Seats[1].Ready {
					t.Fatal("changed helpers kept ready")
				}
				change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: true}, 200)
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
			for len(s.rooms[id].Seats) < n {
				h.command(current(h), "add_bot", nil, 200)
			}
			h.command(current(h), "ready", nil, 200)
			guest.command(current(guest), "ready", nil, 200)
			h.command(current(h), "start", nil, 200)
			if s.rooms[id].Game.Catan.CitiesKnights.Helpers.Rules != game.CatanHelpersKnightsRules {
				t.Fatal("missing marker")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if !s.rooms[id].CatanOptions.Helpers {
				t.Fatal("rematch lost helpers")
			}
			change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: n > 4}, 200)
			for _, c := range []*testClient{h, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			h.command(current(h), "start", nil, 200)
			if s.rooms[id].Game.Catan.CitiesKnights.Helpers != nil || len(s.rooms[id].Game.Catan.HelperDisplay) != 0 {
				t.Fatal("disabled helpers survived")
			}
		})
	}
}

func TestCatanHelpersKnightsHTTPProgressPrivacyRecoveryAndClock(t *testing.T) {
	s, ts, clients, id := newHelpersKnightsHTTP(t, 3, "cities-knights", false)
	testHelpersKnightsHTTPProgressClock(t, s, ts, clients, id)
}

func testHelpersKnightsHTTPProgressClock(t *testing.T, s *Server, ts *httptest.Server, clients []*testClient, id string) {
	t.Helper()
	for s.rooms[id].Game.Phase == "catan_setup_settlement" || s.rooms[id].Game.Phase == "catan_setup_road" || s.rooms[id].Game.Phase == "catan_setup_city" {
		r := s.rooms[id]
		a, e := r.Game.BotAction(r.Game.Turn)
		if e != nil {
			t.Fatal(e)
		}
		clients[r.Game.Turn].command(current(clients[r.Game.Turn]), "action", a, 200)
	}
	r := s.rooms[id]
	g := r.Game.Catan
	p := r.Game.Turn
	// Give Diara to the active seat without changing helper inventory.
	old := g.Players[p].Helper.ID
	if at := slices.Index(g.HelperDisplay, 6); at >= 0 {
		g.HelperDisplay[at] = old
	} else {
		for i := range g.Players {
			if g.Players[i].Helper.ID == 6 {
				g.Players[i].Helper.ID = old
			}
		}
	}
	g.TurnSerial = 10
	g.Players[p].Helper = &game.CatanHelperSeat{ID: 6, AcquiredTurn: 1}
	for c := 2; c < 5; c++ {
		g.Players[p].Resources[c]++
		g.Bank[c]--
	}
	r.Game.Phase = "catan_turn"
	if g.Two != nil {
		g.Two.Rolls = []int{2, 3}
		g.RollID = 2
		g.Dice = []int{1, 2}
		g.CitiesKnights.EventDie = 0
	}
	r.TurnDeadline = time.Now().Add(41 * time.Second).UnixMilli()
	if e := s.save(r); e != nil {
		t.Fatal(e)
	}
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_helper", Choice: "progress_buy", Color: 0, Tokens: []int{0, 0, 1, 1, 1}}, 200)
	r = s.rooms[id]
	budget := r.CatanTimeLeft
	deadline := r.TurnDeadline
	if budget < 39000 || budget > 41000 || r.Game.Catan.HelperPending.Kind != "progress" {
		t.Fatal("helper did not pause active clock", budget)
	}
	before, _ := json.Marshal(r)
	clients[(p+1)%len(g.Players)].command(current(clients[(p+1)%len(g.Players)]), "action", game.Action{Type: "catan_helper_choice", Card: 0}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("wrong actor mutated progress")
	}
	for i, c := range clients {
		view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		pending := view["helperPending"].(map[string]any)
		if (pending["cards"] != nil) != (i == p) {
			t.Fatal("private candidates leaked", i)
		}
		if view["citiesKnights"].(map[string]any)["progressDecks"] != nil {
			t.Fatal("private deck leaked")
		}
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	r = s.rooms[id]
	if r.TurnDeadline != deadline || r.CatanTimeLeft != budget {
		t.Fatal("restart lost timer")
	}
	selected := r.Game.Catan.HelperPending.Cards[0]
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_helper_choice", Card: selected}, 200)
	if s.rooms[id].TurnDeadline != deadline {
		t.Fatal("choice refreshed helper clock")
	}
	// Timeout resolves the remaining exchange and turns on automatic play.
	s.mu.Lock()
	r = s.rooms[id]
	r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	r.BotAt = 0
	s.expireSetups(time.Now())
	r = s.rooms[id]
	if !r.Seats[p].AutoPlay || r.Game.Catan.HelperPending != nil || r.Game.Phase != "catan_turn" {
		t.Fatal("timeout failed to resolve helper")
	}
	if left := r.TurnDeadline - time.Now().UnixMilli(); left > budget || left < budget-1500 {
		t.Fatal("active clock not resumed", left, budget)
	}
	s.mu.Unlock()
}

func TestCatanHelpersKnightsNaturalHTTPGames(t *testing.T) {
	for _, tc := range []struct {
		n      int
		scene  string
		events bool
	}{{3, "cities-knights", false}, {6, "cities-knights", true}, {3, "tribe", true}, {5, "cloth", false}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			s, ts, clients, id := newHelpersKnightsHTTP(t, tc.n, tc.scene, tc.events)
			restored, responses := false, 0
			for step := 0; step < 15000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				p := r.Game.CatanPendingActor()
				if p < 0 {
					p = explorerHTTPActor(r.Game)
				}
				a, e := r.Game.BotAction(p)
				if e != nil {
					t.Fatal(step, r.Game.Phase, e)
				}
				if a.Type == "catan_helper_choice" {
					responses++
				}
				clients[p].command(current(clients[p]), "action", a, 200)
				if !restored && responses > 0 {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored = true
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || !restored {
				t.Fatal("incomplete natural match", r.Game.Round, responses)
			}
			_, profile := clients[tc.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			if record["catanExpansionRules"].(map[string]any)["helpers_knights"] != game.CatanHelpersKnightsRules {
				t.Fatal("history lost adaptation")
			}
			t.Log("completed", r.Game.Round, "rounds", responses, "helper responses")
		})
	}
}
