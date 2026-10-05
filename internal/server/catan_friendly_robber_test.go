package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
	"time"
)

// Explicit response fixture: all productive land adjoins a protected player,
// and the desert adjoins one protected and one unprotected potential victim.
func TestCatanFriendlyRobberHTTPRestartAutoplayAndDiscardTimeout(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "discard_timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			state, err := game.NewCatanFriendlyRobber(3, game.CatanOptions{}, game.CatanBaseConfiguration{})
			if err != nil {
				t.Fatal(err)
			}
			g := state.Catan
			g.SetupStep = g.SetupLimit()
			g.TurnSerial = 1
			state.Turn, state.Phase, g.ResumePhase = 0, "catan_robber", "catan_turn"
			g.Tiles = []game.CatanTile{{ID: 0, Resource: game.CatanDesert, Vertices: []int{1, 2}}, {ID: 1, Resource: 0, Number: 6, Vertices: []int{1}}, {ID: 2, Resource: 1, Number: 8, Vertices: []int{1}}, {ID: 3, Resource: 2, Number: 5, Vertices: []int{1}}}
			g.Robber = 3
			g.Vertices[1].Owner, g.Vertices[1].Level = 1, 1
			g.Vertices[2].Owner, g.Vertices[2].Level = 2, 1
			g.Players[0].Score, g.Players[1].Score, g.Players[2].Score = 2, 3, 3
			// Player 1's third point is secret and must not remove protection.
			for i, card := range g.DevDeck {
				if card == 4 {
					g.DevDeck = append(g.DevDeck[:i], g.DevDeck[i+1:]...)
					g.Players[1].Dev[4] = 1
					break
				}
			}
			count := 1
			if mode == "discard_timeout" {
				count = 8
				state.Phase = "catan_discard"
				g.DiscardDue[1] = 4
			}
			g.Players[1].Resources[0] = count
			g.Bank[0] -= count
			g.Players[2].Resources[1] = 1
			g.Bank[1]--
			s.mu.Lock()
			r := s.rooms[id]
			r.Game = state
			r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
			if err = s.save(r); err != nil {
				t.Fatal(err)
			}
			s.mu.Unlock()
			before, _ := json.Marshal(r)
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
				t.Fatal("friendly state/clock lost on restart")
			}
			ts2 := httptest.NewServer(next.Handler())
			defer ts2.Close()
			for _, c := range clients {
				c.base = ts2.URL
			}
			if mode == "discard_timeout" {
				next.mu.Lock()
				at := time.UnixMilli(next.rooms[id].TurnDeadline)
				next.expireSetups(at)
				next.mu.Unlock()
				r = next.rooms[id]
				if r.Game.Phase != "catan_robber" || r.Game.Catan.Players[1].Resources[0] != 4 || r.TurnDeadline != at.Add(turnLimit).UnixMilli() {
					t.Fatal("discard timeout did not reach robber with standard clock")
				}
				count = 4
			}
			before, _ = json.Marshal(next.rooms[id])
			for _, p := range []int{1, 2, 3} {
				clients[p].command(current(clients[p]), "action", map[string]any{"type": "catan_robber", "tile": 0}, 400)
			}
			clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_robber", "tile": 1}, 400)
			after, _ = json.Marshal(next.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid friendly action changed room")
			}
			for viewer, c := range clients {
				v := current(c)["game"].(map[string]any)["catan"].(map[string]any)
				protected := v["friendlyRobber"].(map[string]any)["protectedPlayers"].([]any)
				if len(protected) != 2 || protected[0] != float64(0) || protected[1] != float64(1) {
					t.Fatal("hidden VP affected public protection", viewer, protected)
				}
				legal := v["legal"].(map[string]any)["robber"].([]any)
				if (viewer == 0 && (len(legal) != 1 || legal[0] != float64(0))) || (viewer != 0 && len(legal) != 0) {
					t.Fatal("legal hints disagree", viewer, legal)
				}
				for owner, raw := range v["players"].([]any) {
					p := raw.(map[string]any)
					for _, key := range []string{"dev", "resources"} {
						if _, visible := p[key]; visible != (owner == viewer) {
							t.Fatal("private hand leaked")
						}
					}
				}
			}
			deadline := next.rooms[id].TurnDeadline
			if mode == "autoplay" {
				setAutoPlay(clients[0], current(clients[0]), true, 200)
				next.mu.Lock()
				next.rooms[id].BotAt = 0
				next.runBots(time.Now())
				next.mu.Unlock()
			} else {
				clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_robber", "tile": 0}, 200)
			}
			r = next.rooms[id]
			if r.Game.Phase != "catan_turn" || r.Game.Catan.Robber != 0 || r.Game.Catan.Players[1].Resources[0] != count || r.Game.Catan.Players[2].Resources[1] != 0 || r.Game.Catan.Players[0].Resources[1] != 1 || r.TurnDeadline != deadline {
				t.Fatal("fallback/theft/clock", r.Game.Phase, r.TurnDeadline, deadline)
			}
		})
	}
}
