package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func newPublicEventsHTTP(t *testing.T, n int, scenario string, helpers, fixed bool, extras ...map[string]any) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("事件玩家%d", p))
	}
	recipe := map[string]any{"name": "事件牌公开验收", "kind": "catan", "capacity": n, "catanEvents": game.CatanEventCatalogue}
	if n == 2 {
		recipe["catanTwoScenario"] = scenario
	} else {
		recipe["catanScenario"] = scenario
	}
	if scenario != "barbarian-attack" {
		recipe["catanOptions"] = game.CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}
	}
	if fixed {
		recipe["catanBaseConfiguration"] = game.CatanBaseConfiguration{Layout: "fixed"}
	}
	for _, extra := range extras {
		for key, value := range extra {
			recipe[key] = value
		}
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
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	if s.rooms[id].Game.Catan.EventDeck == nil {
		t.Fatal("public opening missing event deck")
	}
	return s, ts, clients, id
}

func assertPublicEventsPrivacy(t *testing.T, clients []*testClient) {
	t.Helper()
	for viewer, c := range clients {
		v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
		deck, ok := v["eventDeck"].(map[string]any)
		if !ok || deck["catalogue"] != game.CatanEventCatalogue || deck["referenceOnly"] != false || deck["deck"] != nil || deck["drawPile"] != nil {
			t.Fatal("missing public metadata or exposed deck", deck)
		}
		for p, raw := range v["players"].([]any) {
			if (raw.(map[string]any)["resources"] != nil) != (p == viewer) {
				t.Fatal("resource privacy", viewer, p)
			}
		}
	}
}

func TestCatanPublicEventsConfigurationAndRematch(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("事件房主")
	guest := newClient(t, ts.URL)
	guest.register("事件客人")
	for _, bad := range []map[string]any{
		{"kind": "catan", "capacity": 3, "catanEvents": "legacy-reference-v1"},
		{"kind": "catan", "capacity": 3, "catanEvents": game.CatanEventCatalogue, "catanScenario": "transport"},
		{"kind": "splendor", "capacity": 3, "catanEvents": game.CatanEventCatalogue},
	} {
		bad["name"] = "无效事件"
		host.post("/api/rooms", bad, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "事件开关", "capacity": 3}, 201)
	id := raw["id"].(string)
	guest.command(current(host), "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	change := func(c *testClient, kind, key string, value any, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	ready := func() {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
	}
	ready()
	before, _ := json.Marshal(s.rooms[id])
	change(guest, "catan_events", "enabled", true, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("unauthorized mutated room")
	}
	change(host, "catan_events", "enabled", true, 200)
	if s.rooms[id].Seats[0].Ready || s.rooms[id].Seats[1].Ready || !s.rooms[id].Seats[2].Ready {
		t.Fatal("readiness not reset")
	}
	ready()
	change(host, "catan_events", "enabled", true, 200)
	if !s.rooms[id].Seats[0].Ready || !s.rooms[id].Seats[1].Ready {
		t.Fatal("noop cleared ready")
	}
	change(host, "catan_scenario", "catanScenario", "rivers", 200)
	if s.rooms[id].CatanEvents != game.CatanEventCatalogue {
		t.Fatal("compatible scenario lost deck")
	}
	change(host, "catan_scenario", "catanScenario", "transport", 200)
	if s.rooms[id].CatanEvents != "" {
		t.Fatal("unsupported scenario kept deck")
	}
	before, _ = json.Marshal(s.rooms[id])
	change(host, "catan_events", "enabled", true, 400)
	after, _ = json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid enable changed room")
	}
	change(host, "catan_scenario", "catanScenario", "", 200)
	// Both enabled variants and disabled waiting-room preferences coexist with events.
	for _, variant := range []struct{ command, key string }{
		{"catan_harbors", "catanHarbors"}, {"catan_friendly_robber", "catanFriendlyRobber"},
	} {
		change(host, variant.command, variant.key, map[string]any{"enabled": true}, 200)
		change(host, "catan_events", "enabled", true, 200)
		change(host, variant.command, variant.key, map[string]any{"enabled": false}, 200)
	}
	change(host, "catan_events", "enabled", true, 200)
	if summary(s.rooms[id])["catanEvents"] != game.CatanEventCatalogue {
		t.Fatal("lobby lost catalogue")
	}
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	ready()
	host.command(current(host), "start", nil, 200)
	if s.rooms[id].Game.Catan.EventDeck == nil {
		t.Fatal("start lost deck")
	}
	change(host, "catan_events", "enabled", false, 400)
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if s.rooms[id].CatanEvents != game.CatanEventCatalogue || s.rooms[id].Game != nil {
		t.Fatal("rematch lost choice or kept old deck")
	}
	change(host, "catan_events", "enabled", false, 200)
	ready()
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.EventDeck != nil || g.RevealedEvent != nil || g.Rivers != nil || g.Fishing != nil {
		t.Fatal("base contaminated")
	}
}

func TestCatanPublicEventsNaturalHTTPMatches(t *testing.T) {
	type recipe struct {
		scenario       string
		n              int
		helpers, fixed bool
	}
	cases := []recipe{}
	for n := 2; n <= 6; n++ {
		for _, scenario := range []string{"", "rivers", "caravans"} {
			cases = append(cases, recipe{scenario: scenario, n: n})
		}
	}
	for _, n := range []int{3, 6} {
		cases = append(cases, recipe{scenario: "barbarian-attack", n: n}, recipe{n: n, helpers: true})
	}
	for _, n := range []int{5, 6} {
		cases = append(cases, recipe{n: n, fixed: true})
	}
	for _, scenario := range []string{"shores", "islands", "fog", "desert"} {
		for _, n := range []int{3, 4, 5, 6} {
			cases = append(cases, recipe{scenario: scenario, n: n, helpers: n == 4})
			if n > 4 {
				cases = append(cases, recipe{scenario: scenario, n: n, helpers: true})
			}
		}
	}

	for _, scenario := range []string{"cloth", "wonders", "new_world"} {
		for n := 3; n <= 6; n++ {
			for _, helpers := range []bool{false, true} {
				cases = append(cases, recipe{scenario: scenario, n: n, helpers: helpers})
			}
		}
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%d/helpers=%t/fixed=%t", tc.scenario, tc.n, tc.helpers, tc.fixed), func(t *testing.T) {
			s, ts, clients, id := newPublicEventsHTTP(t, tc.n, tc.scenario, tc.helpers, tc.fixed)
			seen := map[string]int{}
			steps := 0
			restored := false
			for ; steps < 18000 && !s.rooms[id].Game.Finished; steps++ {
				r := s.rooms[id]
				state := r.Game
				actor := twoHTTPActor(state)
				a, e := state.BotAction(actor)
				if e != nil {
					t.Fatal("bot", steps, state.Phase, e)
				}
				code, result := clients[actor].request("POST", "/api/rooms/"+id, map[string]any{"type": "action", "version": r.Version, "nonce": randomID(12), "action": a})
				if code != 200 {
					t.Fatal("natural HTTP", steps, state.Phase, a, code, result)
				}
				seen[a.Type]++
				if !s.rooms[id].Game.Finished && steps%173 == 0 {
					assertPublicEventsPrivacy(t, clients)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored = true
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || r.Status != "finished" || (len(r.Game.Winners) == 0 || tc.scenario != "cloth" && len(r.Game.Winners) != 1) || r.TurnDeadline != 0 || !restored || seen["catan_roll"] == 0 {
				t.Fatal("unfinished natural event game", steps, r.Game.Round, seen)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			winner := r.Game.Winners[0]
			_, profile := clients[tc.n].request("GET", "/api/players/"+r.Seats[winner].ID, nil)
			history := profile["history"].([]any)
			if len(history) != 1 {
				t.Fatal("duplicate archive")
			}
			record := history[0].(map[string]any)
			if record["catanExpansionRules"].(map[string]any)["event_cards"] != game.CatanEventCatalogue {
				t.Fatal("history lost deck", record)
			}
			if tc.scenario == "cloth" && record["catanExpansionRules"].(map[string]any)["event_cloth_fallback"] != game.CatanEventClothFallbackRules {
				t.Fatal("missing cloth fallback history")
			}
			t.Log("natural actions", steps, "round", r.Game.Round, "seen", seen)
		})
	}
}
