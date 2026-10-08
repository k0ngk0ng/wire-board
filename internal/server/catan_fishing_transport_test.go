package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanFishingTransportNaturalHTTP(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, ts, clients, id := newTradersKnightsHTTP(t, n, events, "transport", knights)
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
							if fish["transport"] != "catan-fishing-transport-2025" {
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
					if record["catanExpansionRules"].(map[string]any)["fishing_transport"] != "catan-fishing-transport-2025" {
						t.Fatal("history recipe")
					}
					t.Log("complete", r.Game.Round)
				})
			}
		}
	}
}

func TestCatanFishingTransportWaitingSwitchIsolation(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			c := newClient(t, ts.URL)
			c.register("捕鱼运输切换")
			body := map[string]any{"kind": "catan", "name": "切换", "capacity": n, "catanOptions": map[string]any{"fiveSix": false}, "catanFishing": true, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "catanEvents": game.CatanEventCatalogue}
			field, command := "catanScenario", "catan_scenario"

			body[field] = "transport"
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
			if n == 2 {
				field, command = "catanTwoScenario", "catan_two_scenario"
			}
			if n > 4 {
				target = "barbarian-attack"
			}
			if n > 4 {
				c.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": false, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			}
			c.post("/api/rooms/"+id, map[string]any{"type": command, field: target, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			if s.rooms[id].CatanFishing {
				t.Fatal("base retained fishing")
			}
		})
	}
}

func TestCatanFishingTransportFishHTTPClockRestartTimeout(t *testing.T) {
	for _, knights := range []bool{false, true} {
		t.Run(fmt.Sprint(knights), func(t *testing.T) {
			s, ts, clients, id := newTradersKnightsHTTP(t, 2, true, "transport", knights)
			for step := 0; step < 1500 && s.rooms[id].Game.Phase != "catan_turn"; step++ {
				r := s.rooms[id]
				actor := twoHTTPActor(r.Game)
				a, err := r.Game.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				clients[actor].command(current(clients[actor]), "action", a, 200)
			}
			actor := s.rooms[id].Game.Turn
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_end"}, 200)
			r := s.rooms[id]
			deadline := r.TurnDeadline
			if left := deadline - time.Now().UnixMilli(); left < 119000 || left > 120000 {
				t.Fatal("not two minutes", left)
			}
			view := current(clients[actor])["game"].(map[string]any)["catan"].(map[string]any)["transport"].(map[string]any)
			choices := view["choices"].(map[string]any)
			ids := []int{}
			paid := 0
			for _, raw := range choices["fishTokens"].([]any) {
				token := raw.(map[string]any)
				ids = append(ids, int(token["id"].(float64)))
				paid += int(token["fish"].(float64))
				if paid >= int(choices["fishCost"].(float64)) {
					break
				}
			}
			before := len(r.Game.Catan.Fishing.Tokens.Hands[actor])
			points := r.Game.Catan.Transport.Travel.Points
			seq := int(r.Game.Catan.Transport.Sequence)
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_transport_fish", Offer: seq, Tokens: ids}, 200)
			if s.rooms[id].TurnDeadline != deadline || s.rooms[id].Game.Catan.Transport.Travel.Points != points+2 {
				t.Fatal("fish reset clock or lost boost")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			_ = ts
			r = s.rooms[id]
			if r.TurnDeadline != deadline || len(r.Game.Catan.Fishing.Tokens.Hands[actor]) != before-len(ids) || !r.Game.Catan.Transport.Travel.FishUsed {
				t.Fatal("restart lost paid boost")
			}
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_transport_fish", Offer: seq, Tokens: ids}, 400)
			clients[actor].command(current(clients[actor]), "action", game.Action{Type: "catan_transport_wheat", Offer: seq}, 400)
			s.mu.Lock()
			s.expireSetups(time.UnixMilli(deadline + 1))
			s.mu.Unlock()
			if !s.rooms[id].Seats[actor].AutoPlay {
				t.Fatal("timeout did not enable autoplay")
			}
			for step := 0; step < 40 && s.rooms[id].Game.Phase == "catan_transport_move" && s.rooms[id].Game.Turn == actor; step++ {
				s.mu.Lock()
				s.rooms[id].BotAt = 0
				s.runBots(time.UnixMilli(deadline + 2000 + int64(step)*2000))
				s.mu.Unlock()
			}
			if s.rooms[id].Game.Phase == "catan_transport_move" && s.rooms[id].Game.Turn == actor {
				t.Fatal("autoplay stuck in travel")
			}
			if len(s.rooms[id].Game.Catan.Fishing.Tokens.Hands[actor]) != before-len(ids) {
				t.Fatal("autoplay charged fish twice")
			}
		})
	}
}
