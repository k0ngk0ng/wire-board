package server

import (
	"fmt"
	"testing"
)

func TestCatanFishingCaravansNaturalHTTP(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, ts, clients, id := newTradersKnightsHTTP(t, n, events, "caravans", knights)
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
								if g.Caravans.Pending != nil {
									for _, bid := range g.Caravans.Pending.Bids {
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
							fish := view["fishing"].(map[string]any)
							if fish["caravans"] != "catan-fishing-caravans-2025" {
								t.Fatal("missing recipe", fish)
							}
							for _, p := range fish["tokens"].(map[string]any)["players"].([]any) {
								if p.(map[string]any)["tokens"] != nil {
									t.Fatal("spectator fish leak")
								}
							}
						}
					}
					r := s.rooms[id]
					if !r.Game.Finished || !restored {
						t.Fatal("unfinished", r.Game.Round)
					}
					_, profile := clients[n].request("GET", "/api/players/"+r.Seats[r.Game.Winners[0]].ID, nil)
					record := profile["history"].([]any)[0].(map[string]any)
					if record["catanExpansionRules"].(map[string]any)["fishing_caravans"] != "catan-fishing-caravans-2025" {
						t.Fatal("history recipe")
					}
					t.Log("complete", r.Game.Round)
				})
			}
		}
	}
}

func TestCatanFishingCaravansWaitingSwitchIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			c := newClient(t, ts.URL)
			c.register("捕鱼商队切换")
			body := map[string]any{"kind": "catan", "name": "切换", "capacity": n, "catanOptions": map[string]any{"fiveSix": n > 4}, "catanFishing": true}
			field, command := "catanScenario", "catan_scenario"
			if n == 2 {
				field, command = "catanTwoScenario", "catan_two_scenario"
			}
			body[field] = "caravans"
			raw := c.post("/api/rooms", body, 201)
			id := raw["id"].(string)
			c.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": false, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			if s.rooms[id].CatanFishing {
				t.Fatal("fishing remained enabled")
			}
			c.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": true, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			c.post("/api/rooms/"+id, map[string]any{"type": command, field: "", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			if s.rooms[id].CatanFishing {
				t.Fatal("base retained fishing")
			}
		})
	}
}
