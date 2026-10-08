package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Three-to-six-player standalone games use public creation. Other recipes keep
// explicit internal provisioning, followed by actual ready/start and actions.
func TestCatanCitiesKnightsConfiguredFullHTTPGames(t *testing.T) {
	testCatanCitiesKnightsConfiguredFullHTTPGames(t, "", false, 3, 4, 5, 6)
}

func TestCatanCitiesKnightsSeafarersConfiguredFullHTTPGames(t *testing.T) {
	for _, scenario := range []string{"shores", "islands", "fog", "desert", "new_world", "wonders", "cloth"} {
		t.Run(scenario, func(t *testing.T) {
			players := []int{3, 4, 5, 6}
			testCatanCitiesKnightsConfiguredFullHTTPGames(t, scenario, false, players...)
		})
	}
}

func TestCatanFishingCitiesKnightsPublicFullHTTPGames(t *testing.T) {
	testCatanCitiesKnightsConfiguredFullHTTPGames(t, "fishing", false, 3, 4, 5, 6)
}

func TestCatanHarborsCitiesKnightsFullHTTPGames(t *testing.T) {
	testCatanCitiesKnightsConfiguredFullHTTPGames(t, "", true, 3, 4, 5, 6)
}

func testCatanCitiesKnightsConfiguredFullHTTPGames(t *testing.T, scenario string, harbors bool, players ...int) {
	testCatanCitiesKnightsVariantsFullHTTPGames(t, scenario, harbors, false, players...)
}
func TestCatanFriendlyKnightsPublicFullHTTPGames(t *testing.T) {
	for i, scene := range []string{"", "shores", "islands", "fog", "desert", "new_world", "wonders", "cloth"} {
		for _, harbors := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/harbors=%v", scene, harbors), func(t *testing.T) {
				offset := 0
				if harbors {
					offset = 2
				}
				testCatanCitiesKnightsVariantsFullHTTPGames(t, scene, harbors, true, 3+(i+offset)%4)
			})
		}
	}
}
func testCatanCitiesKnightsVariantsFullHTTPGames(t *testing.T, scenario string, harbors, friendly bool, players ...int) {
	testCatanCitiesKnightsEventsFullHTTPGames(t, scenario, harbors, friendly, false, players...)
}

func testCatanCitiesKnightsEventsFullHTTPGames(t *testing.T, scenario string, harbors, friendly, events bool, players ...int) {
	testCatanCitiesKnightsFishingFullHTTPGames(t, scenario, harbors, friendly, events, false, players...)
}

func testCatanCitiesKnightsFishingFullHTTPGames(t *testing.T, scenario string, harbors, friendly, events, fishing bool, players ...int) {
	if len(players) == 0 {
		players = []int{3, 6}
	}
	type recipe struct {
		players int
		layout  string
	}
	recipes := []recipe{}
	for _, n := range players {
		layouts := []string{""}
		if scenario != "" && scenario != "fishing" && n > 4 {
			for _, info := range game.CatanSeafarersScenarios(n) {
				if info.ID == scenario {
					layouts = info.Layouts
				}
			}
		}
		if scenario != "" && scenario != "fishing" && n <= 4 {
			layouts = []string{"fixed", "variable"}
			if harbors {
				if n == 3 {
					layouts = []string{"fixed"}
				} else {
					layouts = []string{"variable"}
				}
			}
			if scenario == "new_world" {
				layouts = []string{"prepared"}
			}
		}
		if fishing {
			layouts = []string{"fixed"}
			if scenario == "new_world" {
				layouts = []string{"prepared"}
			} else if n > 4 && scenario == "shores" {
				layouts = []string{"variable"}
			}
		}
		for _, layout := range layouts {
			recipes = append(recipes, recipe{n, layout})
		}
	}
	for _, config := range recipes {
		n, layout := config.players, config.layout
		name := fmt.Sprint(n)
		if layout != "" {
			name += "/" + layout
		}
		t.Run(name, func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, n+1)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("骑士玩家%d", p))
			}
			options := game.CatanOptions{FiveSix: n > 4}
			public := n <= 4 || scenario == "" || scenario == "fishing" || game.CatanCitiesKnightsSeafarersSupported(scenario)
			body := map[string]any{"kind": "catan", "name": "城市骑士验证", "capacity": n, "catanOptions": options, "catanFishing": fishing}
			if events {
				body["catanEvents"] = game.CatanEventCatalogue
			}
			if public {
				body["catanScenario"] = "cities-knights"
				if scenario != "" {
					body["catanScenario"] = scenario
					body["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
				}
			}
			if public && harbors {
				body["catanHarbors"] = game.CatanHarborsSetup{Enabled: true}
			}
			if friendly {
				body["catanFriendlyRobber"] = game.CatanFriendlyRobberSetup{Enabled: true}
			}
			r := clients[0].post("/api/rooms", body, 201)
			id := r["id"].(string)
			for p := 1; p < n; p++ {
				clients[p].command(current(clients[0]), "join", nil, 200)
			}
			if scenario != "" && scenario != "fishing" {
				if public {
					selectSeafarers(clients[0], &game.CatanSeafarersSetup{Scenario: scenario, Layout: layout}, 200)
				} else {
					provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: scenario})
				}
			}
			if !public {
				provisionCatanCitiesKnights(t, s, id)
			}
			if harbors && !public {
				provisionCatanHarbors(t, s, id)
			}
			for p := 0; p < n; p++ {
				clients[p].command(current(clients[0]), "ready", nil, 200)
			}
			clients[0].command(current(clients[0]), "start", nil, 200)
			state := s.rooms[id].Game
			if public && scenario != "" && scenario != "fishing" && (state.Catan.Seafarers == nil || state.Catan.CitiesKnights == nil || state.Catan.Seafarers.Layout != layout) {
				t.Fatal("public combination ignored selected layout")
			}
			if scenario == "fishing" && (state.Catan.Fishing == nil || state.Catan.CitiesKnights == nil || state.Catan.Seafarers != nil) {
				t.Fatal("missing combined fishing opening")
			}
			clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			restart := func() {
				before, _ := json.Marshal(s.rooms[id])
				ts.Close()
				s.Close()
				next, e := New(s.cfg, s.files)
				if e != nil {
					t.Fatal(e)
				}
				stopBotTicker(next)
				nextHTTP := httptest.NewServer(next.Handler())
				t.Cleanup(func() { nextHTTP.Close(); next.Close() })
				after, _ := json.Marshal(next.rooms[id])
				if string(before) != string(after) {
					t.Fatal("live city state changed on restart")
				}
				s, ts = next, nextHTTP
				for _, c := range clients {
					c.base = ts.URL
				}
			}
			restart()
			restartedRoll, restartedResponse := false, false
			steps, automatic, timeouts := 0, 0, 0
			for ; steps < 8000 && !s.rooms[id].Game.Finished; steps++ {
				room := s.rooms[id]
				state = room.Game
				g := state.Catan
				if friendly {
					assertPublicFriendlyProtection(t, state)
					if g.FriendlyRobber.Knights != game.CatanFriendlyKnightsRules {
						t.Fatal("lost combination recipe")
					}
				}
				if scenario == "tribe" {
					assertTribeProgressInventory(t, g)
				}
				if scenario == "fishing" || fishing {
					assertFishingCityInventory(t, g)
				}
				if harbors {
					assertPublicHarborPoints(t, state)
				}
				if scenario == "cloth" {
					c := g.Seafarers.Cloth
					total := c.Stock
					for _, held := range c.Held {
						total += held
					}
					for _, village := range c.Villages {
						total += village.Stock
					}
					want := 50
					if n > 4 {
						want = 70
					}
					if total != want+c.Issued || c.Issued < 0 || (n <= 4 && c.Issued != 0) || c.Stock < 0 {
						t.Fatal("HTTP cloth inventory", total, want)
					}
				}
				if (g.RollID >= 4 && !restartedRoll) || (state.CatanPendingActor() >= 0 && !restartedResponse) {
					if g.RollID >= 4 {
						restartedRoll = true
					}
					if state.CatanPendingActor() >= 0 {
						restartedResponse = true
					}
					restart()
					room = s.rooms[id]
					state = room.Game
					g = state.Catan
				}
				actor := state.Turn
				if p := state.CatanPendingActor(); p >= 0 {
					actor = p
				} else if state.Phase == "catan_discard" {
					for p, due := range g.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				if events && steps%53 == 0 {
					assertPublicEventsPrivacy(t, clients)
				}
				if steps%53 == 0 {
					for _, viewer := range []int{actor, n} {
						v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
						if scenario == "fishing" || fishing {
							fish := v["fishing"].(map[string]any)["tokens"].(map[string]any)
							if fish["drawPile"] != nil {
								t.Fatal("hidden fish supply")
							}
							for p, raw := range fish["players"].([]any) {
								_, visible := raw.(map[string]any)["tokens"]
								if visible != (viewer == p && len(g.Fishing.Tokens.Hands[p]) > 0) {
									t.Fatal("fish faces leaked")
								}
							}
						}
						if scenario == "tribe" {
							tr := v["seafarers"].(map[string]any)["tribe"].(map[string]any)
							for _, reward := range tr["development"].([]any) {
								if _, leak := reward.(map[string]any)["card"]; leak {
									t.Fatal("tribe reward leaked")
								}
							}
						}
						k := v["citiesKnights"].(map[string]any)
						if _, ok := k["progressDecks"]; ok {
							t.Fatal("hidden deck leaked")
						}
						for p, raw := range k["players"].([]any) {
							_, ok := raw.(map[string]any)["progress"]
							if ok != (p == viewer) {
								t.Fatal("progress hand leaked")
							}
						}
						for p, raw := range v["players"].([]any) {
							_, ok := raw.(map[string]any)["resources"]
							if ok != (p == viewer) {
								t.Fatal("resource hand leaked")
							}
						}
					}
				}
				if steps%29 == 0 && (g.SetupStep < g.SetupLimit() || state.CatanPendingActor() >= 0 || state.Phase == "catan_discard") {
					version := room.Version
					s.mu.Lock()
					room.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
					s.expireSetups(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("timeout failed to advance", state.Phase)
					}
					timeouts++
					reclaimTimeoutHumans(t, s, clients, id)
					continue
				}
				if steps%17 == 0 {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					version := s.rooms[id].Version
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					if s.rooms[id].Version <= version {
						t.Fatal("autoplay failed to advance", state.Phase)
					}
					if !s.rooms[id].Game.Finished {
						setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					}
					automatic++
					continue
				}
				action, e := state.BotAction(actor)
				if e != nil {
					t.Fatal(steps, state.Phase, e)
				}
				clients[actor].command(current(clients[actor]), "action", action, 200)
			}
			room := s.rooms[id]
			if !room.Game.Finished || room.Status != "finished" || len(room.Game.Winners) == 0 || (scenario != "cloth" && len(room.Game.Winners) != 1) || !restartedRoll || automatic == 0 || timeouts == 0 {
				t.Fatal("incomplete city game", steps, room.Game.Phase)
			}
			winner := room.Game.Winners[0]
			target := 13
			if scenario == "fishing" {
				assertFishingCityInventory(t, room.Game.Catan)
				if room.Game.Catan.Fishing.Tokens.BootOwner == winner {
					target++
				}
			}
			if harbors {
				target++
			}
			if sea := room.Game.Catan.Seafarers; sea != nil {
				target = sea.VictoryPoints
				if harbors {
					target++
				}
			}
			if fishing && room.Game.Catan.Fishing.Tokens.BootOwner == winner {
				target++
			}
			if (scenario != "wonders" && scenario != "cloth" && room.Game.Catan.Players[winner].Score < target) || room.Game.Catan.CitiesKnights.Invasions == 0 {
				t.Fatal("wrong expansion victory")
			}
			if scenario == "wonders" {
				level, rival := 0, 0
				for _, card := range room.Game.Catan.Seafarers.Wonders.Cards {
					if card.Owner == winner {
						level = card.Level
					} else if card.Owner >= 0 {
						rival = max(rival, card.Level)
					}
				}
				want := 12
				if harbors {
					want++
				}
				if fishing && room.Game.Catan.Fishing.Tokens.BootOwner == winner {
					want++
				}
				if target != want || (level != 4 && (level <= rival || room.Game.Catan.Players[winner].Score < target)) {
					t.Fatal("wrong combined wonder victory")
				}
			}
			if scenario == "cloth" {
				g := room.Game.Catan
				c := g.Seafarers.Cloth
				want := 16
				if harbors {
					want++
				}
				if fishing && g.Fishing.Tokens.BootOwner == winner {
					want++
				}
				if target != want {
					t.Fatal("wrong cloth target")
				}
				turnTarget := 16
				if harbors {
					turnTarget++
				}
				if fishing && g.Fishing.Tokens.BootOwner == room.Game.Turn {
					turnTarget++
				}
				if g.Players[room.Game.Turn].Score >= turnTarget {
					if !slices.Equal(room.Game.Winners, []int{room.Game.Turn}) {
						t.Fatal("cloth points priority")
					}
				} else {
					empty, best, held := 0, -1, -1
					for _, v := range c.Villages {
						if v.Stock == 0 {
							empty++
						}
					}
					winners := []int{}
					for p, player := range g.Players {
						if player.Eliminated {
							continue
						}
						if player.Score > best || (player.Score == best && c.Held[p] > held) {
							best, held, winners = player.Score, c.Held[p], []int{p}
						} else if player.Score == best && c.Held[p] == held {
							winners = append(winners, p)
						}
					}
					if empty < c.EmptyLimit || !slices.Equal(room.Game.Winners, winners) {
						t.Fatal("incorrect depletion result")
					}
				}
			}
			code, profile := clients[n].request("GET", "/api/players/"+room.Seats[winner].ID, nil)
			if code != 200 {
				t.Fatal(code)
			}
			stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
			history := profile["history"].([]any)[0].(map[string]any)
			if sea := room.Game.Catan.Seafarers; sea != nil {
				versions := history["catanExpansionRules"].(map[string]any)
				if scenario == "tribe" && versions["tribe_progress"] != game.CatanTribeProgressRules {
					t.Fatal("missing tribe progress version")
				}
				count := 2
				if fishing {
					count++
					if versions["fishing_sea_knights"] != game.CatanFishingSeaKnightsRules || versions["fishing"] != game.CatanFishingRules {
						t.Fatal("missing fishing sea knights history")
					}
				}
				if events {
					count++
				}
				if friendly {
					count++
				}
				if harbors {
					count++
				}
				if sea.NumberRecipe != "" && versions["number_recipe"] != sea.NumberRecipe {
					t.Fatal("missing sea number version")
				}
				if history["catanScenario"] != sea.Scenario || history["catanLayout"] != sea.Layout || history["catanRules"] != sea.Rules || versions["seafarers"] != sea.Rules || versions["cities_knights"] != room.Game.Catan.CitiesKnightsSetup().Rules || len(history["catanExpansions"].([]any)) != count {
					t.Fatal("missing combined frozen identity", history)
				}
			} else if history["catanLayout"] != "variable" || history["catanRules"] != room.Game.Catan.CitiesKnightsSetup().Rules || !slices.ContainsFunc(history["catanExpansions"].([]any), func(v any) bool { return v == "cities_knights" }) {
				t.Fatal("missing frozen expansion identity")
			}
			if events {
				rules := history["catanExpansionRules"].(map[string]any)
				if scenario == "cloth" && rules["event_cloth_fallback"] != game.CatanEventClothFallbackRules {
					t.Fatal("missing cloth event fallback history")
				}
				if rules["event_knights"] != game.CatanEventKnightsRules || rules["event_cards"] != game.CatanEventCatalogue {
					t.Fatal("missing actual event combination history", history)
				}
			}
			if scenario == "fishing" {
				rules := history["catanExpansionRules"].(map[string]any)
				if history["catanScenario"] != "fishing" || rules["fishing"] != game.CatanFishingRules || rules["cities_knights"] != room.Game.Catan.CitiesKnightsSetup().Rules {
					t.Fatal("wrong combined fishing history", history)
				}
				if n > 4 && rules["number_recipe"] != game.CatanExtendedNumberRecipe {
					t.Fatal("missing combined fishing number recipe")
				}
			}
			if friendly && scenario == "fishing" && history["catanExpansionRules"].(map[string]any)["friendly_fishing_fallback"] != game.CatanFriendlyFishingFallbackRules {
				t.Fatal("missing friendly fishing version")
			}
			if friendly && history["catanExpansionRules"].(map[string]any)["friendly_knights"] != game.CatanFriendlyKnightsRules {
				t.Fatal("missing friendly knights history")
			}
			if harbors && (room.Game.Catan.Harbors == nil || history["catanExpansionRules"].(map[string]any)["harbors"] != game.CatanHarborsRules) {
				t.Fatal("harbor rules missing from game/history")
			}
			if stats["wins"] != float64(1) || stats["played"] != float64(1) {
				t.Fatal("missing history result")
			}
			if public {
				matchID := room.MatchID
				var before, after string
				if err := s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", matchID).Scan(&before); err != nil {
					t.Fatal(err)
				}
				restart()
				s.mu.Lock()
				err := s.save(s.rooms[id])
				s.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				if err := s.db.QueryRow("SELECT snapshot FROM match_history WHERE id=?", matchID).Scan(&after); err != nil || before != after {
					t.Fatal("city archive changed after restart", err)
				}
				var count int
				if err := s.db.QueryRow("SELECT count(*) FROM rating_ledger WHERE match_id=?", matchID).Scan(&count); err != nil || count != n {
					t.Fatal("duplicate city ratings", count, err)
				}
			}
			clients[n].post("/api/rooms/"+id+"/watch", map[string]any{"leave": true}, 200)
			clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 400)
			t.Logf("steps=%d autoplay=%d timeout=%d responseRestart=%v", steps, automatic, timeouts, restartedResponse)
		})
	}
}

func TestCatanFishingKnightsVariantsPublicFullHTTPGames(t *testing.T) {
	for mode := 1; mode <= 3; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			testCatanCitiesKnightsVariantsFullHTTPGames(t, "fishing", mode&2 != 0, mode&1 != 0, 3+mode%4, 3+(mode+2)%4)
		})
	}
}
