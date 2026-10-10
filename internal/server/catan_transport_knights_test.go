package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestCatanTransportKnightsNaturalHTTP(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, ts, clients, id := newTradersKnightsHTTP(t, n, events, "transport")
				restored := false
				for step := 0; step < 16000 && !s.rooms[id].Game.Finished; step++ {
					r := s.rooms[id]
					p := twoHTTPActor(r.Game)
					a, e := r.Game.BotAction(p)
					if e != nil {
						t.Fatal(step, r.Game.Phase, e)
					}
					clients[p].command(current(clients[p]), "action", a, 200)
					if step == 97 {
						s, ts = restartRiversHTTP(t, s, ts, clients, id)
						restored = true
					}
					if step%71 == 0 {
						g := s.rooms[id].Game.Catan
						total := g.Transport.GoldBank
						for _, gold := range g.Transport.Gold {
							total += gold
						}
						want := g.Transport.Map.Gold
						if total != want+g.Transport.GoldIssued {
							t.Fatal("gold inventory")
						}
						for color, total := range g.Bank {
							for _, seat := range g.Players {
								total += seat.Resources[color]
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
								t.Fatal("card inventory", color, total)
							}
						}
						v := current(clients[n])["game"].(map[string]any)["catan"].(map[string]any)
						if v["victoryTarget"] != float64(15) {
							t.Fatal("target")
						}
						// Hands and progress are revealed by design once the
						// game is finished, and a game can end on this step.
						finished := s.rooms[id].Game.Finished
						for _, raw := range v["players"].([]any) {
							if !finished && raw.(map[string]any)["resources"] != nil {
								t.Fatal("hidden hand leaked")
							}
						}
						for _, raw := range v["citiesKnights"].(map[string]any)["players"].([]any) {
							if !finished && raw.(map[string]any)["progress"] != nil {
								t.Fatal("progress leaked")
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
				if record["catanExpansionRules"].(map[string]any)["transportKnights"] != game.CatanTransportKnightsRules {
					t.Fatal("history marker")
				}
				t.Log("rounds", r.Game.Round)
			})
		}
	}
}
func TestCatanTransportKnightsClockAndBaseIsolation(t *testing.T) {
	for _, n := range []int{2, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts, clients, id := newTradersKnightsHTTPAtPillage(t, n, "transport")
			r := s.rooms[id]
			actor := twoHTTPActor(r.Game)
			version := r.Version
			clients[n].command(current(clients[n]), "action", game.Action{Type: "catan_pillage", Vertex: 0}, 400)
			if s.rooms[id].Version != version {
				t.Fatal("spectator response")
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			r = s.rooms[id]
			s.mu.Lock()
			r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
			s.expireSetups(time.Now())
			s.mu.Unlock()
			if s.rooms[id].Version <= version || !s.rooms[id].Seats[actor].AutoPlay {
				t.Fatal("timeout did not autoplay")
			}
			reclaimTimeoutHumans(t, s, clients, id)
			host := clients[0]
			for p, seat := range s.rooms[id].Seats {
				if seat.ID == s.rooms[id].Host {
					host = clients[p]
				}
			}
			host.command(current(host), "close", nil, 200)
			host.command(current(host), "rematch", nil, 200)
			id = current(host)["id"].(string)
			host.post("/api/rooms/"+id, map[string]any{"type": "catan_cities_knights", "catanCitiesKnights": nil, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			for _, c := range clients[:n] {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if g.CitiesKnights != nil || g.Transport.Knights != "" || len(g.Bank) != 5 || len(g.DevDeck) == 0 {
				t.Fatal("base transport isolation")
			}
		})
	}
}
