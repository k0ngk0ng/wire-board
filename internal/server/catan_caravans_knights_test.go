package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

func newCaravanKnightsHTTP(t *testing.T, n int, events bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	return newTradersKnightsHTTP(t, n, events, "caravans")
}
func newTradersKnightsHTTP(t *testing.T, n int, events bool, scenario string, fishing ...bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("商队骑士%d", p))
	}
	recipe := map[string]any{"kind": "catan", "name": "商队骑士验收", "capacity": n, "catanOptions": game.CatanOptions{FiveSix: n > 4}, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}}
	if publicCatanTradersCombination(scenario) || scenario == "transport" || scenario == "barbarian-attack" && n > 2 {
		recipe["catanScenario"] = scenario
		recipe["catanOptions"] = game.CatanOptions{}
	} else if n == 2 {
		recipe["catanTwoScenario"] = scenario
	} else {
		recipe["catanScenario"] = scenario
	}
	if len(fishing) > 0 {
		if !publicCatanTradersCombination(scenario) {
			recipe["catanFishing"] = true
		}
		if !fishing[0] {
			delete(recipe, "catanCitiesKnights")
		}
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
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	clients[0].command(current(clients[0]), "start", nil, 200)
	ordered := make([]*testClient, n+1)
	for _, c := range clients[:n] {
		ordered[int(current(c)["you"].(float64))] = c
	}
	ordered[n] = clients[n]
	ordered[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, ordered, id
}
func TestCatanCaravansKnightsNaturalHTTP(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, ts, clients, id := newCaravanKnightsHTTP(t, n, events)
				restored := false
				bids := 0
				for step := 0; step < 14000 && !s.rooms[id].Game.Finished; step++ {
					r := s.rooms[id]
					actor := twoHTTPActor(r.Game)
					a, err := r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(step, r.Game.Phase, err)
					}
					clients[actor].command(current(clients[actor]), "action", a, 200)
					if a.Type == "catan_caravan_bid" {
						bids++
					}
					if !restored && a.Type == "catan_caravan_bid" {
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						restored = true
					}
					if step%97 == 0 {
						g := s.rooms[id].Game.Catan
						for color, total := range g.Bank {
							for p, seat := range g.Players {
								total += seat.Resources[color]
								if q := g.Caravans.Pending; q != nil && color < len(q.Bids[p]) {
									total += q.Bids[p][color]
								}
							}
							want := 19
							if n > 4 {
								want = 24
							}
							if color >= 5 {
								want = 12
								if n > 4 {
									want = 18
								}
							}
							if total != want {
								t.Fatal("inventory", color, total, want)
							}
						}
						view := current(clients[n])["game"].(map[string]any)["catan"].(map[string]any)
						if view["victoryTarget"] != float64(15) {
							t.Fatal("target")
						}
						for _, raw := range view["players"].([]any) {
							if raw.(map[string]any)["resources"] != nil {
								t.Fatal("spectator hand leak")
							}
						}
						for _, raw := range view["citiesKnights"].(map[string]any)["players"].([]any) {
							if raw.(map[string]any)["progress"] != nil {
								t.Fatal("progress leak")
							}
						}
					}
				}
				r := s.rooms[id]
				if !r.Game.Finished || !restored {
					t.Fatal("unfinished", r.Game.Round, bids)
				}
				_, profile := clients[n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
				record := profile["history"].([]any)[0].(map[string]any)
				if record["catanExpansionRules"].(map[string]any)["caravansKnights"] != game.CatanCaravansKnightsRules {
					t.Fatal("history marker")
				}
				t.Log("rounds", r.Game.Round, "bids", bids)
			})
		}
	}
}
func TestCatanCaravansKnightsConfigurationAndBaseIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newCaravanKnightsHTTP(t, n, true)
			host := clients[0]
			// Seat order is stable at present; find the authoritative host after start.
			for p, seat := range s.rooms[id].Seats {
				if seat.ID == s.rooms[id].Host {
					host = clients[p]
				}
			}
			host.command(current(host), "close", nil, 200)
			host.command(current(host), "rematch", nil, 200)
			id = current(host)["id"].(string)
			change := func(c *testClient, on bool, status int) {
				var setup any
				if on {
					setup = game.CatanCitiesKnightsSetup{}
				}
				c.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": setup, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			guest := clients[0]
			if guest == host {
				guest = clients[1]
			}
			change(guest, false, 400)
			change(host, false, 200)
			change(host, true, 200)
			change(host, false, 200)
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			for _, c := range clients[:n] {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if g.CitiesKnights != nil || g.Caravans.Knights != "" || len(g.Bank) != 5 || len(g.DevDeck) == 0 {
				t.Fatal("plain caravan changed")
			}
		})
	}
}

func TestCatanCaravansKnightsTimeoutAndAutoplay(t *testing.T) {
	for _, n := range []int{2, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newCaravanKnightsHTTP(t, n, true)
			for step := 0; s.rooms[id].Game.Catan.Caravans.Pending == nil; step++ {
				if step > 4000 {
					t.Fatal("no vote")
				}
				r := s.rooms[id]
				p := twoHTTPActor(r.Game)
				a, e := r.Game.BotAction(p)
				if e != nil {
					t.Fatal(e)
				}
				clients[p].command(current(clients[p]), "action", a, 200)
			}
			r := s.rooms[id]
			actor := twoHTTPActor(r.Game)
			if r.TurnDeadline == 0 {
				t.Fatal("missing responder clock")
			}
			version := r.Version
			clients[n].command(current(clients[n]), "action", game.Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 0, 0, 0}}, 400)
			if s.rooms[id].Version != version {
				t.Fatal("spectator mutated response")
			}
			s.mu.Lock()
			r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
			s.expireSetups(time.Now())
			s.mu.Unlock()
			if s.rooms[id].Version <= version || !s.rooms[id].Seats[actor].AutoPlay {
				t.Fatal("timeout did not enable autoplay")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			reclaimTimeoutHumans(t, s, clients, id)
			actor = twoHTTPActor(s.rooms[id].Game)
			setAutoPlay(clients[actor], current(clients[actor]), true, 200)
			version = s.rooms[id].Version
			s.mu.Lock()
			s.rooms[id].BotAt = 0
			s.runBots(time.Now())
			s.mu.Unlock()
			if s.rooms[id].Version <= version {
				t.Fatal("autoplay stalled")
			}
		})
	}
}
