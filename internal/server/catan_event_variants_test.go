package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanEventVariantsPublicNaturalHTTP(t *testing.T) {
	type recipe struct {
		scene                             string
		n                                 int
		helpers, fixed, friendly, harbors bool
	}
	cases := []recipe{}
	for n := 3; n <= 6; n++ {
		cases = append(cases, recipe{n: n, helpers: n == 4 || n == 5, friendly: true, harbors: true})
	}
	for _, n := range []int{5, 6} {
		cases = append(cases, recipe{n: n, fixed: true, friendly: true, harbors: true})
	}
	for _, scene := range []string{"shores", "islands", "fog", "desert", "cloth", "wonders", "new_world"} {
		for _, n := range []int{3, 6} {
			cases = append(cases, recipe{scene: scene, n: n, helpers: n == 3, friendly: true, harbors: true})
		}
	}
	cases = append(cases, recipe{n: 3, friendly: true}, recipe{n: 6, harbors: true})
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s/%d/helpers=%t/fixed=%t/friendly=%t/harbors=%t", tc.scene, tc.n, tc.helpers, tc.fixed, tc.friendly, tc.harbors), func(t *testing.T) {
			extra := map[string]any{"catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: tc.friendly}, "catanHarbors": game.CatanHarborsSetup{Enabled: tc.harbors}}
			s, ts, clients, id := newPublicEventsHTTP(t, tc.n, tc.scene, tc.helpers, tc.fixed, extra)
			restored := false
			steps := 0
			for ; steps < 18000 && !s.rooms[id].Game.Finished; steps++ {
				r := s.rooms[id]
				state := r.Game
				if tc.friendly {
					assertPublicFriendlyProtection(t, state)
				}
				if tc.harbors {
					assertPublicHarborPoints(t, state)
				}
				actor := twoHTTPActor(state)
				a, e := state.BotAction(actor)
				if e != nil {
					t.Fatal(steps, state.Phase, e)
				}
				code, result := clients[actor].request("POST", "/api/rooms/"+id, map[string]any{"type": "action", "version": r.Version, "nonce": randomID(12), "action": a})
				if code != 200 {
					t.Fatal(steps, state.Phase, a, code, result)
				}
				if !s.rooms[id].Game.Finished && steps%173 == 0 {
					assertPublicEventsPrivacy(t, clients)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored = true
				}
			}
			r := s.rooms[id]
			if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) == 0 || r.TurnDeadline != 0 || !restored {
				t.Fatal("incomplete match", steps, r.Game.Phase)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			_, profile := clients[tc.n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
			history := profile["history"].([]any)
			if len(history) != 1 {
				t.Fatal("duplicate archive")
			}
			rules := history[0].(map[string]any)["catanExpansionRules"].(map[string]any)
			if rules["event_cards"] != game.CatanEventCatalogue || tc.friendly && rules["friendly_robber"] != game.CatanFriendlyRobberRules || tc.harbors && rules["harbors"] != game.CatanHarborsRules {
				t.Fatal("lost actual rules", rules)
			}
			t.Log("natural actions", steps, "round", r.Game.Round)
		})
	}
}

func TestCatanEventVariantsKnightsNaturalHTTP(t *testing.T) {
	for _, scene := range []string{"", "shores", "islands", "fog", "desert", "cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) { testCatanCitiesKnightsEventsFullHTTPGames(t, scene, true, true, true, 3, 6) })
	}
}

func TestCatanEventVariantsPublicToggleRestoreBase(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, c := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("事件变体房主")
			c.register("事件变体朋友")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "事件变体", "capacity": n, "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: true}, "catanEvents": game.CatanEventCatalogue}, 201)
			id := raw["id"].(string)
			c.command(raw, "join", nil, 200)
			for i := 2; i < n; i++ {
				h.command(current(h), "add_bot", nil, 200)
			}
			ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
			events := func(enabled bool) {
				h.post("/api/rooms/"+id, map[string]any{"type": "catan_events", "enabled": enabled, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			}
			ready()
			before, _ := json.Marshal(s.rooms[id])
			selectCatanFriendlyRobber(c, true, 400)
			selectCatanHarbors(c, true, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("guest mutation")
			}
			selectCatanFriendlyRobber(h, true, 200)
			selectCatanHarbors(h, true, 200)
			for _, seat := range s.rooms[id].Seats {
				if seat.Ready != seat.Bot {
					t.Fatal("readiness not reset")
				}
			}
			if s.rooms[id].CatanEvents != game.CatanEventCatalogue {
				t.Fatal("adding variants lost event choice")
			}
			ready()
			selectCatanFriendlyRobber(h, true, 200)
			selectCatanHarbors(h, true, 200)
			for _, seat := range s.rooms[id].Seats {
				if !seat.Ready {
					t.Fatal("noop cleared ready")
				}
			}
			events(false)
			events(true)
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "cloth", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			if s.rooms[id].CatanEvents == "" || !s.rooms[id].CatanFriendlyRobber.Enabled || !s.rooms[id].CatanHarbors.Enabled {
				t.Fatal("map switch lost variants")
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
			ready()
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if g.EventDeck == nil || g.FriendlyRobber == nil || g.Harbors == nil || !g.Options.Helpers {
				t.Fatal("missing combined start")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if s.rooms[id].CatanEvents == "" {
				t.Fatal("rematch lost events")
			}
			selectCatanFriendlyRobber(h, false, 200)
			selectCatanHarbors(h, false, 200)
			events(false)
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: n > 4}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
			ready()
			h.command(current(h), "start", nil, 200)
			g = s.rooms[id].Game.Catan
			if g.EventDeck != nil || g.Seafarers != nil || g.FriendlyRobber != nil || g.Harbors != nil || g.Options.Helpers {
				t.Fatal("base contaminated")
			}
		})
	}
}
