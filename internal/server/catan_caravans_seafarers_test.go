package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"os"
	"testing"
	"time"
)

func TestCatanCaravansSeaPublicHTTP(t *testing.T) { runCaravansSeaHTTP(t, false, "caravans-desert") }
func TestCatanCaravansSeaEventsHTTP(t *testing.T) { runCaravansSeaHTTP(t, true, "caravans-desert") }
func TestCatanCaravansTribeHTTP(t *testing.T)     { runCaravansSeaHTTP(t, true, "caravans-tribe") }
func TestCatanCaravansShoresHTTP(t *testing.T)    { runCaravansSeaHTTP(t, true, "caravans-shores") }
func TestCatanCaravansIslandsHTTP(t *testing.T)   { runCaravansSeaHTTP(t, true, "caravans-islands") }
func TestCatanCaravansIslandsOrdinaryHTTP(t *testing.T) {
	runCaravansSeaHTTP(t, false, "caravans-islands")
}
func TestCatanCaravansWorldHTTP(t *testing.T) { runCaravansSeaHTTP(t, true, "caravans-new-world") }
func runCaravansSeaHTTP(t *testing.T, events bool, scenario string) {
	runCaravansSeaHTTPOptions(t, events, scenario, false)
}
func TestCatanCaravansHelpersHTTP(t *testing.T) {
	for _, scene := range []string{"caravans-shores", "caravans-islands", "caravans-desert", "caravans-tribe", "caravans-new-world"} {
		t.Run(scene, func(t *testing.T) { runCaravansSeaHTTPOptions(t, true, scene, true) })
	}
}
func runCaravansSeaHTTPOptions(t *testing.T, events bool, scenario string, helpers bool) {
	runCaravansSeaHTTPVariants(t, events, scenario, helpers, false)
}
func TestCatanCaravansVariantsHTTP(t *testing.T) {
	for _, scene := range []string{"caravans-islands", "caravans-desert", "caravans-new-world", "caravans-shores"} {
		t.Run(scene, func(t *testing.T) { runCaravansSeaHTTPVariants(t, true, scene, true, true) })
	}
}
func runCaravansSeaHTTPVariants(t *testing.T, events bool, scenario string, helpers, variants bool) {
	counts := []int{2, 3, 4, 5, 6}
	if scenario == "caravans-islands" {
		counts = []int{2, 3, 4, 5, 6}
	}
	for _, n := range counts {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			clients := make([]*testClient, n)
			for p := range clients {
				clients[p] = newClient(t, ts.URL)
				clients[p].register(fmt.Sprintf("航海商队%d", p))
			}
			h := clients[0]
			recipe := map[string]any{"kind": "catan", "name": "商队沙漠", "capacity": n, "catanScenario": scenario}
			if events {
				recipe["catanEvents"] = game.CatanEventCatalogue
			}
			if variants {
				recipe["catanFriendlyRobber"] = game.CatanFriendlyRobberSetup{Enabled: true}
				recipe["catanHarbors"] = game.CatanHarborsSetup{Enabled: true}
			}
			if helpers {
				recipe["catanOptions"] = game.CatanOptions{Helpers: true, AllHelpers: true}
			}
			raw := h.post("/api/rooms", recipe, 201)
			id := raw["id"].(string)
			for p := 1; p < n; p++ {
				clients[p].command(current(h), "join", nil, 200)
			}
			for _, c := range clients {
				c.command(current(c), "ready", nil, 200)
			}
			if scenario == "caravans-new-world" {
				clients[1].command(current(clients[1]), "catan_world_map_shuffle", nil, 400)
				h.command(current(h), "catan_world_map_shuffle", nil, 200)
				for _, seat := range s.rooms[id].Seats {
					if seat.Ready {
						t.Fatal("map edit retained ready")
					}
				}
				for _, c := range clients {
					c.command(current(c), "ready", nil, 200)
				}
			}
			h.command(current(h), "start", nil, 200)
			if scenario == "caravans-new-world" {
				expected, e := game.NewCatanCaravansWorldWithMap(n, s.rooms[id].CatanNewWorldMap)
				if e != nil {
					t.Fatal(e)
				}
				a, _ := json.Marshal(expected.Catan.Tiles)
				b, _ := json.Marshal(s.rooms[id].Game.Catan.Tiles)
				if string(a) != string(b) {
					t.Fatal("approved board rerolled")
				}
			}
			ordered := make([]*testClient, n)
			for _, c := range clients {
				ordered[int(current(c)["you"].(float64))] = c
			}
			clients = ordered
			bidTimedOut := false
			initialRaw, marshalErr := json.Marshal(s.rooms[id].Game)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			var initial game.State
			if marshalErr = json.Unmarshal(initialRaw, &initial); marshalErr != nil {
				t.Fatal(marshalErr)
			}
			trace := []catanTraceAction{}
			peak, lastGain := 0, 0
			diagnosticRound := -1
			diagnosticPath := ""
			for step := 0; step < 18000 && !s.rooms[id].Game.Finished; step++ {
				g := s.rooms[id].Game
				total := 0
				for _, player := range g.Catan.Players {
					total += player.Score
				}
				if total > peak {
					peak, lastGain = total, g.Round
				}
				if g.Round-lastGain >= 80 && g.Round != diagnosticRound {
					saveCatanLongGame(t, &initial, g, trace)
					diagnosticRound = g.Round
					t.Fatalf("no new score peak for 80 rounds: round=%d score=%d peak=%d", g.Round, total, peak)
				}
				if scenario == "caravans-shores" && n == 2 && step%500 == 0 {
					if step > 0 {
						nextPath := saveCatanLongGame(t, &initial, g, trace)
						if diagnosticPath != "" {
							if err := os.Remove(diagnosticPath); err != nil {
								t.Fatal(err)
							}
						}
						diagnosticPath = nextPath
					}
					t.Log("step", step, "round", g.Round, "phase", g.Phase, "scores", g.Catan.Players[0].Score, g.Catan.Players[1].Score)
				}
				if events && !bidTimedOut && g.Phase == "catan_caravan_bid" {
					actor := twoHTTPActor(g)
					deadline := s.rooms[id].TurnDeadline
					if left := deadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
						t.Fatal("bid clock", left)
					}
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					if s.rooms[id].TurnDeadline != deadline {
						t.Fatal("restart reset bid clock")
					}
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(deadline + 1))
					s.mu.Unlock()
					if !s.rooms[id].Seats[actor].AutoPlay || !s.rooms[id].Seats[actor].TimeoutAutoPlay {
						t.Fatal("bid timeout did not take over")
					}
					setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					bidTimedOut = true
					continue
				}
				p := twoHTTPActor(g)
				a, err := g.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				trace = append(trace, catanTraceAction{Player: p, Action: a})
				clients[p].command(current(clients[p]), "action", a, 200)
				if step == 83 {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
				}
			}
			if diagnosticPath != "" && s.rooms[id].Game.Finished {
				if err := os.Remove(diagnosticPath); err != nil {
					t.Fatal(err)
				}
			}
			if events && !bidTimedOut {
				t.Fatal("no bid timeout exercised")
			}
			if !s.rooms[id].Game.Finished {
				saveCatanLongGame(t, &initial, s.rooms[id].Game, trace)
				t.Fatal("unfinished")
			}
			_, profile := h.request("GET", "/api/players/"+s.rooms[id].Host, nil)
			record := profile["history"].([]any)[0].(map[string]any)
			if helpers {
				rules := record["catanExpansionRules"].(map[string]any)
				if rules["caravans_helpers"] != game.CatanCaravansHelpersRules {
					t.Fatal("missing helper history")
				}
			}
			if record["catanScenario"] != scenario || record["catanRules"] != game.CatanCaravansSeafarersRules {
				t.Fatal("history")
			}
			if events {
				rules := record["catanExpansionRules"].(map[string]any)
				if rules["event_cards"] != game.CatanEventCatalogue {
					t.Fatal("missing events history")
				}
			}
			h.command(current(h), "rematch", nil, 200)
			cmd, field := "catan_scenario", "catanScenario"
			if n == 2 {
				cmd, field = "catan_two_scenario", "catanTwoScenario"
			}
			target := ""
			if n > 4 {
				target = "transport"
			}
			h.post("/api/rooms/"+id, map[string]any{"type": cmd, field: target, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			if s.rooms[id].CatanScenario != target {
				t.Fatal("base switch")
			}
		})
	}
}

func TestCatanCaravansSeaRejectsUnsupported(t *testing.T) {
	for _, n := range []int{1, 7} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
		if r.setCatanScenario("caravans-desert") == nil {
			t.Fatal("unsupported count", n)
		}
	}
	for _, mutate := range []func(*Room){func(r *Room) { r.CatanFishing = true }, func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} }, func(r *Room) { r.CatanOptions.AllHelpers = true }} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: 3, CatanScenario: "caravans-desert"}
		mutate(r)
		if r.validateCatanScenario() == nil {
			t.Fatal("unsupported mix")
		}
	}
}

func TestCatanCaravansIslandsPublicBounds(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
		err := r.setCatanScenario("caravans-islands")
		if (err == nil) != (n >= 2 && n <= 6) {
			t.Fatalf("count %d: %v", n, err)
		}
		if err != nil && r.CatanScenario != "" {
			t.Fatal("rejected selection changed room")
		}
	}
	for _, mutate := range []func(*Room){func(r *Room) { r.CatanFishing = true }, func(r *Room) { r.CatanOptions.AllHelpers = true }, func(r *Room) { r.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{} }} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: 3, CatanScenario: "caravans-islands"}
		mutate(r)
		if r.validateCatanScenario() == nil {
			t.Fatal("unsupported nesting accepted")
		}
	}
}

func TestCatanCaravanVariantScenarioSwitch(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		r := &Room{Kind: "catan", Status: "waiting", Capacity: n}
		if e := r.setCatanScenario("caravans-islands"); e != nil {
			t.Fatal(e)
		}
		r.CatanOptions, _ = game.NormalizeCatanOptions(game.CatanOptions{Helpers: true, AllHelpers: true})
		if e := r.setCatanFriendlyRobber(game.CatanFriendlyRobberSetup{Enabled: true}); e != nil {
			t.Fatal(e)
		}
		if e := r.setCatanHarbors(game.CatanHarborsSetup{Enabled: true}); e != nil {
			t.Fatal(e)
		}
		if e := r.setCatanScenario("caravans-new-world"); e != nil {
			t.Fatal(e)
		}
		if !r.CatanOptions.Helpers || !r.friendlyRobberEnabled() || r.CatanHarbors == nil || r.CatanNewWorldMap == nil {
			t.Fatal("sea switch lost configuration")
		}
		if e := r.setCatanScenario("transport"); e != nil {
			t.Fatal(e)
		}
		if r.CatanOptions.Helpers || r.CatanHarbors != nil || r.CatanFriendlyRobber != nil || r.CatanNewWorldMap != nil {
			t.Fatal("unsupported options leaked into transport")
		}
	}
}
