package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanExplorerFishingHTTPResponseClockAndTimeout(t *testing.T) {
	s, ts, clients, id := newPublicExplorerRecipeHTTP(t, 3, "explorers-and-pirates", true, false, true, true)
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
	// Explicit legal midgame fixture: two real cities on opposite lake banks,
	// full fish hands and Alchemist. Apply validates the full inventory before
	// producing; the separate natural HTTP tests never inject components.
	for i := range g.Vertices {
		g.Vertices[i].Owner, g.Vertices[i].Level, g.Vertices[i].Harbor = -1, 0, false
	}
	for i := range g.Edges {
		g.Edges[i].Owner = -1
	}
	for p := range g.Players {
		g.Players[p].Score = 0
	}
	lake := g.Tiles[g.Fishing.Map.Lakes[0].Tile]
	for p, corner := range []int{0, 3} {
		v := &g.Vertices[lake.Vertices[corner]]
		v.Owner, v.Level = p, 2
		g.Players[p].Score = 2
		g.CitiesKnights.Players[p].Improvements[game.CatanScience] = 3
	}
	f := &g.Fishing.Tokens
	f.Hands = [][]int{{0, 1, 2, 3, 4, 5, 6}, {7, 8, 9, 10, 11, 12, 13}, {}}
	f.DrawPile, f.Discard, f.BootOwner = []int{}, []int{}, -1
	for token := 14; token < 30; token++ {
		f.DrawPile = append(f.DrawPile, token)
	}
	actor := r.Game.Turn
	for track, deck := range g.CitiesKnights.ProgressDecks {
		if index := slices.Index(deck, 0); index >= 0 {
			g.CitiesKnights.ProgressDecks[track] = slices.Delete(deck, index, index+1)
		}
	}
	g.CitiesKnights.Players[actor].Progress = append(g.CitiesKnights.Players[actor].Progress, 0)
	r.TurnDeadline = time.Now().Add(37 * time.Second).UnixMilli()
	clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}, Prompt: int(g.TurnSerial)}, 200)
	r = s.rooms[id]
	if r.Game.Phase != "catan_fish_replace" || len(r.Game.Catan.Fishing.Tokens.Pending) != 2 || r.CatanTimeLeft <= 0 || r.CatanTimeLeft > 37000 {
		t.Fatal("fish response did not pause action budget", r.Game.Phase, r.CatanTimeLeft)
	}
	budget := r.CatanTimeLeft
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	r = s.rooms[id]
	p := r.Game.CatanPendingActor()
	wrong := (p + 1) % 3
	clients[wrong].command(current(clients[wrong]), "action", game.Action{Type: "catan_fish_keep", Prompt: int(r.Game.Catan.TurnSerial)}, 400)
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_fish_keep", Prompt: int(r.Game.Catan.TurnSerial)}, 200)
	r = s.rooms[id]
	second := r.Game.CatanPendingActor()
	if second == p || r.CatanTimeLeft != budget {
		t.Fatal("second fish responder lost paused budget")
	}
	s.mu.Lock()
	s.expireSetups(time.UnixMilli(r.TurnDeadline))
	s.mu.Unlock()
	r = s.rooms[id]
	if !r.Seats[second].AutoPlay || r.Game.Phase != "catan_aqueduct" || r.Game.Catan.Fishing.Pending != nil || r.CatanTimeLeft != budget {
		t.Fatal("fish timeout failed to continue to aqueduct", r.Game.Phase)
	}
	reclaimTimeoutHumans(t, s, clients, id)
	for i := 0; s.rooms[id].Game.Phase == "catan_aqueduct" && i < 3; i++ {
		r = s.rooms[id]
		p = r.Game.CatanPendingActor()
		a, err := r.Game.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
	r = s.rooms[id]
	left := r.TurnDeadline - time.Now().UnixMilli()
	if r.Game.Phase != "catan_turn" || r.CatanTimeLeft != 0 || left <= 0 || left > budget {
		t.Fatal("response chain gained or lost original action budget", r.Game.Phase, left, budget)
	}
}

func TestCatanExplorerFishingPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("探险渔夫房主")
	guest.register("探险渔夫朋友")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 3, "catanScenario": "land-ho", "catanFishingLakes": true},
		{"kind": "catan", "capacity": 3, "catanScenario": "islands", "catanFishing": true, "catanFishingLakes": true},
		{"kind": "catan", "capacity": 3, "catanScenario": "pirate-lairs", "catanFishing": true, "catanOptions": game.CatanOptions{AllHelpers: true}},
		{"kind": "catan", "capacity": 2, "catanScenario": "pirate-lairs", "catanFishing": true, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}},
	} {
		body["name"] = "错误捕鱼配置"
		h.post("/api/rooms", body, 400)
	}
	raw := h.post("/api/rooms", map[string]any{"kind": "catan", "capacity": 3, "name": "探索者捕鱼设置", "catanScenario": "land-ho", "catanFishing": true, "catanFishingLakes": true, "catanEvents": game.CatanEventCatalogue}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	change := func(c *testClient, typ, key string, v any, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": typ, key: v, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	for _, c := range []*testClient{h, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	change(guest, "catan_fishing_lakes", "enabled", false, 400)
	change(h, "catan_fishing_lakes", "enabled", true, 200)
	if !s.rooms[id].Seats[0].Ready {
		t.Fatal("no-op cleared readiness")
	}
	change(h, "catan_fishing_lakes", "enabled", false, 200)
	if s.rooms[id].Seats[0].Ready || s.rooms[id].Seats[1].Ready {
		t.Fatal("map changed without readiness reset")
	}
	change(h, "catan_fishing_lakes", "enabled", true, 200)
	for _, scene := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
		change(h, "catan_scenario", "catanScenario", scene, 200)
	}
	change(h, "catan_cities_knights", "catanCitiesKnights", game.CatanCitiesKnightsSetup{}, 200)
	change(h, "catan_cities_knights", "catanCitiesKnights", nil, 200)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
	if !s.rooms[id].CatanFishingLakes || current(h)["catanFishingLakes"] != true {
		t.Fatal("lost saved lake setting")
	}
	for _, c := range []*testClient{h, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.Fishing == nil || !g.Explorer.Board.FishingLakes || len(g.Players) != 2 || g.Two != nil || g.EventDeck == nil {
		t.Fatal("actual two-player recipe lost")
	}
	change(h, "catan_fishing_lakes", "enabled", false, 400)
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	change(h, "catan_fishing", "enabled", false, 200)
	if s.rooms[id].CatanFishingLakes {
		t.Fatal("disabled fishing retained lake")
	}
	change(h, "catan_events", "enabled", false, 200)
	change(h, "catan_scenario", "catanScenario", "land-ho", 200)
	for _, c := range []*testClient{h, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	h.command(current(h), "start", nil, 200)
	g = s.rooms[id].Game.Catan
	if g.Fishing != nil || g.Explorer.Board.Fishing != "" || g.EventDeck != nil {
		t.Fatal("original Explorer polluted")
	}
	h.command(current(h), "close", nil, 200)
	// Ordinary two-player rules must not inherit an Explorer fishing draft.
	h.command(current(h), "leave", nil, 200)
	raw = h.post("/api/rooms", map[string]any{"kind": "catan", "capacity": 2, "name": "双人配置切换", "catanScenario": "land-ho", "catanFishing": true, "catanFishingLakes": true}, 201)
	id = raw["id"].(string)
	change(h, "catan_two_scenario", "catanTwoScenario", "", 200)
	if r := s.rooms[id]; r.CatanFishing || r.CatanFishingLakes || r.CatanScenario != "" || r.CatanTwoRules != game.CatanTwoRules {
		t.Fatal("ordinary two-player recipe retained Explorer fishing")
	}
}

func TestCatanExplorerFishingNaturalHTTPGames(t *testing.T) {
	for _, tc := range []struct {
		n                   int
		scenario            string
		city, events, lakes bool
	}{
		{2, "land-ho", false, false, false},
		{3, "fish-for-catan", true, false, true},
		{6, "explorers-and-pirates", true, true, true},
	} {
		t.Run(fmt.Sprintf("%d/%s", tc.n, tc.scenario), func(t *testing.T) {
			s, ts, clients, id := newPublicExplorerRecipeHTTP(t, tc.n, tc.scenario, tc.city, tc.events, true, tc.lakes)
			seen := map[string]int{}
			restored, timedout := false, false
			for step := 0; step < 14000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				state := r.Game
				p := state.CatanPendingActor()
				if p < 0 {
					p = explorerHTTPActor(state)
				}
				if state.Phase == "catan_roll" && state.Catan.RollID > 0 && !timedout {
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(r.TurnDeadline))
					s.mu.Unlock()
					if !s.rooms[id].Seats[p].AutoPlay {
						t.Fatal("timeout did not enter persistent autoplay")
					}
					reclaimTimeoutHumans(t, s, clients, id)
					timedout = true
					continue
				}
				a, err := state.BotAction(p)
				if err != nil {
					t.Fatal(step, state.Phase, err)
				}
				req := map[string]any{"type": "action", "action": a, "version": r.Version, "nonce": randomID(12)}
				clients[p].post("/api/rooms/"+id, req, 200)
				seen[a.Type]++
				if !restored && a.Type == "catan_explorer_begin_move" {
					before, _ := json.Marshal(s.rooms[id])
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					clients[p].post("/api/rooms/"+id, req, 200)
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("saved replay mutated state")
					}
					restored = true
				}
				if step%137 == 0 {
					for viewer, c := range clients {
						v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
						f := v["fishing"].(map[string]any)
						tokens := f["tokens"].(map[string]any)
						if f["pending"] != nil || tokens["drawPile"] != nil || tokens["hands"] != nil {
							t.Fatal("private fish save exposed")
						}
						for player, seat := range tokens["players"].([]any) {
							if player != viewer && seat.(map[string]any)["tokens"] != nil {
								t.Fatal("opponent fish exposed")
							}
						}
					}
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || !restored || !timedout {
				t.Fatal("incomplete match", r.Game.Phase, seen)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			assertPublicExplorerHistory(t, s, clients, id)
			_, profile := clients[tc.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			rules := profile["history"].([]any)[0].(map[string]any)["catanExpansionRules"].(map[string]any)
			lakeRule := "without-lakes"
			if tc.lakes {
				lakeRule = "with-lakes"
			}
			if rules["explorer_fishing"] != r.Game.Catan.Fishing.Explorer || rules["explorer_fishing_lakes"] != lakeRule {
				t.Fatal("lost fishing recipe history", rules)
			}
			t.Log("completed", r.Game.Round, "rounds", seen)
		})
	}
}
