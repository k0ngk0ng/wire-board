package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanAttackKnightsNaturalHTTP(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, ts, clients, id := newTradersKnightsHTTP(t, n, events, "barbarian-attack")
				restored := false
				for step := 0; step < 16000 && !s.rooms[id].Game.Finished; step++ {
					r := s.rooms[id]
					p := twoHTTPActor(r.Game)
					a, e := r.Game.BotAction(p)
					if e != nil {
						t.Fatal(step, r.Game.Phase, e)
					}
					clients[p].command(current(clients[p]), "action", a, 200)
					if step == 97 {
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						restored = true
					}
					if step%71 == 0 {
						g := s.rooms[id].Game.Catan
						total := g.Attack.GoldBank
						for _, gold := range g.Attack.Gold {
							total += gold
						}
						want := g.Attack.Map.Gold
						if total != want+g.Attack.GoldIssued {
							t.Fatal("gold inventory")
						}
						for color, total := range g.Bank {
							for _, seat := range g.Players {
								total += seat.Resources[color]
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
								t.Fatal("card inventory", color, total)
							}
						}
						v := current(clients[n])["game"].(map[string]any)["catan"].(map[string]any)
						if v["victoryTarget"] != float64(13) {
							t.Fatal("target")
						}
						for _, raw := range v["players"].([]any) {
							if raw.(map[string]any)["resources"] != nil {
								t.Fatal("hidden hand leaked")
							}
						}
						for _, raw := range v["citiesKnights"].(map[string]any)["players"].([]any) {
							if raw.(map[string]any)["progress"] != nil {
								t.Fatal("progress leaked")
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
				if record["catanExpansionRules"].(map[string]any)["attackKnights"] != game.CatanAttackKnightsRules {
					t.Fatal("history marker")
				}
				if n == 2 && record["catanExpansionRules"].(map[string]any)["two_attack"] != game.CatanTwoAttackKnightsRules {
					t.Fatal("two-player history marker")
				}
				t.Log("rounds", r.Game.Round)
			})
		}
	}
}

func TestCatanAttackKnightsPlanHTTPClockAndBase(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newTradersKnightsHTTP(t, n, true, "barbarian-attack")
			for step := 0; ; step++ {
				r := s.rooms[id]
				if r.Game.Catan.Attack.City.Plan != nil {
					break
				}
				if step > 2500 {
					t.Fatal("missing end plan")
				}
				actor := twoHTTPActor(r.Game)
				a, err := r.Game.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				clients[actor].command(current(clients[actor]), "action", a, 200)
			}
			r := s.rooms[id]
			actor := twoHTTPActor(r.Game)
			deadline := r.TurnDeadline
			if left := deadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
				t.Fatal("end clock", left)
			}
			action, err := r.Game.BotAction(actor)
			if err != nil {
				t.Fatal(err)
			}
			version := r.Version
			clients[n].command(current(clients[n]), "action", action, 400)
			clients[(actor+1)%n].command(current(clients[(actor+1)%n]), "action", action, 400)
			if s.rooms[id].Version != version || s.rooms[id].TurnDeadline != deadline {
				t.Fatal("invalid action changed clock")
			}
			for viewer, c := range clients {
				v := current(c)["game"].(map[string]any)["catan"].(map[string]any)["attack"].(map[string]any)["city"].(map[string]any)
				if (v["choices"] != nil) != (viewer == actor) || v["before"] != nil {
					t.Fatal("choices leaked")
				}
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			if s.rooms[id].TurnDeadline != deadline {
				t.Fatal("restore reset clock")
			}
			r = s.rooms[id]
			r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
			s.mu.Lock()
			s.expireSetups(time.Now())
			s.mu.Unlock()
			if !s.rooms[id].Seats[actor].AutoPlay || s.rooms[id].Version <= version {
				t.Fatal("timeout did not take over")
			}
			reclaimTimeoutHumans(t, s, clients, id)
			host := clients[0]
			for p, seat := range s.rooms[id].Seats {
				if seat.ID == s.rooms[id].Host {
					host = clients[p]
				}
			}
			host.command(current(host), "close", nil, 200)
			host.command(current(host), "rematch", nil, 200)
			id = current(host)["id"].(string)
			host.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": nil, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			for _, c := range clients[:n] {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if g.Attack.City != nil || g.CitiesKnights != nil || len(g.Bank) != 5 || len(g.Attack.Deck) == 0 {
				t.Fatal("ordinary attack changed")
			}
		})
	}
}

func TestCatanAttackKnightsRetreatClock(t *testing.T) {
	// Clock transitions only; live plan/permissions are checked through HTTP above.
	var state game.State
	if err := json.Unmarshal([]byte(`{"kind":"catan","phase":"catan_attack_city_move","turn":0,"catan":{}}`), &state); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	r := &Room{Status: "playing", Game: &state, TurnDeadline: now.Add(37 * time.Second).UnixMilli()}
	r.Game.Phase = "catan_attack_city_retreat"
	if !r.adjustCatanResponseClock("catan_attack_city_move", 0, 0, now) || r.CatanTimeLeft != 37000 || r.TurnDeadline != now.Add(120*time.Second).UnixMilli() {
		t.Fatal("retreat budget")
	}
	r.Game.Phase = "catan_attack_city_move"
	later := now.Add(13 * time.Second)
	r.adjustCatanResponseClock("catan_attack_city_retreat", 1, 0, later)
	if r.TurnDeadline != later.Add(37*time.Second).UnixMilli() || r.CatanTimeLeft != 0 {
		t.Fatal("retreat did not restore budget")
	}
	deadline := r.TurnDeadline
	r.adjustCatanResponseClock("catan_attack_city_move", 0, 0, later.Add(time.Second))
	if r.TurnDeadline != deadline {
		t.Fatal("move resets clock")
	}
	r.Game.Phase = "catan_roll"
	r.adjustCatanResponseClock("catan_attack_city_move", 0, 0, later)
	if r.TurnDeadline != later.Add(120*time.Second).UnixMilli() {
		t.Fatal("next turn budget")
	}
	r.Game.Phase = "catan_turn"
	r.TurnDeadline = later.Add(33 * time.Second).UnixMilli()
	r.Game.Phase = "catan_attack_city_treason_remove"
	r.adjustCatanResponseClock("catan_turn", -1, 0, later)
	r.Game.Phase = "catan_attack_city_treason_place"
	r.adjustCatanResponseClock("catan_attack_city_treason_remove", 1, 0, later.Add(10*time.Second))
	r.Game.Phase = "catan_turn"
	r.adjustCatanResponseClock("catan_attack_city_treason_place", 0, 0, later.Add(20*time.Second))
	if r.TurnDeadline != later.Add(53*time.Second).UnixMilli() {
		t.Fatal("treason lost action budget")
	}
}
