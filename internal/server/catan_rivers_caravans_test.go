package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanRiversCaravansNaturalHTTP(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, ts, clients, id := newTradersKnightsHTTP(t, n, events, "rivers-caravans", knights)
					restored := false
					for step := 0; step < 16000 && !s.rooms[id].Game.Finished; step++ {
						r := s.rooms[id]
						actor := twoHTTPActor(r.Game)
						a, e := r.Game.BotAction(actor)
						if e != nil {
							t.Fatal(step, r.Game.Phase, e)
						}
						clients[actor].command(current(clients[actor]), "action", a, 200)
						if step == 83 {
							s, ts = restartRiversHTTP(t, s, ts, clients, id)
							restored = true
						}
						if step%97 == 0 {
							g := s.rooms[id].Game.Catan
							for color, total := range g.Bank {
								for _, p := range g.Players {
									total += p.Resources[color]
								}
								if q := g.Caravans.Pending; q != nil {
									for _, bid := range q.Bids {
										if color < len(bid) {
											total += bid[color]
										}
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
									t.Fatal("resource supply", color, total, want)
								}
							}
							view := current(clients[n])["game"].(map[string]any)["catan"].(map[string]any)
							caravans := view["caravans"].(map[string]any)
							if caravans["rivers"] != game.CatanRiversCaravansRules || view["rivers"] == nil {
								t.Fatal("missing combination")
							}

						}
					}
					r := s.rooms[id]
					if !r.Game.Finished || !restored {
						t.Fatal("unfinished", r.Game.Round)
					}
					_, profile := clients[n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
					record := profile["history"].([]any)[0].(map[string]any)
					if record["catanExpansionRules"].(map[string]any)["rivers_caravans"] != game.CatanRiversCaravansRules {
						t.Fatal("history recipe")
					}
					t.Log("complete", r.Game.Round)
				})
			}
		}
	}
}

func TestCatanRiversCaravansWaitingIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s, ts := setupServer(t)
		stopBotTicker(s)
		c := newClient(t, ts.URL)
		c.register(fmt.Sprint("河流商队切换", n))
		raw := c.post("/api/rooms", map[string]any{"kind": "catan", "name": "组合切换", "capacity": n, "catanScenario": "rivers-caravans", "catanEvents": game.CatanEventCatalogue}, 201)
		id := raw["id"].(string)
		c.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
		if s.rooms[id].CatanCitiesKnights == nil {
			t.Fatal("knights not enabled")
		}
		c.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": true, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
		field, cmd, target := "catanScenario", "catan_scenario", ""
		if n == 2 {
			field, cmd = "catanTwoScenario", "catan_two_scenario"
		}
		if n > 4 {
			target = "transport"
		}
		c.post("/api/rooms/"+id, map[string]any{"type": cmd, field: target, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
		if s.rooms[id].CatanCitiesKnights != nil || s.rooms[id].CatanScenario == "rivers-caravans" {
			t.Fatal("combination leaked after switching")
		}
	}
}
func TestCatanRiversCaravansBidRestartTimeout(t *testing.T) {
	s, ts, clients, id := newTradersKnightsHTTP(t, 2, true, "rivers-caravans", false)
	for step := 0; step < 3000 && s.rooms[id].Game.Phase != "catan_caravan_bid"; step++ {
		r := s.rooms[id]
		actor := twoHTTPActor(r.Game)
		a, e := r.Game.BotAction(actor)
		if e != nil {
			t.Fatal(e)
		}
		clients[actor].command(current(clients[actor]), "action", a, 200)
	}
	r := s.rooms[id]
	if r.Game.Phase != "catan_caravan_bid" {
		t.Fatal("no bidding")
	}
	actor := twoHTTPActor(r.Game)
	deadline := r.TurnDeadline
	if left := deadline - time.Now().UnixMilli(); left < 119000 || left > 120000 {
		t.Fatal("not120s", left)
	}
	before := len(r.Game.Catan.Caravans.Wagons)
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	_ = ts
	if s.rooms[id].TurnDeadline != deadline {
		t.Fatal("restart reset clock")
	}
	s.mu.Lock()
	s.expireSetups(time.UnixMilli(deadline + 1))
	s.mu.Unlock()
	if !s.rooms[id].Seats[actor].AutoPlay {
		t.Fatal("timeout did not enable autoplay")
	}
	for step := 0; step < 30 && s.rooms[id].Game.Catan.Caravans.Pending != nil; step++ {
		r = s.rooms[id]
		p := twoHTTPActor(r.Game)
		if r.Seats[p].AutoPlay {
			s.mu.Lock()
			s.rooms[id].BotAt = 0
			s.runBots(time.UnixMilli(deadline + 2000 + int64(step)*2000))
			s.mu.Unlock()
		} else {
			a, e := r.Game.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			clients[p].command(current(clients[p]), "action", a, 200)
		}
	}
	r = s.rooms[id]
	if r.Game.Catan.Caravans.Pending != nil || len(r.Game.Catan.Caravans.Wagons) != before+2 {
		t.Fatal("two-wagon round did not finish")
	}
}

func TestCatanRiversCaravansSwitchFromBase(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			c := newClient(t, ts.URL)
			c.register(fmt.Sprint("切入河流商队", n))
			options := game.CatanOptions{FiveSix: n > 4}
			raw := c.post("/api/rooms", map[string]any{"kind": "catan", "name": "基础切组合", "capacity": n, "catanOptions": options}, 201)
			id := raw["id"].(string)
			c.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "rivers-caravans", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			r := s.rooms[id]
			if r.CatanOptions != (game.CatanOptions{}) || r.CatanScenario != "rivers-caravans" || r.Capacity != n {
				t.Fatal("combination did not normalize base settings")
			}
			if err := r.validateCatanScenario(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
