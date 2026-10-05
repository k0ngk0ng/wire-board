package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Fishing has no public room option yet. Install its initial state in a real
// table; every layout, build, roll, response and action then uses production
// HTTP handlers, autoplay or timeout handling, through final history recording.
func TestCatanFishingNewWorldExtendedFullHTTPGames(t *testing.T) {
	for _, n := range []int{5, 6} {
		for sample := 0; sample < 2; sample++ {
			t.Run(fmt.Sprintf("%d/sample%d", n, sample), func(t *testing.T) {
				s, ts, clients, id, _, _ := newFishingActionTable(t, n, "catan_turn", "resource")
				layout, err := game.GenerateCatanNewWorldMap(n)
				if err != nil {
					t.Fatal(err)
				}
				initial, err := game.NewCatanFishingNewWorld(n, game.CatanOptions{FiveSix: true}, layout)
				if err != nil {
					t.Fatal(err)
				}
				s.mu.Lock()
				r := s.rooms[id]
				r.Game = initial
				r.startTurnClock(time.Now())
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
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
				steps, automatic, timeouts, paid := 0, 0, 0, 0
				for ; steps < 10000 && !s.rooms[id].Game.Finished; steps++ {
					r = s.rooms[id]
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
					case state.Phase == "catan_turn" && g.Paired.Second:
						label = "secondary"
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
						if total != 24 {
							t.Fatal("resource supply", steps, color, total)
						}
					}
					cards := len(g.DevDeck) + len(g.DevDiscard)
					for _, p := range g.Players {
						for _, count := range p.Dev {
							cards += count
						}
					}
					if cards != 34 {
						t.Fatal("development supply", steps, cards)
					}
					seen := make([]bool, 44)
					f := g.Fishing
					if f.Tokens.BootOwner >= 0 {
						seen[29] = true
					}
					checkTokens := func(ids []int) {
						t.Helper()
						for _, id := range ids {
							if id < 0 || id >= 44 || seen[id] {
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
							if tokens["drawPile"] != nil || fish["pending"] != nil || fish["worldSetup"].(map[string]any)["numbers"] != nil {
								t.Fatal("hidden fishing state exposed")
							}
							for p, raw := range tokens["players"].([]any) {
								_, visible := raw.(map[string]any)["tokens"]
								if visible != (p == viewer && len(f.Tokens.Hands[p]) > 0) {
									t.Fatal("fish privacy")
								}
							}
							world := v["seafarers"].(map[string]any)["newWorld"].(map[string]any)
							if world["ports"] != nil {
								t.Fatal("hidden port order")
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
				r = s.rooms[id]
				if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || !restored["ports"] || !restored["grounds"] || !restored["secondary"] || automatic == 0 || timeouts == 0 || paid == 0 {
					t.Fatal("incomplete full-game coverage", steps, automatic, timeouts, paid, restored)
				}
				winner := r.Game.Winners[0]
				target := 12
				if r.Game.Catan.Fishing.Tokens.BootOwner == winner {
					target++
				}
				if r.Game.Catan.Players[winner].Score < target || len(r.Game.Catan.Ports) != 11 || len(r.Game.Catan.Fishing.Map.Grounds) != 8 {
					t.Fatal("wrong finished layout/victory")
				}
				restart("finished")
				code, profile := clients[n].request("GET", "/api/players/"+s.rooms[id].Seats[winner].ID, nil)
				if code != 200 || profile["stats"].(map[string]any)["catan"].(map[string]any)["wins"] != float64(1) {
					t.Fatal("win missing from history")
				}
				t.Logf("steps=%d autoplay=%d timeouts=%d paid=%d winner=%d score=%d restarts=%v", steps, automatic, timeouts, paid, winner, s.rooms[id].Game.Catan.Players[winner].Score, restored)
			})
		}
	}
}
