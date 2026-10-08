package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"slices"
	"testing"
	"time"
)

func TestCatanFishingSeaKnightsNaturalHTTP(t *testing.T) {
	for i, scene := range []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) {
			testCatanCitiesKnightsFishingFullHTTPGames(t, scene, true, true, i%2 == 0, true, 3+i%4)
		})
	}
}
func TestCatanFishingSeaKnightsPublicSelection(t *testing.T) {
	for _, scene := range []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h := newClient(t, ts.URL)
			h.register("海图骑士渔夫")
			r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "海图骑士渔夫", "capacity": 3, "catanScenario": scene, "catanFishing": true}, 201)
			id := r["id"].(string)
			for _, on := range []bool{true, false, true} {
				var k *game.CatanCitiesKnightsSetup
				if on {
					k = &game.CatanCitiesKnightsSetup{}
				}
				selectCatanCitiesKnights(h, k, 200)
				if (s.rooms[id].CatanCitiesKnights != nil) != on || !s.rooms[id].CatanFishing {
					t.Fatal("toggle lost combination")
				}
			}
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{Helpers: true}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
			for i := 0; i < 2; i++ {
				h.command(current(h), "add_bot", nil, 200)
			}
			h.command(current(h), "ready", nil, 200)
			h.command(current(h), "start", nil, 200)
			if s.rooms[id].Game.Catan.Fishing.SeaKnights != game.CatanFishingSeaKnightsRules {
				t.Fatal("wrong opening")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if current(h)["catanFishing"] != true || current(h)["catanCitiesKnights"] == nil {
				t.Fatal("lost rematch")
			}
		})
	}
}

func TestCatanFishingSeaKnightsResponseClock(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, timeout := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/timeout%t", n, timeout), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for p := range clients {
					clients[p] = newClient(t, ts.URL)
					clients[p].register(fmt.Sprintf("海图回应%d", p))
				}
				r := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "换鱼金矿引水渠", "capacity": n, "catanScenario": "shores", "catanFishing": true, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "catanOptions": game.CatanOptions{FiveSix: n > 4}, "catanEvents": game.CatanEventCatalogue}, 201)
				id := r["id"].(string)
				for p := 1; p < n; p++ {
					clients[p].command(current(clients[0]), "join", nil, 200)
				}
				for p := 0; p < n; p++ {
					clients[p].command(current(clients[0]), "ready", nil, 200)
				}
				clients[0].command(current(clients[0]), "start", nil, 200)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				for s.rooms[id].Game.Catan.SetupStep < s.rooms[id].Game.Catan.SetupLimit() {
					state := s.rooms[id].Game
					a, err := state.BotAction(state.Turn)
					if err != nil {
						t.Fatal(err)
					}
					clients[state.Turn].command(current(clients[state.Turn]), "action", a, 200)
				}
				room := s.rooms[id]
				g := room.Game.Catan
				owner := room.Game.Turn
				p := (owner + 1) % n
				for i := range g.Vertices {
					g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
				}
				gold := -1
				for i := range g.Tiles {
					if g.Tiles[i].Resource < 5 {
						g.Tiles[i].Number = 6
					}
					if g.Tiles[i].Resource == game.CatanGold {
						gold = i
						g.Tiles[i].Number = 2
					}
				}
				if gold < 0 {
					t.Fatal("missing gold")
				}
				v := g.Tiles[g.Fishing.Map.Lakes[0].Tile].Vertices[0]
				g.Vertices[v].Owner, g.Vertices[v].Level = p, 2
				v = g.Tiles[gold].Vertices[0]
				g.Vertices[v].Owner, g.Vertices[v].Level = owner, 2
				g.CitiesKnights.Players[p].Improvements[game.CatanScience] = 3
				g.CitiesKnights.Players[owner].Improvements[game.CatanScience] = 3
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
				room.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err := s.save(room); err != nil {
					t.Fatal(err)
				}
				clients[owner].command(current(clients[owner]), "action", game.Action{Type: "catan_roll"}, 200)
				sequence := []string{}
				for step := 0; s.rooms[id].Game.Phase != "catan_turn" && step < 50; step++ {
					s, ts = restartRiversHTTP(t, s, ts, clients, id)
					room = s.rooms[id]
					phase := room.Game.Phase
					if phase == "catan_fish_replace" || phase == "catan_gold" || phase == "catan_aqueduct" {
						if len(sequence) == 0 || sequence[len(sequence)-1] != phase {
							sequence = append(sequence, phase)
						}
						if room.CatanTimeLeft < 40000 || room.CatanTimeLeft > 45000 {
							t.Fatal("lost turn budget", room.CatanTimeLeft)
						}
					}
					actor := twoHTTPActor(room.Game)
					a, err := room.Game.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					clients[n].command(current(clients[n]), "action", a, 400)
					if timeout {
						s.mu.Lock()
						s.expireSetups(time.UnixMilli(room.TurnDeadline))
						s.mu.Unlock()
						if !s.rooms[id].Seats[actor].TimeoutAutoPlay {
							t.Fatal("no persistent takeover")
						}
						reclaimTimeoutHumans(t, s, clients, id)
					} else {
						clients[actor].command(current(clients[actor]), "action", a, 200)
					}
				}
				room = s.rooms[id]
				if !slices.Equal(sequence, []string{"catan_fish_replace", "catan_gold", "catan_aqueduct"}) || room.Game.Phase != "catan_turn" || room.Game.Turn != owner {
					t.Fatal("response ordering", sequence, room.Game.Phase)
				}
				if !timeout {
					left := room.TurnDeadline - time.Now().UnixMilli()
					if left < 38000 || left > 45000 {
						t.Fatal("turn time reset", left)
					}
				}
			})
		}
	}
}
