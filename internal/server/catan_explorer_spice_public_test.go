package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanExplorerSpicePublicCompleteHTTPGames(t *testing.T) {
	testPublicExplorerMissionGames(t, "spices-for-catan")
}

func TestCatanExplorerMissionsPublicCompleteHTTPGames(t *testing.T) {
	for _, scene := range []string{"pirate-lairs", "fish-for-catan", "explorers-and-pirates"} {
		t.Run(scene, func(t *testing.T) { testPublicExplorerMissionGames(t, scene) })
	}
}

func testPublicExplorerMissionGames(t *testing.T, scenario string) {
	t.Helper()
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newPublicExplorerScenarioHTTP(t, n, scenario)
			initial := s.rooms[id].Game.Catan
			baseTiles := map[string]int{"pirate-lairs": 58, "fish-for-catan": 58, "spices-for-catan": 65, "explorers-and-pirates": 72}
			extendedTiles := map[string]int{"pirate-lairs": 79, "fish-for-catan": 88, "spices-for-catan": 88, "explorers-and-pirates": 97}
			targets := map[string]int{"pirate-lairs": 12, "fish-for-catan": 15, "spices-for-catan": 15, "explorers-and-pirates": 17}
			wantTiles := baseTiles[scenario]
			if n > 4 {
				wantTiles = extendedTiles[scenario]
			}
			wantLairs := scenario != "spices-for-catan"
			wantFish := scenario != "pirate-lairs"
			wantSpice := scenario == "spices-for-catan" || scenario == "explorers-and-pirates"
			if len(initial.Tiles) != wantTiles || (initial.Explorer.Lairs != nil) != wantLairs || (initial.Explorer.Spice != nil) != wantSpice || (initial.Explorer.Fish != nil) != wantFish || initial.Explorer.Board.Target != targets[scenario] || (initial.Paired != nil) != (n > 4) {
				t.Fatal("wrong mission opening")
			}
			privacy := func(state *game.State) {
				if wantSpice {
					assertExplorerSpiceHTTPPrivacy(t, clients, state)
				} else if wantFish {
					assertExplorerFishHTTPPrivacy(t, clients, state)
				} else {
					assertExplorerHTTPPrivacy(t, clients)
				}
			}
			stock := append([]int(nil), initial.Bank...)
			goldStock := initial.Explorer.Economy.GoldBank
			for _, v := range initial.Explorer.Economy.Gold {
				goldStock += v
			}
			timedout, automatic, second := false, false, false
			setupSteps := 0
			for step := 0; step < 10000 && !s.rooms[id].Game.Finished; step++ {
				r := s.rooms[id]
				g := r.Game
				actor := explorerHTTPActor(g)
				if g.Phase == "catan_explorer_setup" {
					setupSteps++
				}
				second = second || (g.Catan.Paired != nil && g.Catan.Paired.Second)
				before := r.Version
				if !timedout && g.Phase == "catan_turn" {
					s.mu.Lock()
					s.expireSetups(time.UnixMilli(r.TurnDeadline))
					s.mu.Unlock()
					if !s.rooms[id].Seats[actor].AutoPlay {
						t.Fatal("timeout did not persist takeover")
					}
					reclaimTimeoutHumans(t, s, clients, id)
					timedout = true
				} else if !automatic && g.Phase == "catan_explorer_move" {
					setAutoPlay(clients[actor], current(clients[actor]), true, 200)
					s.mu.Lock()
					s.rooms[id].BotAt = 0
					s.runBots(time.Now())
					s.mu.Unlock()
					setAutoPlay(clients[actor], current(clients[actor]), false, 200)
					automatic = true
				} else {
					intent, err := g.BotAction(actor)
					if err != nil {
						t.Fatal(step, g.Phase, err)
					}
					a := fishHTTPPreview(t, current(clients[actor]), intent)
					clients[actor].command(current(clients[actor]), "action", a, 200)
				}
				r = s.rooms[id]
				if r.Version <= before {
					t.Fatal("public game stalled", step)
				}
				cg := r.Game.Catan
				for resource, bank := range cg.Bank {
					total := bank
					if bank < 0 {
						t.Fatal("negative bank")
					}
					for _, p := range cg.Players {
						if p.Resources[resource] < 0 {
							t.Fatal("negative hand")
						}
						total += p.Resources[resource]
					}
					if total != stock[resource] {
						t.Fatal("resource inventory changed")
					}
				}
				e := cg.Explorer.Economy
				total := e.GoldBank
				for _, v := range e.Gold {
					if v < 0 {
						t.Fatal("negative gold")
					}
					total += v
				}
				if total != goldStock+e.GoldIssued {
					t.Fatal("gold ledger changed")
				}
				if step%103 == 0 {
					privacy(r.Game)
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
				}
			}
			r := s.rooms[id]
			expectedSetup := 4 * n
			if n == 2 {
				expectedSetup += 4
			}
			if r.Status != "finished" || len(r.Game.Winners) != 1 || r.Game.Catan.Players[r.Game.Winners[0]].Score < targets[scenario] || !automatic || !timedout || (n > 4 && !second) || setupSteps != expectedSetup {
				t.Fatal("incomplete public spice game", r.Status, setupSteps, scenario)
			}
			if scenario == "spices-for-catan" && (len(r.Game.Catan.Explorer.Spice.Deliveries) == 0 || len(r.Game.Catan.Explorer.Fish.Deliveries) == 0) {
				t.Fatal("spice game lacked delivery coverage")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			privacy(s.rooms[id].Game)
			assertPublicExplorerHistory(t, s, clients, id)
		})
	}
}

func TestCatanExplorerSpicePublicConfiguration(t *testing.T) {
	testPublicExplorerMissionConfiguration(t, "spices-for-catan")
}
func TestCatanExplorerMissionsPublicConfiguration(t *testing.T) {
	for _, scene := range []string{"pirate-lairs", "fish-for-catan", "explorers-and-pirates"} {
		t.Run(scene, func(t *testing.T) { testPublicExplorerMissionConfiguration(t, scene) })
	}
}

func testPublicExplorerMissionConfiguration(t *testing.T, scenario string) {
	t.Helper()
	for _, n := range []int{2, 4, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
			host.register("香料房主")
			guest.register("香料朋友")
			raw := host.post("/api/rooms", map[string]any{"kind": "catan", "capacity": n, "name": "香料公开设置", "catanScenario": scenario}, 201)
			id := raw["id"].(string)
			guest.command(current(host), "join", nil, 200)
			change := func(c *testClient, kind, key, value string, status int) {
				c.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			for _, c := range []*testClient{host, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			change(host, "catan_scenario", "catanScenario", scenario, 200)
			if !s.rooms[id].Seats[0].Ready || !s.rooms[id].Seats[1].Ready {
				t.Fatal("unchanged selection cleared readiness")
			}
			before, _ := json.Marshal(s.rooms[id])
			change(guest, "catan_scenario", "catanScenario", "land-ho", 400)
			for _, o := range []game.CatanOptions{{Helpers: true}, {FiveSix: true}} {
				host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": o, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
			}
			if n > 4 {
				for _, scene := range []string{"land-ho", "shores", ""} {
					change(host, "catan_scenario", "catanScenario", scene, 400)
				}
			}
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid options changed room")
			}
			if n <= 4 {
				change(host, "catan_scenario", "catanScenario", "land-ho", 200)
				if n == 2 {
					change(host, "catan_two_scenario", "catanTwoScenario", "caravans", 200)
				} else {
					change(host, "catan_scenario", "catanScenario", "shores", 200)
				}
				change(host, "catan_scenario", "catanScenario", scenario, 200)
				if s.rooms[id].CatanTwoRules != "" || s.rooms[id].CatanSeafarers != nil || s.rooms[id].Seats[0].Ready {
					t.Fatal("old recipe or readiness survived")
				}
				for _, c := range []*testClient{host, guest} {
					c.command(current(c), "ready", nil, 200)
				}
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
			host.command(current(host), "start", nil, 200)
			if len(s.rooms[id].Game.Catan.Players) != 2 || s.rooms[id].Game.Catan.Paired != nil {
				t.Fatal("capacity mistaken for actual players")
			}
			change(host, "catan_scenario", "catanScenario", "land-ho", 400)
			host.command(current(host), "close", nil, 200)
			host.command(current(host), "rematch", nil, 200)
			if s.rooms[id].CatanScenario != scenario {
				t.Fatal("rematch lost scenario")
			}
		})
	}
}
