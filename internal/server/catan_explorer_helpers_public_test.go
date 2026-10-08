package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanExplorerHelpersPublicConfiguration(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("助手房主")
			guest.register("助手朋友")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "探险助手", "capacity": n, "catanScenario": "land-ho", "catanOptions": game.CatanOptions{Helpers: true, AllHelpers: true}}, 201)
			id := raw["id"].(string)
			guest.command(current(h), "join", nil, 200)
			change := func(c *testClient, typ, key string, value any, status int) {
				c.post("/api/rooms/"+id, map[string]any{"type": typ, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			for _, c := range []*testClient{h, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			change(h, "catan_options", "catanOptions", game.CatanOptions{Helpers: true, AllHelpers: true}, 200)
			if !s.rooms[id].Seats[0].Ready {
				t.Fatal("same options reset readiness")
			}
			before, _ := json.Marshal(s.rooms[id])
			change(guest, "catan_options", "catanOptions", game.CatanOptions{}, 400)
			change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: true, Helpers: true}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("rejected options mutated room")
			}
			change(h, "catan_options", "catanOptions", game.CatanOptions{Helpers: true}, 200)
			if s.rooms[id].Capacity != n || s.rooms[id].Seats[1].Ready {
				t.Fatal("helper toggle changed capacity or kept readiness")
			}
			for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates", "land-ho"} {
				change(h, "catan_scenario", "catanScenario", scenario, 200)
				if !s.rooms[id].CatanOptions.Helpers {
					t.Fatal("scenario lost helpers")
				}
			}
			change(h, "catan_fishing", "enabled", true, 200)
			change(h, "catan_fishing_lakes", "enabled", true, 200)
			if n > 2 {
				change(h, "catan_cities_knights", "catanCitiesKnights", &game.CatanCitiesKnightsSetup{Layout: "variable"}, 200)
				change(h, "catan_cities_knights", "catanCitiesKnights", nil, 200)
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
			for _, c := range []*testClient{h, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if g.Explorer.Helpers == nil || g.Explorer.Helpers.Rules != game.CatanExplorerHelpersRules || len(g.Players) != 2 || len(g.HelperDisplay) != 2 || g.Options.FiveSix {
				t.Fatal("wrong actual-seat initialization")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if !s.rooms[id].CatanOptions.Helpers {
				t.Fatal("rematch lost helpers")
			}
			change(h, "catan_options", "catanOptions", game.CatanOptions{}, 200)
			if n == 2 {
				change(h, "catan_two_scenario", "catanTwoScenario", "", 200)
			}
			for _, c := range []*testClient{h, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			h.command(current(h), "start", nil, 200)
			g = s.rooms[id].Game.Catan
			if g.Options.Helpers || len(g.HelperDisplay) > 0 || g.HelperPending != nil || n == 2 && g.Explorer != nil {
				t.Fatal("disabled helpers survived restart")
			}
		})
	}
}

func TestCatanExplorerHelpersHTTPProductionClock(t *testing.T) {
	s, ts, clients, id := newPublicExplorerOptionsHTTP(t, 3, "land-ho", true, false, game.CatanOptions{Helpers: true, AllHelpers: true})
	r := s.rooms[id]
	for step := 0; r.Game.Catan.Explorer.Setup != nil && step < 80; step++ {
		a, err := r.Game.BotAction(r.Game.Turn)
		if err != nil {
			t.Fatal(err)
		}
		clients[r.Game.Turn].command(current(clients[r.Game.Turn]), "action", a, 200)
		r = s.rooms[id]
	}
	g := r.Game.Catan
	actor := r.Game.Turn
	hilda := -1
	for p := range g.Players {
		if g.Players[p].Helper.ID == 3 {
			hilda = p
		}
	}
	// Legal controlled production fixture: no producing buildings, science 3 and
	// Alchemist to roll 2. Complete natural matches below do not inject state.
	for i := range g.Vertices {
		g.Vertices[i].Owner = -1
		g.Vertices[i].Level = 0
		g.Vertices[i].Harbor = false
	}
	for i := range g.Edges {
		g.Edges[i].Owner = -1
	}
	for p := range g.Players {
		g.Players[p].Score = 0
	}
	g.CitiesKnights.Players[hilda].Improvements[game.CatanScience] = 3
	for track, deck := range g.CitiesKnights.ProgressDecks {
		if i := slices.Index(deck, 0); i >= 0 {
			g.CitiesKnights.ProgressDecks[track] = slices.Delete(deck, i, i+1)
		}
	}
	g.CitiesKnights.Players[actor].Progress = append(g.CitiesKnights.Players[actor].Progress, 0)
	r.TurnDeadline = time.Now().Add(37 * time.Second).UnixMilli()
	clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: int(g.TurnSerial)}, 200)
	r = s.rooms[id]
	if r.Game.Phase != "catan_helper" || r.Game.CatanPendingActor() != hilda || r.CatanTimeLeft <= 0 || r.CatanTimeLeft > 37000 {
		t.Fatal("helper failed to pause budget", r.Game.Phase, r.CatanTimeLeft)
	}
	budget, deadline := r.CatanTimeLeft, r.TurnDeadline
	clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_helper_choice", Color: 0, Prompt: int(g.TurnSerial)}, 400)
	req := map[string]any{"type": "action", "action": game.Action{Type: "catan_helper_choice", Color: 0, Prompt: int(g.TurnSerial)}, "version": r.Version, "nonce": randomID(12)}
	clients[hilda].post("/api/rooms/"+id, req, 200)
	r = s.rooms[id]
	if r.Game.Catan.HelperPending.Kind != "exchange" || r.TurnDeadline != deadline || r.CatanTimeLeft != budget {
		t.Fatal("follow-up exchange reset clock")
	}
	before, _ := json.Marshal(r)
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	clients[hilda].post("/api/rooms/"+id, req, 200)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("helper request replayed after restart")
	}
	s.mu.Lock()
	s.expireSetups(time.UnixMilli(deadline))
	s.mu.Unlock()
	r = s.rooms[id]
	if !r.Seats[hilda].AutoPlay || r.Game.Phase != "catan_aqueduct" || r.CatanTimeLeft != budget {
		t.Fatal("timeout failed to continue", r.Game.Phase)
	}
	// Persistent autoplay remains on across the next required response and save.
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	if !s.rooms[id].Seats[hilda].AutoPlay {
		t.Fatal("autoplay lost on restart")
	}
	s.mu.Lock()
	s.expireSetups(time.UnixMilli(s.rooms[id].TurnDeadline))
	s.mu.Unlock()
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || !r.Seats[hilda].AutoPlay || r.CatanTimeLeft != 0 {
		t.Fatal("aqueduct did not resume construction", r.Game.Phase)
	}
}

func TestCatanExplorerHelpersNaturalHTTPGames(t *testing.T) {
	for _, tc := range []struct {
		n                       int
		scenario                string
		city, fish, events, all bool
	}{
		{2, "land-ho", false, false, true, false},
		{5, "pirate-lairs", false, true, true, true},
		{3, "explorers-and-pirates", true, true, false, true},
	} {
		t.Run(fmt.Sprintf("%d/%s", tc.n, tc.scenario), func(t *testing.T) {
			s, ts, clients, id := newPublicExplorerOptionsHTTP(t, tc.n, tc.scenario, tc.city, tc.events, game.CatanOptions{Helpers: true, AllHelpers: tc.all}, tc.fish, tc.fish)
			restored, responses := false, 0
			for step := 0; step < 18000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				p := r.Game.CatanPendingActor()
				if p < 0 {
					p = explorerHTTPActor(r.Game)
				}
				a, err := r.Game.BotAction(p)
				if err != nil {
					t.Fatal(step, r.Game.Phase, err)
				}
				if a.Type == "catan_helper_choice" {
					responses++
				}
				clients[p].command(current(clients[p]), "action", a, 200)
				if !restored && responses > 0 {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored = true
				}
				if step%137 == 0 {
					view := current(clients[tc.n])["game"].(map[string]any)["catan"].(map[string]any)
					if view["explorer"].(map[string]any)["helperRules"] != game.CatanExplorerHelpersRules {
						t.Fatal("missing helper rules")
					}
					if q, ok := view["helperPending"].(map[string]any); ok && q["resources"] != nil {
						t.Fatal("spectator saw private helper hand")
					}
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || !restored || responses == 0 {
				t.Fatal("incomplete helper match", r.Game.Round, responses)
			}
			assertPublicExplorerHistory(t, s, clients, id)
			_, profile := clients[tc.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			if record["catanExpansionRules"].(map[string]any)["explorer_helpers"] != game.CatanExplorerHelpersRules || record["catanOptions"].(map[string]any)["helpers"] != true {
				t.Fatal("history lost helpers")
			}
			t.Log("completed", r.Game.Round, "rounds", responses, "helper responses")
		})
	}
}

func TestCatanExplorerHelpersHTTPSevenContinuesDiscard(t *testing.T) {
	s, _, clients, id := newPublicExplorerOptionsHTTP(t, 3, "land-ho", true, false, game.CatanOptions{Helpers: true, AllHelpers: true})
	r := s.rooms[id]
	for step := 0; r.Game.Catan.Explorer.Setup != nil && step < 80; step++ {
		a, e := r.Game.BotAction(r.Game.Turn)
		if e != nil {
			t.Fatal(e)
		}
		clients[r.Game.Turn].command(current(clients[r.Game.Turn]), "action", a, 200)
		r = s.rooms[id]
	}
	g := r.Game.Catan
	actor := r.Game.Turn
	protected, discarder := (actor+1)%3, (actor+2)%3
	old := g.Players[protected].Helper.ID
	for i, id := range g.HelperDisplay {
		if id == 5 {
			g.HelperDisplay[i] = old
		}
	}
	g.Players[protected].Helper.ID = 5
	for p := range g.Players {
		for c, n := range g.Players[p].Resources {
			g.Bank[c] += n
			g.Players[p].Resources[c] = 0
		}
	}
	for _, p := range []int{protected, discarder} {
		g.Players[p].Resources[0] = 5
		g.Players[p].Resources[1] = 5
		g.Bank[0] -= 5
		g.Bank[1] -= 5
	}
	for track, deck := range g.CitiesKnights.ProgressDecks {
		if i := slices.Index(deck, 0); i >= 0 {
			g.CitiesKnights.ProgressDecks[track] = slices.Delete(deck, i, i+1)
		}
	}
	g.CitiesKnights.Players[actor].Progress = append(g.CitiesKnights.Players[actor].Progress, 0)
	r.TurnDeadline = time.Now().Add(31 * time.Second).UnixMilli()
	clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_progress", Card: 0, Tokens: []int{3, 4}, Prompt: int(g.TurnSerial)}, 200)
	r = s.rooms[id]
	budget := r.CatanTimeLeft
	if r.Game.Phase != "catan_helper" || r.Game.CatanPendingActor() != protected || r.Game.Catan.DiscardDue[protected] != 0 || r.Game.Catan.DiscardDue[discarder] != 5 || budget <= 0 || budget > 31000 {
		t.Fatal("seven helper ordering failed", r.Game.Phase)
	}
	clients[protected].command(current(clients[protected]), "action", game.Action{Type: "catan_helper_choice", Choice: "flip", Prompt: int(g.TurnSerial)}, 200)
	r = s.rooms[id]
	if r.Game.Phase != "catan_discard" || r.CatanTimeLeft != budget {
		t.Fatal("discard lost paused clock")
	}
	a, e := r.Game.BotAction(discarder)
	if e != nil {
		t.Fatal(e)
	}
	clients[discarder].command(current(clients[discarder]), "action", a, 200)
	r = s.rooms[id]
	if r.Game.Phase != "catan_turn" || r.CatanTimeLeft != 0 || r.TurnDeadline-time.Now().UnixMilli() > budget {
		t.Fatal("seven did not restore construction budget", r.Game.Phase)
	}
}

func TestCatanExplorerHelpersScenarioSwitchIsolation(t *testing.T) {
	options, _ := game.NormalizeCatanOptions(game.CatanOptions{FiveSix: true, Helpers: true, AllHelpers: true})
	r := Room{Kind: "catan", Status: "waiting", Capacity: 6, CatanOptions: options}
	if e := r.setCatanScenario("land-ho"); e != nil {
		t.Fatal(e)
	}
	if r.Capacity != 6 || r.CatanOptions.FiveSix || !r.CatanOptions.Helpers || !r.CatanOptions.AllHelpers {
		t.Fatal("base player-count options leaked into explorer")
	}
	r.Capacity = 2
	if e := r.setCatanTwoScenario(""); e != nil {
		t.Fatal(e)
	}
	if r.CatanOptions != (game.CatanOptions{}) || r.CatanScenario != "" || r.CatanTwoRules != game.CatanTwoRules {
		t.Fatal("explorer helper options leaked into ordinary two-player game")
	}
}
