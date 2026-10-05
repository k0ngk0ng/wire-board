package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanCardResourceAndFleeHTTPRestart(t *testing.T) {
	for _, kind := range []string{"plentiful_year", "robber_flees", "good_neighbors", "calm_seas", "tournament", "helpful_neighbor"} {
		reward := kind == "plentiful_year" || kind == "calm_seas" || kind == "tournament"
		for _, mode := range []string{"manual", "autoplay", "timeout"} {
			t.Run(fmt.Sprintf("%s/%s", kind, mode), func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				s.mu.Lock()
				r := s.rooms[id]
				cardEventResponseFixture(t, r, "base", time.Now())
				g := r.Game.Catan
				g.CardEvent.Kind = kind
				g.CardEvent.Production, g.Tiles[0].Number = 2, 2
				if kind == "calm_seas" {
					// All players tie at zero installed port buildings.
					g.Ports = nil
				}
				if kind == "helpful_neighbor" {
					g.CardEvent.Players, g.CardEvent.Targets = []int{0, 1}, []int{2}
					for p := range 2 {
						// Make two public leaders by upgrading their other building;
						// all three selected production buildings remain settlements.
						for i, v := range g.Vertices {
							if v.Owner == p && v.ID != g.Tiles[0].Vertices[p] {
								g.Vertices[i].Level = 2
								g.Players[p].Score++
								break
							}
						}
						g.Players[p].Resources[p+1]++
						g.Bank[p+1]--
					}
				}
				if kind == "good_neighbors" {
					for p := range g.Players {
						g.Players[p].Resources[p+1]++
						g.Bank[p+1]--
						g.CardEvent.Gifts = append(g.CardEvent.Gifts, game.CatanEventGift{From: p, To: (p + 1) % 3, Color: -1})
					}
				}
				if kind == "robber_flees" {
					g.CardEvent.Players = []int{0}
					g.CardEvent.Production, g.Tiles[0].Number = 4, 4
					g.Robber = 0
					// Keep two explicit known deserts; the production tile remains 0.
					for i := range g.Tiles {
						if g.Tiles[i].Resource == game.CatanDesert {
							g.Tiles[i].Resource = 1
						}
					}
					g.Tiles[1].Resource, g.Tiles[2].Resource = game.CatanDesert, game.CatanDesert
				}
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				first := cardEventHTTPAction(t, s.rooms[id].Game)
				before, _ := json.Marshal(s.rooms[id])
				for _, p := range []int{1, 3} {
					clients[p].command(current(clients[p]), "action", first, 400)
				}
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_earthquake", Edge: 0}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid event choice changed room")
				}
				if reward {
					clients[0].command(current(clients[0]), "action", first, 200)
					if s.rooms[id].Game.Catan.Players[0].Resources[0] != 1 || s.rooms[id].Game.Catan.Players[1].Resources[0] != 0 {
						t.Fatal("gift not applied immediately or production ran too early")
					}
				}
				if kind == "good_neighbors" {
					clients[0].command(current(clients[0]), "action", first, 200)
					g = s.rooms[id].Game.Catan
					if g.CardEvent.Gifts[0].Color != 1 || g.Players[0].Resources[1] != 1 || g.Players[1].Resources[1] != 0 {
						t.Fatal("private gift missing or transferred before all selections")
					}
				}
				if kind == "helpful_neighbor" {
					clients[0].command(current(clients[0]), "action", first, 200)
					g = s.rooms[id].Game.Catan
					if g.Players[0].Resources[1] != 0 || g.Players[2].Resources[1] != 1 || g.Players[0].Resources[0] != 0 {
						t.Fatal("first helpful gift not transferred or production ran early")
					}
				}
				actor := s.rooms[id].Game.CatanPendingActor()
				for viewer, c := range clients {
					view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					key := "eventResources"
					if kind == "robber_flees" {
						key = "fleeDeserts"
					}
					if kind == "helpful_neighbor" {
						key = "eventGifts"
						targets := view["legal"].(map[string]any)["eventTargets"].([]any)
						if (len(targets) == 1) != (viewer == actor) || len(targets) == 1 && targets[0].(float64) != 2 {
							t.Fatal("helpful recipient choices shown to wrong actor")
						}
					}
					if kind == "good_neighbors" {
						key = "eventGifts"
						q := view["cardEvent"].(map[string]any)
						if _, leaked := q["gifts"]; leaked {
							t.Fatal("private selections exposed over HTTP")
						}
						own, ok := q["ownGift"].(map[string]any)
						if viewer == 3 && ok || viewer < 3 && (!ok || int(own["from"].(float64)) != viewer) {
							t.Fatal("wrong viewer received private selection")
						}
						if viewer == 0 && own["color"].(float64) != 1 {
							t.Fatal("giver cannot see their pending choice")
						}
					}
					if (len(view["legal"].(map[string]any)[key].([]any)) > 0) != (viewer == actor) {
						t.Fatal("event legal choices exposed to wrong seat")
					}
					for p, raw := range view["players"].([]any) {
						_, visible := raw.(map[string]any)["resources"]
						if visible != (p == viewer) {
							t.Fatal("event choice leaked hand")
						}
					}
				}
				before, _ = json.Marshal(s.rooms[id])
				ts.Close()
				s.Close()
				next, err := New(s.cfg, s.files)
				if err != nil {
					t.Fatal(err)
				}
				defer next.Close()
				stopBotTicker(next)
				after, _ = json.Marshal(next.rooms[id])
				if string(before) != string(after) {
					t.Fatal("partial resource/flee response changed after restart")
				}
				ts2 := httptest.NewServer(next.Handler())
				defer ts2.Close()
				for _, c := range clients {
					c.base = ts2.URL
				}
				for step := 0; step < 3 && next.rooms[id].Game.CatanPendingActor() >= 0; step++ {
					r = next.rooms[id]
					actor = r.Game.CatanPendingActor()
					resolvedAt := time.Now()
					if mode == "manual" {
						clients[actor].command(current(clients[actor]), "action", cardEventHTTPAction(t, r.Game), 200)
					} else if mode == "autoplay" {
						setAutoPlay(clients[actor], current(clients[actor]), true, 200)
						next.mu.Lock()
						next.rooms[id].BotAt = 0
						next.runBots(resolvedAt)
						next.mu.Unlock()
					} else {
						resolvedAt = time.UnixMilli(r.TurnDeadline)
						next.mu.Lock()
						next.expireSetups(resolvedAt)
						next.mu.Unlock()
					}
					r = next.rooms[id]
					want := int64(45000)
					if r.Game.CatanPendingActor() >= 0 {
						want = 120000
					}
					got := r.TurnDeadline - resolvedAt.UnixMilli()
					if got < want || got > want+1000 || r.CatanTimeLeft != 45000 {
						t.Fatal("event response timing", got, want)
					}
				}
				r = next.rooms[id]
				g = r.Game.Catan
				if g.CardEvent != nil || r.Game.Phase != "catan_turn" || g.RollID != 1 || r.Game.Turn != 0 {
					t.Fatal("event did not finish once")
				}
				for seat, p := range g.Players {
					total := 0
					for _, count := range p.Resources {
						total += count
					}
					want := 1
					if reward || kind == "good_neighbors" {
						want = 2
					}
					if kind == "helpful_neighbor" && seat == 2 {
						want = 3
					}
					if total != want || p.Resources[0] < 1 {
						t.Fatal("wrong event reward/production count", p.Resources)
					}
					if kind == "good_neighbors" && p.Resources[(seat+2)%3+1] != 1 {
						t.Fatal("gift transfer lost, duplicated or sent to wrong neighbor", p.Resources)
					}
				}
				if kind == "robber_flees" && (g.Robber < 0 || g.Tiles[g.Robber].Resource != game.CatanDesert || len(g.Victims) != 0) {
					t.Fatal("flee destination or victim list")
				}
			})
		}
	}
}

func TestCatanCardEpidemicHTTPProgressResponseRestart(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			s.mu.Lock()
			r := s.rooms[id]
			cardEventResponseFixture(t, r, "city", time.Now())
			g := r.Game.Catan
			g.CardEvent = nil
			g.CitiesKnights.Event = &game.CatanCityEvent{Red: 2, Production: 6, Face: 0, Epidemic: true, Tasks: []game.CatanCityEventTask{}}
			g.CitiesKnights.Pending = &game.CatanCityPending{Kind: "progress_discard", Players: []int{1}}
			cityProgressGive(t, g, 1, 0, 3, 4, 5, 6)
			r.Game.Phase = "catan_progress_discard"
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_progress_discard", Cards: []int{3}}, 400)
			before, _ := json.Marshal(s.rooms[id])
			ts.Close()
			s.Close()
			next, err := New(s.cfg, s.files)
			if err != nil {
				t.Fatal(err)
			}
			defer next.Close()
			stopBotTicker(next)
			after, _ := json.Marshal(next.rooms[id])
			if string(before) != string(after) {
				t.Fatal("epidemic production modifier did not persist")
			}
			ts2 := httptest.NewServer(next.Handler())
			defer ts2.Close()
			for _, c := range clients {
				c.base = ts2.URL
			}
			resolvedAt := time.Now()
			if mode == "manual" {
				clients[1].command(current(clients[1]), "action", game.Action{Type: "catan_progress_discard", Cards: []int{3}}, 200)
			} else if mode == "autoplay" {
				setAutoPlay(clients[1], current(clients[1]), true, 200)
				next.mu.Lock()
				next.rooms[id].BotAt = 0
				next.runBots(resolvedAt)
				next.mu.Unlock()
			} else {
				resolvedAt = time.UnixMilli(next.rooms[id].TurnDeadline)
				next.mu.Lock()
				next.expireSetups(resolvedAt)
				next.mu.Unlock()
			}
			r = next.rooms[id]
			g = r.Game.Catan
			if r.Game.Phase != "catan_turn" || g.CitiesKnights.Event != nil || g.RollID != 1 || g.CitiesKnights.Pending != nil || len(g.CitiesKnights.Players[1].Progress) != 4 {
				t.Fatal("epidemic/progress response failed to finish")
			}
			remaining := r.TurnDeadline - resolvedAt.UnixMilli()
			if remaining < 45000 || remaining > 46000 {
				t.Fatal("original action clock not resumed")
			}
			for _, p := range g.Players {
				for color, count := range p.Resources {
					want := 0
					if color == 0 {
						want = 1
					}
					if count != want {
						t.Fatal("restored epidemic produced normal city/commodity", p.Resources)
					}
				}
			}
		})
	}
}
