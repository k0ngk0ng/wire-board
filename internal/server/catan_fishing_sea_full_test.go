package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// All supported sizes start through public creation and use real HTTP,
// autoplay, timeout and restart paths through history recording.
func TestCatanFishingNewWorldExtendedFullHTTPGames(t *testing.T) {
	testFishingSeaExtendedFullHTTP(t, "new_world", 5, 6)
}

func TestCatanFishingFogExtendedFullHTTPGames(t *testing.T) {
	testFishingSeaExtendedFullHTTP(t, "fog", 5, 6)
}

func TestCatanFishingWondersExtendedFullHTTPGames(t *testing.T) {
	testFishingSeaExtendedFullHTTP(t, "wonders", 5, 6)
}

func TestCatanFishingPublicFullHTTPGames(t *testing.T) {
	testFishingSeaExtendedFullHTTP(t, "", 3, 4, 5, 6)
}

func TestCatanFishingSeaPublicFullHTTPGames(t *testing.T) {
	for _, scenario := range []string{"islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		t.Run(scenario, func(t *testing.T) { testFishingSeaExtendedFullHTTP(t, scenario, 3, 4) })
	}
}

func testFishingSeaExtendedFullHTTP(t *testing.T, scenario string, publicSizes ...int) {
	totalPaid := 0
	sizes := []int{5, 6}
	if scenario == "" {
		sizes = []int{3, 4}
	}
	if len(publicSizes) > 0 {
		sizes = publicSizes
	}
	for _, n := range sizes {
		for sample := 0; sample < 2; sample++ {
			t.Run(fmt.Sprintf("%d/sample%d", n, sample), func(t *testing.T) {
				var s *Server
				var ts *httptest.Server
				var clients []*testClient
				var id string
				if scenario != "" && len(publicSizes) > 0 {
					layout := "fixed"
					if n <= 4 && sample == 1 && scenario != "desert" && scenario != "tribe" {
						layout = "variable"
					}
					if scenario == "new_world" {
						layout = "prepared"
					}
					s, ts, clients, id = newPublicFishingSeaTable(t, n, scenario, layout)
				} else if scenario == "" {
					s, ts, clients, id = newPublicFishingTable(t, n)
				}
				supply, devSupply, tokenSupply, ports, grounds := 19, 25, 30, 9, 6
				if n > 4 {
					supply, devSupply, tokenSupply, ports, grounds = 24, 34, 44, 11, 8
				}
				if len(publicSizes) > 0 {
					ports = len(s.rooms[id].Game.Catan.Ports)
					if scenario == "new_world" {
						ports = 10
						if n > 4 {
							ports = 11
						}
					}
				}
				restored := map[string]bool{}
				restart := func(label string) {
					t.Helper()
					before, _ := json.Marshal(s.rooms[id])
					ts.Close()
					s.Close()
					next, err := New(s.cfg, s.files)
					if err != nil {
						t.Fatal(err)
					}
					stopBotTicker(next)
					t.Cleanup(func() { next.Close() })
					ts = httptest.NewServer(next.Handler())
					t.Cleanup(ts.Close)
					s = next
					for _, c := range clients {
						c.base = ts.URL
					}
					after, _ := json.Marshal(s.rooms[id])
					if string(before) != string(after) {
						t.Fatal("restart changed full game", label)
					}
					restored[label] = true
				}
				restart("setup")
				steps, automatic, timeouts, paid := 0, 0, 0, 0
				for ; steps < 10000 && !s.rooms[id].Game.Finished; steps++ {
					r := s.rooms[id]
					state, g := r.Game, r.Game.Catan
					label := ""
					switch {
					case state.Phase == "catan_world_ports" && g.Seafarers.NewWorld.Index == 2:
						label = "ports"
					case state.Phase == "catan_world_fish":
						label = "grounds"
					case state.Phase == "catan_fish_replace":
						label = "fish-response"
					case state.Phase == "catan_gold":
						label = "gold-response"
					case state.Phase == "catan_turn" && g.Paired != nil && g.Paired.Second:
						label = "secondary"
					}
					if label == "" && g.Seafarers != nil && g.Seafarers.Fog != nil && len(g.Seafarers.Fog.Terrain) < 18 {
						label = "discovery"
					}
					if label != "" && !restored[label] {
						restart(label)
						r = s.rooms[id]
						state, g = r.Game, r.Game.Catan
					}
					for color, total := range g.Bank {
						for _, p := range g.Players {
							total += p.Resources[color]
						}
						if total != supply {
							t.Fatal("resource supply", steps, color, total)
						}
					}
					if scenario == "cloth" || scenario == "tribe" {
						assertPublicFishSeaSpecialInventory(t, g)
					}
					cards := len(g.DevDeck) + len(g.DevDiscard)
					if scenario == "tribe" {
						cards += len(g.Seafarers.Tribe.Development)
					}
					for _, p := range g.Players {
						for _, count := range p.Dev {
							cards += count
						}
					}
					if cards != devSupply {
						t.Fatal("development supply", steps, cards)
					}
					seen := make([]bool, tokenSupply)
					f := g.Fishing
					if f.Tokens.BootOwner >= 0 {
						seen[29] = true
					}
					checkTokens := func(ids []int) {
						t.Helper()
						for _, id := range ids {
							if id < 0 || id >= tokenSupply || seen[id] {
								t.Fatal("fish identity", steps, id)
							}
							seen[id] = true
						}
					}
					checkTokens(f.Tokens.DrawPile)
					checkTokens(f.Tokens.Discard)
					for _, hand := range f.Tokens.Hands {
						if len(hand) > 7 {
							t.Fatal("fish cap")
						}
						checkTokens(hand)
					}
					for _, present := range seen {
						if !present {
							t.Fatal("lost fish token", steps)
						}
					}
					if steps%31 == 0 {
						for _, viewer := range []int{state.Turn, n} {
							v := current(clients[viewer])["game"].(map[string]any)["catan"].(map[string]any)
							fish := v["fishing"].(map[string]any)
							tokens := fish["tokens"].(map[string]any)
							if tokens["drawPile"] != nil || fish["pending"] != nil {
								t.Fatal("hidden fishing state exposed")
							}
							for p, raw := range tokens["players"].([]any) {
								_, visible := raw.(map[string]any)["tokens"]
								if visible != (p == viewer && len(f.Tokens.Hands[p]) > 0) {
									t.Fatal("fish privacy")
								}
							}
							sea, _ := v["seafarers"].(map[string]any)
							if world, ok := sea["newWorld"].(map[string]any); ok && world["ports"] != nil {
								t.Fatal("hidden port order")
							}
							if setup, ok := fish["worldSetup"].(map[string]any); ok && setup["numbers"] != nil {
								t.Fatal("hidden ground order")
							}
							if fog, ok := sea["fog"].(map[string]any); ok && (fog["terrain"] != nil || fog["numbers"] != nil) {
								t.Fatal("hidden exploration order")
							}
							for p, raw := range v["players"].([]any) {
								seat := raw.(map[string]any)
								_, hand := seat["resources"]
								_, dev := seat["dev"]
								if hand != (p == viewer) || dev != (p == viewer) {
									t.Fatal("hand privacy")
								}
							}
						}
					}
					actor := state.Turn
					if pending := state.CatanPendingActor(); pending >= 0 {
						actor = pending
					} else if state.Phase == "catan_discard" {
						for p, due := range g.DiscardDue {
							if due > 0 {
								actor = p
								break
							}
						}
					}
					version := r.Version
					if steps%29 == 0 && (g.SetupStep < g.SetupLimit() || state.CatanPendingActor() >= 0 || state.Phase == "catan_discard") {
						s.mu.Lock()
						r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
						s.expireSetups(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("timeout stalled", steps, state.Phase)
						}
						timeouts++
						reclaimTimeoutHumans(t, s, clients, id)
						continue
					}
					if steps%17 == 0 {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						version = s.rooms[id].Version
						s.mu.Lock()
						s.rooms[id].BotAt = 0
						s.runBots(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("autoplay stalled", steps, state.Phase)
						}
						if !s.rooms[id].Game.Finished {
							setAutoPlay(clients[actor], current(clients[actor]), false, 200)
						}
						automatic++
						continue
					}
					action, err := state.BotAction(actor)
					if err != nil {
						t.Fatal(steps, state.Phase, err)
					}
					if action.Type == "catan_fish_road" || action.Type == "catan_fish_ship" || action.Type == "catan_fish_dev" || action.Type == "catan_fish_resource" || action.Type == "catan_fish_robber" || action.Type == "catan_fish_pirate" || action.Type == "catan_fish_steal" {
						paid++
					}
					clients[actor].command(current(clients[actor]), "action", action, 200)
				}
				r := s.rooms[id]
				if !r.Game.Finished || r.Status != "finished" || (len(r.Game.Winners) == 0 || scenario != "cloth" && len(r.Game.Winners) != 1) || !restored["setup"] || scenario == "new_world" && (!restored["ports"] || !restored["grounds"]) || n > 4 && !restored["secondary"] || automatic == 0 || timeouts == 0 {
					t.Fatal("incomplete full-game coverage", steps, automatic, timeouts, paid, restored)
				}
				totalPaid += paid
				winner := r.Game.Winners[0]
				target := 12
				if scenario == "wonders" || scenario == "" {
					target = 10
				}
				if scenario == "islands" || scenario == "tribe" {
					target = 13
				}
				if scenario == "desert" || scenario == "cloth" {
					target = 14
				}
				if r.Game.Catan.Fishing.Tokens.BootOwner == winner {
					target++
				}
				won := r.Game.Catan.Players[winner].Score >= target
				if scenario == "wonders" {
					level, other := 0, 0
					for _, card := range r.Game.Catan.Seafarers.Wonders.Cards {
						if card.Owner == winner {
							level = card.Level
						} else if card.Owner >= 0 && !r.Game.Catan.Players[card.Owner].Eliminated {
							other = max(other, card.Level)
						}
					}
					won = level == 4 || level > 0 && level > other && won
					wantCards := 5
					if n > 4 {
						wantCards = 7
					}
					if len(r.Game.Catan.Seafarers.Wonders.Cards) != wantCards || r.Game.Catan.Seafarers.Pirate != -1 {
						t.Fatal("extended wonder rules changed")
					}
				}
				if scenario == "cloth" {
					won = publicFishingClothWon(t, r.Game)
				}
				if scenario == "tribe" {
					assertPublicFishSeaSpecialInventory(t, r.Game.Catan)
				}
				if !won || (scenario != "tribe" && len(r.Game.Catan.Ports) != ports) || len(r.Game.Catan.Fishing.Map.Grounds) != grounds {
					t.Fatal("wrong finished layout/victory", "won", won, "ports", len(r.Game.Catan.Ports), ports, "grounds", len(r.Game.Catan.Fishing.Map.Grounds), grounds)
				}
				restart("finished")
				code, profile := clients[n].request("GET", "/api/players/"+s.rooms[id].Seats[winner].ID, nil)
				if code != 200 || profile["stats"].(map[string]any)["catan"].(map[string]any)["wins"] != float64(1) {
					t.Fatal("win missing from history")
				}
				if scenario == "" || len(publicSizes) > 0 {
					check := func(profile map[string]any) {
						t.Helper()
						history := profile["history"].([]any)
						if len(history) != 1 {
							t.Fatal("duplicate fishing history")
						}
						record := history[0].(map[string]any)
						wantScenario, wantLayout := "fishing", "variable"
						if scenario != "" {
							wantScenario, wantLayout = scenario, r.Game.Catan.Seafarers.Layout
						}
						if record["catanScenario"] != wantScenario || record["catanLayout"] != wantLayout || record["catanExpansionRules"].(map[string]any)["fishing"] != game.CatanFishingRules {
							t.Fatal("wrong fishing archive", record)
						}
						if scenario == "" && n > 4 && record["catanExpansionRules"].(map[string]any)["number_recipe"] != game.CatanExtendedNumberRecipe {
							t.Fatal("missing fishing number recipe")
						}
						stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
						if stats["wins"] != float64(1) || stats["played"] != float64(1) {
							t.Fatal("duplicate fishing result")
						}
					}
					check(profile)
					if err := s.save(s.rooms[id]); err != nil {
						t.Fatal(err)
					}
					restart("replayed-finish")
					code, profile = clients[n].request("GET", "/api/players/"+s.rooms[id].Seats[winner].ID, nil)
					if code != 200 {
						t.Fatal("history unavailable")
					}
					check(profile)
				}
				t.Logf("steps=%d autoplay=%d timeouts=%d paid=%d winner=%d score=%d restarts=%v", steps, automatic, timeouts, paid, winner, s.rooms[id].Game.Catan.Players[winner].Score, restored)
			})
		}
	}
	// A legal game can finish without a manual fish payment. Require this
	// coverage across the batch; directed action tests verify each paid action.
	if totalPaid == 0 {
		t.Fatal("full-game batch never spent fish manually")
	}
}
