package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanRiversAttackNaturalHTTP(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, ts, clients, id := newTradersKnightsHTTP(t, n, events, "rivers-attack", knights)
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
							rivers := view["rivers"].(map[string]any)
							if rivers["attack"] != game.CatanRiversAttackRules || view["attack"] == nil {
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
					if record["catanExpansionRules"].(map[string]any)["rivers_attack"] != game.CatanRiversAttackRules {
						t.Fatal("history recipe")
					}
					t.Log("complete", r.Game.Round)
				})
			}
		}
	}
}

func TestCatanRiversAttackWaitingIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		s, ts := setupServer(t)
		stopBotTicker(s)
		c := newClient(t, ts.URL)
		c.register(fmt.Sprint("河流蛮族切换", n))
		raw := c.post("/api/rooms", map[string]any{"kind": "catan", "name": "组合切换", "capacity": n, "catanScenario": "rivers-attack", "catanEvents": game.CatanEventCatalogue}, 201)
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
		if s.rooms[id].CatanCitiesKnights != nil || s.rooms[id].CatanScenario == "rivers-attack" {
			t.Fatal("combination leaked after switching")
		}
	}
}
func TestCatanRiversAttackSwitchFromBase(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			c := newClient(t, ts.URL)
			c.register(fmt.Sprint("切入河流蛮族", n))
			options := game.CatanOptions{FiveSix: n > 4}
			raw := c.post("/api/rooms", map[string]any{"kind": "catan", "name": "基础切组合", "capacity": n, "catanOptions": options}, 201)
			id := raw["id"].(string)
			c.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "rivers-attack", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			r := s.rooms[id]
			if r.CatanOptions != (game.CatanOptions{}) || r.CatanScenario != "rivers-attack" || r.Capacity != n {
				t.Fatal("combination did not normalize base settings")
			}
			if err := r.validateCatanScenario(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatanRiversAttackEndTimeoutRestart(t *testing.T) {
	for _, knights := range []bool{false, true} {
		t.Run(fmt.Sprint(knights), func(t *testing.T) {
			s, ts, clients, id := newTradersKnightsHTTP(t, 2, false, "rivers-attack", knights)
			phase := "catan_attack_end"
			if knights {
				phase = "catan_attack_city_move"
			}
			for step := 0; step < 8000 && s.rooms[id].Game.Phase != phase && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				actor := twoHTTPActor(r.Game)
				a, err := r.Game.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				clients[actor].command(current(clients[actor]), "action", a, 200)
			}
			r := s.rooms[id]
			if r.Game.Phase != phase {
				t.Fatal("no knight movement phase", r.Game.Phase)
			}
			actor, deadline := twoHTTPActor(r.Game), r.TurnDeadline
			for viewer, client := range clients {
				view := current(client)["game"].(map[string]any)["catan"].(map[string]any)
				for owner, raw := range view["players"].([]any) {
					if owner != viewer && raw.(map[string]any)["resources"] != nil {
						t.Fatal("private resource hand leaked")
					}
				}
				attack := view["attack"].(map[string]any)
				if attack["deck"] != nil {
					t.Fatal("attack deck order leaked")
				}
			}
			if left := deadline - time.Now().UnixMilli(); left < 119000 || left > 120000 {
				t.Fatal("missing 120-second response", left)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if s.rooms[id].TurnDeadline != deadline {
				t.Fatal("restart reset deadline")
			}
			s.mu.Lock()
			s.expireSetups(time.UnixMilli(deadline + 1))
			s.mu.Unlock()
			if !s.rooms[id].Seats[actor].AutoPlay || !s.rooms[id].Seats[actor].TimeoutAutoPlay {
				t.Fatal("timeout did not hand over")
			}
			for step := 0; step < 40 && s.rooms[id].Game.Phase == phase; step++ {
				s.mu.Lock()
				s.rooms[id].BotAt = 0
				s.runBots(time.UnixMilli(deadline + 2000 + int64(step)*2000))
				s.mu.Unlock()
			}
			if s.rooms[id].Game.Phase == phase {
				t.Fatal("autoplay did not finish movement")
			}
		})
	}
}
