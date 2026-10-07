package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanHarborsPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, c := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("公开港口房主")
	c.register("公开港口朋友")
	for _, body := range []map[string]any{
		{"kind": "splendor", "capacity": 3},
		{"kind": "catan", "capacity": 2},
		{"kind": "catan", "capacity": 5, "catanOptions": game.CatanOptions{FiveSix: true}},
		{"kind": "catan", "capacity": 3, "catanScenario": "fishing"},
		{"kind": "catan", "capacity": 3, "catanScenario": "transport"},
		{"kind": "catan", "capacity": 3, "catanScenario": "islands", "catanFishing": true},
		{"kind": "catan", "capacity": 3, "catanOptions": game.CatanOptions{Helpers: true}},
	} {
		body["name"], body["catanHarbors"] = "非法组合", game.CatanHarborsSetup{Enabled: true}
		h.post("/api/rooms", body, 400)
	}
	raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "公开港口", "capacity": 4, "catanScenario": "islands", "catanHarbors": game.CatanHarborsSetup{Enabled: true}}, 201)
	id := raw["id"].(string)
	c.command(raw, "join", nil, 200)
	h.command(current(h), "add_bot", nil, 200)
	ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
	ready()
	before, _ := json.Marshal(s.rooms[id])
	selectCatanHarbors(c, false, 400)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": true, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid setting changed room")
	}
	selectCatanHarbors(h, true, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same choice cleared ready")
		}
	}
	selectCatanHarbors(h, false, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("changed choice retained ready")
		}
	}
	// A disabled award must not block switching to Fishing, nor change its target.
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": true, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	selectCatanHarbors(h, true, 400)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": false, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	selectCatanHarbors(h, true, 200)
	selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 200)
	ready()
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 3 || g.Harbors == nil || g.CitiesKnights == nil || g.Seafarers.Scenario != "islands" {
		t.Fatal("actual public triple start")
	}
	selectCatanHarbors(h, false, 400)
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	if !s.rooms[id].CatanHarbors.Enabled {
		t.Fatal("rematch lost award")
	}
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "rivers", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	if s.rooms[id].CatanHarbors != nil {
		t.Fatal("standalone retained incompatible award")
	}
}

func assertPublicHarborPoints(t *testing.T, state *game.State) {
	t.Helper()
	g := state.Catan
	points := make([]int, len(g.Players))
	seen := map[int]bool{}
	for _, port := range g.Ports {
		edge := g.Edges[port.Edge]
		for _, id := range []int{edge.A, edge.B} {
			if seen[id] {
				continue
			}
			seen[id] = true
			v := g.Vertices[id]
			if v.Owner >= 0 && v.Owner < len(points) && !g.Players[v.Owner].Eliminated {
				points[v.Owner] += v.Level
			}
		}
	}
	view := state.View(-1)["catan"].(map[string]any)["harbors"].(map[string]any)
	raw, _ := json.Marshal(view["points"])
	var actual []int
	json.Unmarshal(raw, &actual)
	if !slices.Equal(points, actual) {
		t.Fatal("incorrect public port points", points, actual)
	}
	owner := g.Harbors.Owner
	if owner >= 0 && (points[owner] < 3 || points[owner] < slices.Max(points)) {
		t.Fatal("award holder below threshold or rival")
	}
}

func TestCatanHarborsSeaPublicFullHTTPGames(t *testing.T) {
	for _, scene := range []string{"", "shores", "islands", "fog", "desert", "tribe", "cloth", "pirate_islands", "wonders", "new_world"} {
		for _, n := range []int{3, 4} {
			t.Run(fmt.Sprintf("%s/%d", scene, n), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for p := range clients {
					clients[p] = newClient(t, ts.URL)
					clients[p].register(fmt.Sprintf("港口整局%d", p))
				}
				raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "公开港口整局", "capacity": n, "catanScenario": scene, "catanHarbors": game.CatanHarborsSetup{Enabled: true}}, 201)
				id := raw["id"].(string)
				for p := 1; p < n; p++ {
					clients[p].command(current(clients[0]), "join", nil, 200)
				}
				if scene != "" {
					layout := "fixed"
					if n == 4 && scene != "pirate_islands" {
						layout = "variable"
					}
					if scene == "new_world" {
						layout = "prepared"
					}
					selectSeafarers(clients[0], &game.CatanSeafarersSetup{Scenario: scene, Layout: layout}, 200)
				}
				for p := 0; p < n; p++ {
					clients[p].command(current(clients[0]), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				expected := map[string]int{"": 11, "shores": 15, "islands": 14, "fog": 13, "desert": 15, "tribe": 14, "cloth": 15, "pirate_islands": 11, "wonders": 11, "new_world": 13}[scene]
				steps, automatic, timeouts := 0, 0, 0
				for ; steps < 10000 && !s.rooms[id].Game.Finished; steps++ {
					r := s.rooms[id]
					state := r.Game
					if steps%151 == 0 {
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						r, state = s.rooms[id], s.rooms[id].Game
					}
					assertPublicHarborPoints(t, state)
					g := state.Catan
					for color, total := range g.Bank {
						for _, p := range g.Players {
							total += p.Resources[color]
						}
						if total != 19 {
							t.Fatal("resource supply", color, total)
						}
					}
					if scene != "" {
						assertPublicFishSeaSpecialInventory(t, g)
					}
					actor := twoHTTPActor(state)
					if steps%53 == 0 {
						for _, viewer := range []int{actor, n} {
							v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
							if v["victoryTarget"] != float64(expected) || v["harbors"] == nil {
								t.Fatal("wrong combined target", v["victoryTarget"], expected)
							}
							for p, raw := range v["players"].([]any) {
								for _, key := range []string{"resources", "dev"} {
									if (raw.(map[string]any)[key] != nil) != (p == viewer) {
										t.Fatal("private hand exposed")
									}
								}
							}
						}
					}
					version := r.Version
					if steps%101 == 0 {
						s.mu.Lock()
						s.expireSetups(time.UnixMilli(r.TurnDeadline))
						s.mu.Unlock()
						timeouts++
						if !s.rooms[id].Game.Finished {
							reclaimTimeoutHumans(t, s, clients, id)
						}
					} else if steps%19 == 0 {
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
							t.Fatal(steps, state.Phase, err)
						}
						clients[actor].command(current(clients[actor]), "action", a, 200)
					}
					if s.rooms[id].Version <= version {
						t.Fatal("stalled", steps, state.Phase)
					}
				}
				r := s.rooms[id]
				if r.Status != "finished" || !r.Game.Finished || automatic == 0 || timeouts == 0 {
					t.Fatal("incomplete game", steps)
				}
				assertPublicHarborPoints(t, r.Game)
				winner := r.Game.Winners[0]
				if scene == "cloth" {
					assertPublicHarborsClothVictory(t, r.Game, expected)
				} else if scene == "wonders" {
					level, rival := 0, 0
					for _, c := range r.Game.Catan.Seafarers.Wonders.Cards {
						if c.Owner == winner {
							level = c.Level
						} else if c.Owner >= 0 {
							rival = max(rival, c.Level)
						}
					}
					if level != 4 && (level <= rival || r.Game.Catan.Players[winner].Score < expected) {
						t.Fatal("wrong wonder victory")
					}
				} else if r.Game.Catan.Players[winner].Score < expected {
					t.Fatal("wrong score threshold")
				}
				if scene == "pirate_islands" && r.Game.Catan.Seafarers.PirateIslands.Fortresses[winner].Strength != 0 {
					t.Fatal("fortress not recovered")
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				if err := s.save(s.rooms[id]); err != nil {
					t.Fatal(err)
				}
				_, profile := clients[n].request("GET", "/api/players/"+r.Seats[winner].ID, nil)
				records := profile["history"].([]any)
				if len(records) != 1 {
					t.Fatal("duplicate history")
				}
				record := records[0].(map[string]any)
				if record["catanExpansionRules"].(map[string]any)["harbors"] != game.CatanHarborsRules {
					t.Fatal("missing award history")
				}
				if scene != "" && record["catanScenario"] != scene {
					t.Fatal("wrong map history")
				}
				var count int
				if err := s.db.QueryRow("SELECT count(*) FROM rating_ledger WHERE match_id=?", r.MatchID).Scan(&count); err != nil || count != n {
					t.Fatal("duplicate score", count, err)
				}
				t.Logf("steps=%d autoplay=%d timeout=%d", steps, automatic, timeouts)
			})
		}
	}
}

func assertPublicHarborsClothVictory(t *testing.T, s *game.State, target int) {
	t.Helper()
	g := s.Catan
	c := g.Seafarers.Cloth
	if g.Players[s.Turn].Score >= target {
		if !slices.Equal(s.Winners, []int{s.Turn}) {
			t.Fatal("cloth point priority")
		}
		return
	}
	empty, best, held := 0, -1, -1
	for _, v := range c.Villages {
		if v.Stock == 0 {
			empty++
		}
	}
	winners := []int{}
	for p, seat := range g.Players {
		if seat.Eliminated {
			continue
		}
		if seat.Score > best || seat.Score == best && c.Held[p] > held {
			best, held, winners = seat.Score, c.Held[p], []int{p}
		} else if seat.Score == best && c.Held[p] == held {
			winners = append(winners, p)
		}
	}
	if empty < 5 || !slices.Equal(s.Winners, winners) {
		t.Fatal("wrong cloth depletion result")
	}
}

func TestCatanHarborsSeaKnightsPublicFullHTTPGames(t *testing.T) {
	for _, scene := range []string{"shores", "islands", "fog", "desert", "new_world", "wonders", "cloth"} {
		t.Run(scene, func(t *testing.T) { testCatanCitiesKnightsConfiguredFullHTTPGames(t, scene, true, 3, 4) })
	}
}
