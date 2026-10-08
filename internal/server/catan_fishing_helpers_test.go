package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestCatanFishingHelpersNaturalHTTP(t *testing.T) {
	for i, scene := range []string{"", "shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) { testFishingConfiguredFullHTTP(t, scene, true, true, i%2 == 0, true, 3+i%4) })
	}
}
func TestCatanFishingHelpersPublicSelection(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h := newClient(t, ts.URL)
	h.register("渔夫助手配置")
	for _, n := range []int{3, 6} {
		for _, scene := range []string{"fishing", "shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
			t.Run(fmt.Sprintf("%s/%d", scene, n), func(t *testing.T) {
				r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "助手组合", "capacity": n, "catanScenario": scene, "catanFishing": scene != "fishing", "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: true}}, 201)
				id := r["id"].(string)
				if !s.rooms[id].CatanOptions.Helpers {
					t.Fatal("lost helper option")
				}
				for _, on := range []bool{false, true} {
					h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: on, AllHelpers: on}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
					if s.rooms[id].CatanOptions.Helpers != on {
						t.Fatal("failed helper toggle")
					}
				}
				for seats := 1; seats < n; seats++ {
					h.command(current(h), "add_bot", nil, 200)
				}
				h.command(current(h), "ready", nil, 200)
				h.command(current(h), "start", nil, 200)
				h.command(current(h), "close", nil, 200)
				h.command(current(h), "rematch", nil, 200)
				if !current(h)["catanOptions"].(map[string]any)["helpers"].(bool) {
					t.Fatal("rematch lost helpers")
				}
				h.command(current(h), "leave", nil, 200)
			})
		}
	}
}

func TestCatanFishingHelpersResponseClock(t *testing.T) {
	testCatanFishingHelpersResponseClock(t, false)
}
func TestCatanFishingHelpersKnightsResponseClock(t *testing.T) {
	testCatanFishingHelpersResponseClock(t, true)
}
func testCatanFishingHelpersResponseClock(t *testing.T, knights bool) {
	for _, n := range []int{3, 6} {
		for _, timeout := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/timeout%t", n, timeout), func(t *testing.T) {
				var s *Server
				var ts *httptest.Server
				var clients []*testClient
				var id string
				if knights {
					s, ts, clients, id = newHelpersKnightsHTTP(t, n, "fishing", true, true)
				} else {
					s, ts, clients, id = newPublicFishingScenarioConfigured(t, n, "fishing", false, false, true, true)
				}
				for s.rooms[id].Game.Catan.SetupStep < s.rooms[id].Game.Catan.SetupLimit() {
					state := s.rooms[id].Game
					a, err := state.BotAction(state.Turn)
					if err != nil {
						t.Fatal(err)
					}
					clients[state.Turn].command(current(clients[state.Turn]), "action", a, 200)
				}
				r := s.rooms[id]
				g := r.Game.Catan
				owner := r.Game.Turn
				p := (owner + 1) % n
				g.TurnSerial = 10
				if knights {
					g.CitiesKnights.Players[p].Improvements[0] = 3
				}
				for i := range g.Tiles {
					if g.Tiles[i].Resource < 5 {
						g.Tiles[i].Number = 6
					}
				}
				old := g.Players[p].Helper.ID
				if old != 3 {
					if at := slices.Index(g.HelperDisplay, 3); at >= 0 {
						g.HelperDisplay[at] = old
					} else {
						for seat := range g.Players {
							if g.Players[seat].Helper.ID == 3 {
								g.Players[seat].Helper = &game.CatanHelperSeat{ID: old}
								break
							}
						}
					}
				}
				g.Players[p].Helper = &game.CatanHelperSeat{ID: 3}
				for i := range g.Vertices {
					g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
				}
				v := g.Tiles[g.Fishing.Map.Lakes[0].Tile].Vertices[0]
				g.Vertices[v].Owner, g.Vertices[v].Level = p, 1
				f := &g.Fishing.Tokens
				for seat, hand := range f.Hands {
					f.DrawPile = append(f.DrawPile, hand...)
					f.Hands[seat] = nil
				}
				for _, token := range []int{0, 1, 2, 3, 4, 5, 6} {
					at := slices.Index(f.DrawPile, token)
					f.DrawPile = slices.Delete(f.DrawPile, at, at+1)
					f.Hands[p] = append(f.Hands[p], token)
				}
				at := slices.Index(f.DrawPile, 21)
				f.DrawPile[at], f.DrawPile[len(f.DrawPile)-1] = f.DrawPile[len(f.DrawPile)-1], f.DrawPile[at]
				pile := g.EventDeck.Deck.DrawPile
				at = slices.Index(pile, 0)
				pile[at], pile[len(pile)-1] = pile[len(pile)-1], pile[at]
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				clients[owner].command(current(clients[owner]), "action", game.Action{Type: "catan_roll"}, 200)
				seen := map[string]bool{}
				for step := 0; s.rooms[id].Game.Phase != "catan_turn" && step < 30; step++ {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					r = s.rooms[id]
					actor := twoHTTPActor(r.Game)
					phase := r.Game.Phase
					if phase == "catan_fish_replace" || phase == "catan_aqueduct" || phase == "catan_helper" {
						if actor != p || r.Game.Turn != owner {
							t.Fatal("response changed turn")
						}
						if r.CatanTimeLeft < 42000 || r.CatanTimeLeft > 45000 {
							t.Fatal("turn budget not paused", r.CatanTimeLeft)
						}
						label := phase
						if phase == "catan_helper" {
							label += "/" + r.Game.Catan.HelperPending.Kind
						}
						seen[label] = true
					}
					a, err := r.Game.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					clients[n].command(current(clients[n]), "action", a, 400)
					if timeout {
						s.mu.Lock()
						s.expireSetups(time.UnixMilli(r.TurnDeadline))
						s.mu.Unlock()
						if !s.rooms[id].Seats[actor].TimeoutAutoPlay {
							t.Fatal("timeout not persistent")
						}
						reclaimTimeoutHumans(t, s, clients, id)
					} else {
						clients[actor].command(current(clients[actor]), "action", a, 200)
					}
				}
				r = s.rooms[id]
				if !seen["catan_fish_replace"] || knights && !seen["catan_aqueduct"] || !seen["catan_helper/resource"] || !seen["catan_helper/exchange"] || r.Game.Phase != "catan_turn" || r.Game.Turn != owner {
					t.Fatal("incomplete response chain", seen)
				}
				if !timeout {
					left := r.TurnDeadline - time.Now().UnixMilli()
					if left < 40000 || left > 45000 {
						t.Fatal("turn budget reset", left)
					}
				}
			})
		}
	}
}
