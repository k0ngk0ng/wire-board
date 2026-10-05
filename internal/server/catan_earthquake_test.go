package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Damage is explicitly provisioned after legal setup. This checks the real
// repair endpoint/persistence/autoplay, not an event-deck draw or full game.
func TestCatanEarthquakeRepairHTTPRestart(t *testing.T) {
	for _, free := range []bool{false, true} {
		for _, autoplay := range []bool{false, true} {
			t.Run(fmt.Sprintf("free=%v/autoplay=%v", free, autoplay), func(t *testing.T) {
				s, ts, clients, id := newCatanTable(t)
				clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				s.mu.Lock()
				r := s.rooms[id]
				g := r.Game.Catan
				roads := []int{}
				for i := range g.Edges {
					if g.Edges[i].Owner == 0 {
						g.Edges[i].Damaged = true
						roads = append(roads, i)
					}
				}
				if len(roads) != 2 {
					t.Fatal("expected two setup roads")
				}
				for i := range g.Players {
					for color, count := range g.Players[i].Resources {
						g.Bank[color] += count
						g.Players[i].Resources[color] = 0
					}
				}
				r.Game.Turn, r.Game.Phase = 0, "catan_turn"
				if free {
					found := false
					for i, kind := range g.DevDeck {
						if kind == 1 {
							g.Players[0].Dev[1]++
							g.DevDeck = append(g.DevDeck[:i], g.DevDeck[i+1:]...)
							found = true
							break
						}
					}
					if !found {
						t.Fatal("missing Road Building fixture card")
					}
				} else {
					for color := range 2 {
						g.Players[0].Resources[color] = 2
						g.Bank[color] -= 2
					}
				}
				r.TurnDeadline = time.Now().Add(35 * time.Second).UnixMilli()
				deadline := r.TurnDeadline
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				s.mu.Unlock()
				if free {
					clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_dev", Card: 1}, 200)
				}
				// First repair before restart; resume the remaining repair afterwards.
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_repair_road", Edge: roads[0]}, 200)
				before, _ := json.Marshal(s.rooms[id])
				for _, player := range []int{1, 3} {
					clients[player].command(current(clients[player]), "action", game.Action{Type: "catan_repair_road", Edge: roads[1]}, 400)
				}
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_repair_road", Edge: roads[0]}, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("rejected repair changed room")
				}
				for viewer, c := range clients {
					view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
					legal := view["legal"].(map[string]any)
					if (len(legal["repairRoads"].([]any)) == 1) != (viewer == 0) || len(legal["roads"].([]any)) != 0 {
						t.Fatal("incorrect legal repair or new-road hints", viewer)
					}
					for i, raw := range view["players"].([]any) {
						_, visible := raw.(map[string]any)["resources"]
						if visible != (viewer == i) {
							t.Fatal("resource privacy changed")
						}
					}
				}
				if autoplay {
					setAutoPlay(clients[0], current(clients[0]), true, 200)
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
					t.Fatal("restart changed damage, free allowance, autoplay or deadline")
				}
				ts2 := httptest.NewServer(next.Handler())
				defer ts2.Close()
				for _, c := range clients {
					c.base = ts2.URL
				}
				if autoplay {
					next.mu.Lock()
					next.rooms[id].BotAt = 0
					next.runBots(time.Now())
					next.mu.Unlock()
				} else {
					clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_repair_road", Edge: roads[1]}, 200)
				}
				result := next.rooms[id]
				g = result.Game.Catan
				if g.Edges[roads[0]].Damaged || g.Edges[roads[1]].Damaged || result.Game.Phase != "catan_turn" || g.FreeRoads != 0 || result.Game.Turn != 0 || result.TurnDeadline != deadline {
					t.Fatal("repair did not resume normally or refreshed turn clock")
				}
				for color, bank := range g.Bank {
					if bank != 19 || g.Players[0].Resources[color] != 0 {
						t.Fatal("repair resource conservation")
					}
				}
			})
		}
	}
}
