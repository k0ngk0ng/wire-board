package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"reflect"
	"testing"
	"time"
)

func TestCatanRiversSeaAdmission(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, scenario := range []string{"rivers-shores", "rivers-fog", "rivers-desert", "rivers-desert-belt", "rivers-tribe", "rivers-new-world"} {
			r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
			err := r.setCatanScenario(scenario)
			if n > 4 && (scenario == "rivers-desert" || scenario == "rivers-desert-belt" || scenario == "rivers-tribe") {
				if err == nil {
					t.Fatal("unfinished admitted")
				}
				continue
			}
			if err != nil {
				t.Fatal(n, scenario, err)
			}
			if err = r.setCatanEvents(true); err != nil {
				t.Fatal(err)
			}
			if err = r.validateCatanScenario(); err != nil {
				t.Fatal(err)
			}
			if n == 2 {
				err = r.setCatanTwoScenario("")
			} else {
				err = r.setCatanScenario("transport")
			}
			if err != nil {
				t.Fatal("switch", err)
			}
			if publicCatanRiversSea(r.CatanScenario) {
				t.Fatal("stale river configuration")
			}
		}
	}
	r := &Room{Kind: "catan", Status: "waiting", Capacity: 3, CatanScenario: "rivers-shores", CatanOptions: game.CatanOptions{Helpers: true}}
	if r.validateCatanScenario() == nil {
		t.Fatal("unsupported helpers accepted")
	}
}

func TestCatanRiversSeaPublicStart(t *testing.T) {
	for _, scenario := range []string{"rivers-shores", "rivers-fog", "rivers-desert", "rivers-desert-belt", "rivers-tribe", "rivers-new-world"} {
		t.Run(scenario, func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			host := newClient(t, ts.URL)
			host.register("河流海图房主")
			guest := newClient(t, ts.URL)
			guest.register("河流海图同伴")
			raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "河流海图", "capacity": 2, "catanScenario": scenario, "catanEvents": game.CatanEventCatalogue}, 201)
			id := raw["id"].(string)
			guest.command(current(host), "join", nil, 200)
			host.command(current(host), "ready", nil, 200)
			guest.command(current(guest), "ready", nil, 200)
			host.command(current(host), "start", nil, 200)
			clients := []*testClient{host, guest}
			if int(current(host)["you"].(float64)) != 0 {
				clients[0], clients[1] = guest, host
			}
			for step := 0; step < 30 && !s.rooms[id].Game.Finished; step++ {
				g := s.rooms[id].Game
				p := twoHTTPActor(g)
				a, err := g.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				clients[p].command(current(clients[p]), "action", a, 200)
			}
			g := s.rooms[id].Game.Catan
			if g.Rivers == nil || g.Seafarers == nil || g.Two == nil {
				t.Fatal("wrong game")
			}

			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			r := s.rooms[id]
			actor := twoHTTPActor(r.Game)
			deadline := r.TurnDeadline
			if deadline <= 0 {
				t.Fatal("missing deadline")
			}
			s.mu.Lock()
			s.expireSetups(time.UnixMilli(deadline + 1))
			s.mu.Unlock()
			if !s.rooms[id].Seats[actor].AutoPlay || !s.rooms[id].Seats[actor].TimeoutAutoPlay {
				t.Fatal("timeout did not take over")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if !s.rooms[id].Seats[actor].AutoPlay {
				t.Fatal("autoplay not restored")
			}
			host.command(current(host), "close", nil, 200)
			_, profile := host.request("GET", "/api/players/"+s.rooms[id].Host, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			if record["catanScenario"] != scenario || record["catanRules"] != game.CatanRiversSeafarersRules {
				t.Fatal("lost river sea history", record)
			}
			if scenario == "rivers-new-world" && record["catanLayout"] != "river-default" {
				t.Fatal("default map mislabeled as approved")
			}
			rules := record["catanExpansionRules"].(map[string]any)
			if rules["rivers_seafarers"] != game.CatanRiversSeafarersRules || rules["two_rivers_seafarers"] == nil {
				t.Fatal("missing combination version")
			}
			host.command(current(host), "rematch", nil, 200)
			if s.rooms[id].CatanScenario != scenario || s.rooms[id].Game != nil {
				t.Fatal("rematch configuration")
			}
			for _, seat := range s.rooms[id].Seats {
				if seat.AutoPlay || seat.TimeoutAutoPlay {
					t.Fatal("rematch kept takeover")
				}
			}
			host.command(current(host), "ready", nil, 200)
			guest.command(current(guest), "ready", nil, 200)
			host.command(current(host), "start", nil, 200)
			if s.rooms[id].Game.Catan.Rivers == nil || s.rooms[id].Game.Catan.Seafarers == nil {
				t.Fatal("rematch dropped combination")
			}

		})
	}
}

func TestCatanRiversSeaNaturalHTTP(t *testing.T) {
	for _, tc := range []struct {
		scenario string
		n        int
		events   bool
	}{
		{"rivers-shores", 2, false}, {"rivers-fog", 3, true}, {"rivers-desert", 4, false},
		{"rivers-desert-belt", 2, true}, {"rivers-tribe", 3, false}, {"rivers-new-world", 4, true},
		{"rivers-shores", 5, true}, {"rivers-fog", 6, false}, {"rivers-new-world", 6, true},
	} {
		t.Run(fmt.Sprintf("%s/%d/events%t", tc.scenario, tc.n, tc.events), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, tc.n+1)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("河流航海%d", p))
			}
			recipe := map[string]any{"kind": "catan", "name": "河流航海整局", "capacity": tc.n, "catanScenario": tc.scenario}
			if tc.events {
				recipe["catanEvents"] = game.CatanEventCatalogue
			}
			raw := clients[0].post("/api/rooms", recipe, 201)
			id := raw["id"].(string)
			for p := 1; p < tc.n; p++ {
				clients[p].command(current(clients[0]), "join", nil, 200)
			}
			for p := 0; p < tc.n; p++ {
				clients[p].command(current(clients[p]), "ready", nil, 200)
			}
			clients[0].command(current(clients[0]), "start", nil, 200)
			ordered := make([]*testClient, tc.n+1)
			for _, c := range clients[:tc.n] {
				ordered[int(current(c)["you"].(float64))] = c
			}
			ordered[tc.n] = clients[tc.n]
			clients = ordered
			clients[tc.n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			restored := false
			for step := 0; step < 24000 && !s.rooms[id].Game.Finished; step++ {
				g := s.rooms[id].Game
				p := twoHTTPActor(g)
				a, err := g.BotAction(p)
				if err != nil {
					t.Fatal(step, g.Phase, err)
				}
				clients[p].command(current(clients[p]), "action", a, 200)
				if step == 83 {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					restored = true
				}
				if step%131 == 0 {
					view := current(clients[tc.n])["game"].(map[string]any)["catan"].(map[string]any)
					if view["rivers"] == nil || view["seafarers"] == nil {
						t.Fatal("missing public components")
					}
					sea := view["seafarers"].(map[string]any)
					if fog, ok := sea["fog"].(map[string]any); ok {
						if _, ok = fog["terrain"]; ok {
							t.Fatal("fog leak")
						}
					}
				}
			}
			if !s.rooms[id].Game.Finished || !restored {
				t.Fatal("incomplete", s.rooms[id].Game.Round)
			}
			t.Log("round", s.rooms[id].Game.Round, "winners", s.rooms[id].Game.Winners)
		})
	}
}

func TestCatanRiversPreparedPublic(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, n)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("预备河流%d", p))
			}
			h := clients[0]
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "预备河流", "capacity": n, "catanScenario": "rivers-new-world"}, 201)
			id := raw["id"].(string)
			for p := 1; p < n; p++ {
				clients[p].command(current(h), "join", nil, 200)
			}
			clients[1].command(current(clients[1]), "catan_rivers_world_shuffle", nil, 400)
			h.command(current(h), "catan_rivers_world_shuffle", nil, 200)
			confirmed := s.rooms[id].CatanRiversWorldMap
			if confirmed == nil {
				t.Fatal("missing saved map")
			}
			for _, c := range clients {
				c.command(current(c), "ready", nil, 200)
			}
			h.command(current(h), "catan_rivers_world_shuffle", nil, 200)
			for _, seat := range s.rooms[id].Seats {
				if seat.Ready {
					t.Fatal("map change kept ready")
				}
			}
			confirmed = s.rooms[id].CatanRiversWorldMap
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if !reflect.DeepEqual(confirmed, s.rooms[id].CatanRiversWorldMap) {
				t.Fatal("map lost on restart")
			}
			for _, c := range clients {
				c.command(current(c), "ready", nil, 200)
			}
			h.command(current(h), "start", nil, 200)
			if !reflect.DeepEqual(confirmed, s.rooms[id].Game.Catan.RiversWorldMap()) {
				t.Fatal("approved map rerolled")
			}
			h.command(current(h), "catan_rivers_world_shuffle", nil, 400)
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if !reflect.DeepEqual(confirmed, s.rooms[id].CatanRiversWorldMap) {
				t.Fatal("rematch lost map")
			}
			h.command(current(h), "catan_rivers_world_default", nil, 200)
			if s.rooms[id].CatanRiversWorldMap != nil {
				t.Fatal("default kept map")
			}
		})
	}
}
