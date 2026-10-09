package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanFishingAttackNaturalHTTP(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, ts, clients, id := newTradersKnightsHTTP(t, n, events, "barbarian-attack", knights)
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
							fish := view["fishing"].(map[string]any)
							if fish["attack"] != "catan-fishing-attack-2025" {
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
					if record["catanExpansionRules"].(map[string]any)["fishing_attack"] != "catan-fishing-attack-2025" {
						t.Fatal("history recipe")
					}
					t.Log("complete", r.Game.Round)
				})
			}
		}
	}
}

func TestCatanFishingAttackWaitingSwitchIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			c := newClient(t, ts.URL)
			c.register("捕鱼河流切换")
			body := map[string]any{"kind": "catan", "name": "切换", "capacity": n, "catanOptions": map[string]any{"fiveSix": false}, "catanFishing": true, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "catanEvents": game.CatanEventCatalogue}
			field, command := "catanScenario", "catan_scenario"
			if n == 2 {
				field, command = "catanTwoScenario", "catan_two_scenario"
			}
			body[field] = "barbarian-attack"
			raw := c.post("/api/rooms", body, 201)
			id := raw["id"].(string)
			c.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": false, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			if s.rooms[id].CatanCitiesKnights == nil || s.rooms[id].CatanEvents != game.CatanEventCatalogue {
				t.Fatal("disabling fish lost knights/events")
			}
			if s.rooms[id].CatanFishing {
				t.Fatal("fishing remained enabled")
			}
			c.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": true, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			target := ""
			if n > 4 {
				target = "transport"
			}
			c.post("/api/rooms/"+id, map[string]any{"type": command, field: target, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			// Transport now supports fishing; preserve the compatible toggle.
			// Returning to the base scenario still clears it.
			if s.rooms[id].CatanFishing != (target == "transport") {
				t.Fatal("fishing toggle does not match destination scenario")
			}
		})
	}
}

func TestCatanFishingAttackKnightFishHTTPClockRestartTimeout(t *testing.T) {
	for _, knights := range []bool{false, true} {
		t.Run(fmt.Sprint(knights), func(t *testing.T) {
			s, ts, clients, id := newTradersKnightsHTTP(t, 2, true, "barbarian-attack", knights)
			for step := 0; step < 1500; step++ {
				r := s.rooms[id]
				if r.Game.Phase == "catan_turn" {
					break
				}
				actor := twoHTTPActor(r.Game)
				a, e := r.Game.BotAction(actor)
				if e != nil {
					t.Fatal(e)
				}
				clients[actor].command(current(clients[actor]), "action", a, 200)
			}
			r := s.rooms[id]
			actor := r.Game.Turn
			// Fixture injects pieces through JSON while preserving token inventories.
			raw, _ := json.Marshal(r.Game)
			var state map[string]any
			json.Unmarshal(raw, &state)
			g := state["catan"].(map[string]any)
			attack := g["attack"].(map[string]any)
			castle := int(attack["map"].(map[string]any)["castles"].([]any)[0].(float64))
			edge := -1
			for _, e := range r.Game.Catan.Edges {
				for _, tile := range e.Tiles {
					if tile == castle {
						edge = e.ID
						break
					}
				}
				if edge >= 0 {
					break
				}
			}
			if knights {
				attack["city"].(map[string]any)["knights"] = []any{map[string]any{"owner": actor, "edge": edge, "strength": 1, "active": false, "activatedAt": 0, "promotedAt": 0}}
			} else {
				neutralEdge := -1
				for _, e := range r.Game.Catan.Edges {
					isCastle := false
					for _, tile := range e.Tiles {
						if tile == castle {
							isCastle = true
						}
					}
					if !isCastle {
						neutralEdge = e.ID
						break
					}
				}
				attack["knights"] = []any{map[string]any{"player": actor, "edge": edge}, map[string]any{"player": -2, "edge": neutralEdge}}
			}
			raw, _ = json.Marshal(state)
			json.Unmarshal(raw, r.Game)
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_end"}, 200)
			r = s.rooms[id]
			deadline := r.TurnDeadline
			if left := deadline - time.Now().UnixMilli(); left < 119000 || left > 120000 {
				t.Fatal("not two minutes", left)
			}
			view := current(clients[actor])["game"].(map[string]any)["catan"].(map[string]any)["attack"].(map[string]any)
			action := game.Action{Choice: "fish", Edge: edge}
			var fishTokens []any
			if knights {
				city := view["city"].(map[string]any)
				plan := city["plan"].(map[string]any)
				choices := city["choices"].(map[string]any)
				move := choices["moves"].([]any)[0].(map[string]any)
				action.Type = "catan_attack_city_move"
				action.Prompt = int(plan["id"].(float64))
				action.Target = int(move["fish"].([]any)[0].(float64))
				fishTokens = choices["fishTokens"].([]any)
			} else {
				plan := view["endPlan"].(map[string]any)
				move := view["moveChoices"].([]any)[0].(map[string]any)
				action.Type = "catan_attack_move"
				action.Prompt = int(plan["id"].(float64))
				action.Target = int(move["fish"].([]any)[0].(float64))
				fishTokens = view["fishTokens"].([]any)
			}
			for _, raw := range fishTokens {
				token := raw.(map[string]any)
				if token["fish"].(float64) >= 2 {
					action.Tokens = []int{int(token["id"].(float64))}
					break
				}
			}
			if len(action.Tokens) == 0 {
				t.Fatal("starting fish unavailable")
			}
			before := len(r.Game.Catan.Fishing.Tokens.Hands[actor])
			clients[actor].command(current(clients[actor]), "action", action, 200)
			if s.rooms[id].TurnDeadline != deadline || len(s.rooms[id].Game.Catan.Fishing.Tokens.Hands[actor]) != before {
				t.Fatal("draft reset clock/spent")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			_ = ts
			if s.rooms[id].TurnDeadline != deadline {
				t.Fatal("restart reset clock")
			}
			r = s.rooms[id]
			s.mu.Lock()
			s.expireSetups(time.UnixMilli(deadline + 1))
			s.mu.Unlock()
			if !s.rooms[id].Seats[actor].AutoPlay {
				t.Fatal("timeout did not enable autoplay")
			}
			for step := 0; step < 20 && (s.rooms[id].Game.Phase == "catan_attack_end" || s.rooms[id].Game.Phase == "catan_attack_city_move"); step++ {
				s.mu.Lock()
				s.rooms[id].BotAt = 0
				s.runBots(time.UnixMilli(deadline + 2000 + int64(step)*2000))
				s.mu.Unlock()
			}
			r = s.rooms[id]
			if r.Game.Phase == "catan_attack_end" || r.Game.Phase == "catan_attack_city_move" || len(r.Game.Catan.Fishing.Tokens.Hands[actor]) != before-1 {
				t.Fatal("timeout failed to commit one fish payment", r.Game.Phase)
			}
		})
	}
}
