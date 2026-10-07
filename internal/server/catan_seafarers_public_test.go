package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanSeafarersPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("航海公开房主")
	guest.register("航海公开朋友")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2, "catanScenario": "shores"},
		{"kind": "catan", "capacity": 5, "catanScenario": "shores", "catanOptions": game.CatanOptions{FiveSix: true}},
		{"kind": "catan", "capacity": 3, "catanScenario": "cloth"},
		{"kind": "catan", "capacity": 3, "catanScenario": "new_world"},
		{"kind": "splendor", "capacity": 3, "catanScenario": "fog"},
	} {
		body["name"] = "无效航海配置"
		host.post("/api/rooms", body, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "公开航海配置", "capacity": 4, "catanScenario": "shores", "catanOptions": game.CatanOptions{Helpers: true, AllHelpers: true}}, 201)
	id := raw["id"].(string)
	if len(raw["catanSeafarersChoices"].([]any)) != 7 || s.rooms[id].CatanSeafarers.Layout != "fixed" {
		t.Fatal("wrong public catalogue or default layout")
	}
	guest.command(current(host), "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	before, _ := json.Marshal(s.rooms[id])
	selectSeafarers(guest, &game.CatanSeafarersSetup{Scenario: "fog"}, 400)
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "cloth"}, 400)
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "shores", Rules: "unknown"}, 400)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: true}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid public configuration changed room")
	}
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "shores"}, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same setup cleared readiness")
		}
	}
	request := map[string]any{"type": "catan_seafarers", "catanSeafarers": game.CatanSeafarersSetup{Scenario: "fog", Layout: "variable"}, "version": s.rooms[id].Version, "nonce": randomID(12)}
	host.post("/api/rooms/"+id, request, 200)
	if s.rooms[id].CatanScenario != "fog" {
		t.Fatal("map picker failed to synchronize public scenario")
	}
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("changed map did not clear only human readiness")
		}
	}
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	before, _ = json.Marshal(s.rooms[id])
	host.post("/api/rooms/"+id, request, 200)
	after, _ = json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("replay changed configuration")
	}
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 3 || g.Seafarers.Scenario != "fog" || g.Seafarers.Layout != "variable" || !g.Options.Helpers || g.EventDeck != nil {
		t.Fatal("public start did not use actual player count and selected rules")
	}
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "shores"}, 400)
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	if s.rooms[id].CatanSeafarers != nil || !s.rooms[id].CatanOptions.Helpers {
		t.Fatal("returning to base kept sea map or lost helpers")
	}
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "desert", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	if s.rooms[id].CatanSeafarers.Scenario != "desert" {
		t.Fatal("base room could not enter public seafarers")
	}
}

func TestCatanSeafarersPublicCompleteHTTPGames(t *testing.T) {
	for _, scenario := range []string{"shores", "islands", "fog", "desert", "tribe", "pirate_islands", "wonders"} {
		for _, n := range []int{3, 4} {
			for _, helpers := range []bool{false, true} {
				for _, layout := range []string{"fixed", "variable"} {
					if scenario == "pirate_islands" && layout == "variable" {
						continue // The official scenario has only a fixed recipe.
					}
					t.Run(fmt.Sprintf("%s/%d/helpers=%v/%s", scenario, n, helpers, layout), func(t *testing.T) {
						s, ts := setupServer(t)
						stopBotTicker(s)
						clients := make([]*testClient, n+1)
						for p := range clients {
							clients[p] = newClient(t, ts.URL)
							clients[p].register(fmt.Sprintf("公开航海%d", p))
						}
						raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "公开航海完整局", "capacity": n, "catanScenario": scenario, "catanOptions": game.CatanOptions{Helpers: helpers, AllHelpers: helpers && n == 4}}, 201)
						id := raw["id"].(string)
						for p := 1; p < n; p++ {
							clients[p].command(current(clients[0]), "join", nil, 200)
						}
						selectSeafarers(clients[0], &game.CatanSeafarersSetup{Scenario: scenario, Layout: layout}, 200)
						for p := 0; p < n; p++ {
							clients[p].command(current(clients[p]), "ready", nil, 200)
						}
						clients[0].command(current(clients[0]), "start", nil, 200)
						clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
						if s.rooms[id].Game.Catan.EventDeck != nil {
							t.Fatal("public room gained reference events")
						}
						automatic, timeouts := 0, 0
						for step := 0; !s.rooms[id].Game.Finished && step < 12000; step++ {
							r := s.rooms[id]
							state := r.Game
							actor := twoHTTPActor(state)
							if step%173 == 0 {
								s, ts = restartRiversHTTP(t, s, ts, clients, id)
								r, state = s.rooms[id], s.rooms[id].Game
							}
							if step%97 == 0 {
								for _, viewer := range []int{actor, n} {
									v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
									if v["seafarers"].(map[string]any)["scenario"] != scenario || v["eventDeck"] != nil {
										t.Fatal("wrong public rules")
									}
									if scenario == "tribe" {
										tribe := v["seafarers"].(map[string]any)["tribe"].(map[string]any)
										for _, reward := range tribe["development"].([]any) {
											if _, exposed := reward.(map[string]any)["card"]; exposed {
												t.Fatal("unclaimed tribe card identity exposed")
											}
										}
									}
									for p, raw := range v["players"].([]any) {
										for _, field := range []string{"resources", "dev"} {
											if (raw.(map[string]any)[field] != nil) != (p == viewer) {
												t.Fatal("private hand exposed")
											}
										}
									}
								}
							}
							version := r.Version
							if step%101 == 0 {
								s.mu.Lock()
								s.expireSetups(time.UnixMilli(r.TurnDeadline))
								s.mu.Unlock()
								timeouts++
								if !s.rooms[id].Game.Finished {
									reclaimTimeoutHumans(t, s, clients, id)
								}
							} else if step%19 == 0 {
								setAutoPlay(clients[actor], current(clients[actor]), true, 200)
								s.mu.Lock()
								s.rooms[id].BotAt = 0
								s.runBots(time.Now())
								s.mu.Unlock()
								automatic++
								if !s.rooms[id].Game.Finished {
									setAutoPlay(clients[actor], current(clients[actor]), false, 200)
								}
							} else {
								a, err := state.BotAction(actor)
								if err != nil {
									t.Fatal(step, state.Phase, err)
								}
								clients[actor].command(current(clients[actor]), "action", a, 200)
							}
							if s.rooms[id].Version <= version {
								t.Fatal("public match stalled", step, state.Phase)
							}
						}
						r := s.rooms[id]
						if r.Status != "finished" || !r.Game.Finished || len(r.Game.Winners) != 1 || automatic == 0 || timeouts == 0 {
							t.Fatal("natural match failed to finish")
						}
						winner := r.Game.Winners[0]
						assertPublicSeaVictory(t, r.Game, winner)
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						code, profile := clients[n].request("GET", "/api/players/"+r.Seats[winner].ID, nil)
						if code != 200 {
							t.Fatal(code)
						}
						record := profile["history"].([]any)[0].(map[string]any)
						if record["catanScenario"] != scenario || record["catanExpansionRules"].(map[string]any)["seafarers"] != game.CatanSeafarersRules {
							t.Fatal("scenario missing from history")
						}
						stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
						if stats["played"] != float64(1) || stats["wins"] != float64(1) {
							t.Fatal("duplicate or missing result")
						}
					})
				}
			}
		}
	}
}

func TestCatanSeafarersPublicRejectsCorruptSavedSetup(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("海图配置恢复")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "海图配置恢复", "capacity": 3, "catanScenario": "shores"}, 201)
	id := raw["id"].(string)
	for range 2 {
		host.command(current(host), "add_bot", nil, 200)
	}
	host.command(current(host), "ready", nil, 200)
	valid, _ := json.Marshal(s.rooms[id])
	for _, damage := range []func(*Room){
		func(r *Room) { r.CatanSeafarers = nil },
		func(r *Room) { r.CatanSeafarers.Scenario = "fog" },
		func(r *Room) { r.CatanSeafarers.Layout = "prepared" },
		func(r *Room) { r.CatanSeafarers.Rules = "wrong" },
		func(r *Room) { r.CatanOptions.AllHelpers = true },
		func(r *Room) { r.CatanOptions.FiveSix = true },
		func(r *Room) { r.CatanTwoRules = game.CatanTwoRules },
		func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} },
		func(r *Room) { r.CatanFriendlyRobber = &game.CatanFriendlyRobberSetup{} },
		func(r *Room) { r.CatanHarbors = &game.CatanHarborsSetup{} },
		func(r *Room) { r.CatanBaseConfiguration = &game.CatanBaseConfiguration{} },
		func(r *Room) { r.CatanNewWorldMap = &game.CatanNewWorldMap{} },
	} {
		var r Room
		if err := json.Unmarshal(valid, &r); err != nil {
			t.Fatal(err)
		}
		damage(&r)
		s.rooms[id] = &r
		before, _ := json.Marshal(&r)
		host.command(current(host), "start", nil, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) || s.rooms[id].Game != nil {
			t.Fatal("invalid save started or partially changed")
		}
	}
}

func assertPublicSeaVictory(t *testing.T, s *game.State, winner int) {
	t.Helper()
	g := s.Catan
	sea := g.Seafarers
	if sea.Scenario == "wonders" {
		level, otherMax := -1, 0
		for _, card := range sea.Wonders.Cards {
			if card.Owner == winner {
				level = card.Level
			} else if card.Owner >= 0 {
				otherMax = max(otherMax, card.Level)
			}
		}
		if level != 4 && !(level > otherMax && g.Players[winner].Score >= sea.VictoryPoints) {
			t.Fatal("invalid wonder victory", level, otherMax, g.Players[winner].Score)
		}
		return
	}
	if g.Players[winner].Score < sea.VictoryPoints {
		t.Fatal("wrong scenario victory threshold")
	}
	if sea.Scenario == "pirate_islands" && sea.PirateIslands.Fortresses[winner].Strength != 0 {
		t.Fatal("pirate winner has not recovered their fortress")
	}
}

func TestCatanSeafarersPublicSpecialScenarioSwitches(t *testing.T) {
	r := &Room{Kind: "catan", Status: "waiting", Capacity: 3, CatanOptions: game.CatanOptions{Helpers: true}, Seats: []Seat{{Ready: true}, {Bot: true, Ready: true}}}
	for _, scenario := range []string{"tribe", "wonders", "pirate_islands"} {
		if err := r.setCatanScenario(scenario); err != nil {
			t.Fatal(err)
		}
		if r.CatanSeafarers.Scenario != scenario || r.CatanSeafarers.Layout != "fixed" || !r.CatanOptions.Helpers || r.Seats[0].Ready || !r.Seats[1].Ready {
			t.Fatal("scenario change did not retain helpers and reset readiness correctly")
		}
		r.Seats[0].Ready = true
		if scenario != "pirate_islands" {
			if err := r.setCatanSeafarers(game.CatanSeafarersSetup{Scenario: scenario, Layout: "variable"}); err != nil {
				t.Fatal(err)
			}
			r.Seats[0].Ready = true
			if err := r.setCatanScenario(scenario); err != nil || r.CatanSeafarers.Layout != "variable" || !r.Seats[0].Ready {
				t.Fatal("same scenario lost layout or readiness", err)
			}
		} else {
			before, _ := json.Marshal(r)
			if err := r.setCatanSeafarers(game.CatanSeafarersSetup{Scenario: scenario, Layout: "variable"}); err == nil {
				t.Fatal("pirate islands accepted an unprinted variable recipe")
			}
			after, _ := json.Marshal(r)
			if string(before) != string(after) {
				t.Fatal("invalid layout changed setup")
			}
		}
	}
}
