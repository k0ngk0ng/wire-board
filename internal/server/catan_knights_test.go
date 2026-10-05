package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanKnightsHTTPDisplacementRestoreAndResponses(t *testing.T) {
	for _, variant := range []string{"base", "cloth"} {
		for _, mode := range []string{"manual", "timeout", "autoplay"} {
			t.Run(variant+"/"+mode, func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				s.mu.Lock()
				r := s.rooms[id]
				state, err := game.NewCatanCitiesKnights(3, game.CatanOptions{})
				if err != nil {
					t.Fatal(err)
				}
				g := state.Catan
				g.SetupStep = 6
				g.TurnSerial = 1
				g.Ports = nil
				state.Turn, state.Phase = 0, "catan_turn"
				g.Vertices = []game.CatanVertex{{ID: 0, Owner: -1}, {ID: 1, Owner: -1}, {ID: 2, Owner: -1}, {ID: 3, Owner: -1}}
				g.Edges = []game.CatanEdge{{ID: 0, A: 0, B: 1, Owner: 0}, {ID: 1, A: 1, B: 2, Owner: 0}, {ID: 2, A: 2, B: 3, Owner: 1}}
				g.Tiles = []game.CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0, 1, 2, 3}}}
				g.CitiesKnights.ActionSerial = 5
				g.CitiesKnights.Knights = []game.CatanKnight{{Owner: 0, Vertex: 0, Strength: 2, Active: true}, {Owner: 1, Vertex: 2, Strength: 1, Active: true, ActivatedAt: 2, PromotedAt: 3}}
				if variant == "cloth" {
					// Internal response fixture, not an enabled room recipe. Player 2's
					// route 4--0--5 is opened by the attacker's departure from vertex 0.
					g.SetupStep = 9
					g.Vertices = append(g.Vertices, game.CatanVertex{ID: 4, Owner: 2, Level: 1}, game.CatanVertex{ID: 5, Owner: -1})
					g.Edges = append(g.Edges, game.CatanEdge{ID: 3, A: 4, B: 0, Owner: 2, Ship: true}, game.CatanEdge{ID: 4, A: 0, B: 5, Owner: 2, Ship: true})
					g.Seafarers = &game.CatanSeafarers{Scenario: "cloth", Pirate: -1, VictoryPoints: 16, Cloth: &game.CatanClothState{
						Stock: 10, Held: make([]int, 3), HomeTiles: []int{0}, EmptyLimit: 5,
						Villages: []game.CatanClothVillage{{Vertex: 5, Number: 6, Stock: 5}},
					}}
				}
				r.Game = state
				r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
				if err = s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_knight_move", "vertex": 0, "target": 2}, 200)
				r = s.rooms[id]
				if r.Game.CatanPendingActor() != 1 || r.Game.Turn != 0 || r.CatanTimeLeft < 44000 || r.CatanTimeLeft > 45000 {
					t.Fatal("displacement did not pause attacker's clock")
				}
				remaining := r.TurnDeadline - time.Now().UnixMilli()
				if remaining < 119000 || remaining > 120000 {
					t.Fatal("retreat response must receive 120 seconds")
				}
				if variant == "cloth" && r.Game.Catan.Seafarers.Cloth.Held[2] != 0 {
					t.Fatal("trade awarded before mandatory retreat finished")
				}
				savedTime := r.CatanTimeLeft
				before, _ := json.Marshal(r)
				for _, p := range []int{0, 2, 3} {
					clients[p].command(current(clients[p]), "action", map[string]any{"type": "catan_knight_retreat", "vertex": 3}, 400)
				}
				clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_knight_retreat", "vertex": 0}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("invalid retreat mutated room")
				}
				for p, c := range clients {
					view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					legal := view["legal"].(map[string]any)["knightRetreat"].([]any)
					if (len(legal) > 0) != (p == 1) {
						t.Fatal("HTTP retreat legal locations for wrong actor")
					}
					for owner, raw := range view["players"].([]any) {
						_, visible := raw.(map[string]any)["resources"]
						if visible != (owner == p) {
							t.Fatal("retreat response exposed private hand")
						}
					}
				}
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
					t.Fatal("retreat or activation locks lost on restart")
				}
				ts2 := httptest.NewServer(next.Handler())
				defer ts2.Close()
				for _, c := range clients {
					c.base = ts2.URL
				}
				var resolvedAt time.Time
				switch mode {
				case "manual":
					clients[1].command(current(clients[1]), "action", map[string]any{"type": "catan_knight_retreat", "vertex": 3}, 200)
					resolvedAt = time.Now()
				case "timeout":
					next.mu.Lock()
					resolvedAt = time.UnixMilli(next.rooms[id].TurnDeadline)
					next.expireSetups(resolvedAt)
					next.mu.Unlock()
				case "autoplay":
					next.mu.Lock()
					next.rooms[id].Seats[1].AutoPlay = true
					next.rooms[id].BotAt = 0
					resolvedAt = time.Now()
					next.runBots(resolvedAt)
					next.mu.Unlock()
				}
				r = next.rooms[id]
				if r.Game.Phase != "catan_turn" || r.Game.Turn != 0 || r.Game.CatanPendingActor() != -1 {
					t.Fatal("retreat failed to resume original player")
				}
				found := false
				for _, n := range r.Game.Catan.CitiesKnights.Knights {
					if n.Owner == 1 {
						found = n.Vertex == 3 && n.Active && n.ActivatedAt == 2 && n.PromotedAt == 3
					}
				}
				if !found {
					t.Fatal("retreated knight lost position or activation/promotion locks")
				}
				if variant == "cloth" {
					cloth := r.Game.Catan.Seafarers.Cloth
					if cloth.Held[2] != 1 || cloth.Villages[0].Stock != 4 || cloth.Stock != 10 || len(cloth.Villages[0].Traders) != 1 || cloth.Villages[0].Traders[0] != 2 {
						t.Fatal("completed retreat did not establish third player's trade exactly once", cloth)
					}
					view := current(clients[3])["game"].(map[string]any)["catan"].(map[string]any)
					public := view["seafarers"].(map[string]any)["cloth"].(map[string]any)
					if public["held"].([]any)[2] != float64(1) {
						t.Fatal("spectator did not receive public cloth result")
					}
				}
				remaining = r.TurnDeadline - resolvedAt.UnixMilli()
				if remaining > savedTime || remaining < savedTime-1000 {
					t.Fatal("retreat restored wrong action clock", remaining, savedTime)
				}
			})
		}
	}
}
