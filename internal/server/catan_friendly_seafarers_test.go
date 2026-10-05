package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

// Response fixture on a real scenario map; formal combined room configuration
// remains closed until the separate room/UI acceptance stage.
func TestCatanFriendlyPirateHTTPRestartManualAndAutoplay(t *testing.T) {
	for _, autoplay := range []bool{false, true} {
		name := "manual"
		if autoplay {
			name = "autoplay"
		}
		t.Run(name, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			state, err := game.NewCatanFriendlySeafarers(3, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "desert"})
			if err != nil {
				t.Fatal(err)
			}
			g := state.Catan
			g.SetupStep = g.SetupLimit()
			g.TurnSerial = 1
			state.Turn, state.Phase, g.ResumePhase = 0, "catan_robber", "catan_turn"
			// Two separated ships: one owner has two public points plus a hidden VP;
			// the other has three public points and can be targeted.
			first, second, badTile, goodTile := -1, -1, -1, -1
			for _, edge := range g.Edges {
				for _, tile := range edge.Tiles {
					if g.Tiles[tile].Resource == game.CatanSea {
						first, badTile = edge.ID, tile
						break
					}
				}
				if first >= 0 {
					break
				}
			}
			for _, edge := range g.Edges {
				if edge.ID == first {
					continue
				}
				for _, tile := range edge.Tiles {
					if g.Tiles[tile].Resource == game.CatanSea && !slices.Contains(g.Edges[first].Tiles, tile) {
						second, goodTile = edge.ID, tile
						break
					}
				}
				if second >= 0 {
					break
				}
			}
			if second < 0 {
				t.Fatal("map fixture lacks separate sea edges")
			}
			g.Edges[first].Owner, g.Edges[first].Ship = 1, true
			g.Edges[second].Owner, g.Edges[second].Ship = 2, true
			g.Players[0].Score, g.Players[1].Score, g.Players[2].Score = 2, 3, 3
			for i, card := range g.DevDeck {
				if card == 4 {
					g.DevDeck = append(g.DevDeck[:i], g.DevDeck[i+1:]...)
					g.Players[1].Dev[4] = 1
					break
				}
			}
			g.Players[1].Resources[0] = 1
			g.Bank[0]--
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
				t.Fatal("sea protection or clock lost on restart")
			}
			ts2 := httptest.NewServer(next.Handler())
			defer ts2.Close()
			for _, c := range clients {
				c.base = ts2.URL
			}
			for i, c := range clients {
				view := current(c)["game"].(map[string]any)["catan"].(map[string]any)
				protected := view["friendlyRobber"].(map[string]any)["protectedPlayers"].([]any)
				if len(protected) != 2 || protected[0] != float64(0) || protected[1] != float64(1) {
					t.Fatal("hidden VP removed protection")
				}
				legal := view["legal"].(map[string]any)["pirate"].([]any)
				if i == 0 && (slices.Contains(legal, any(float64(badTile))) || !slices.Contains(legal, any(float64(goodTile)))) || i != 0 && len(legal) != 0 {
					t.Fatal("wrong pirate hints", i, legal)
				}
				for owner, raw := range view["players"].([]any) {
					p := raw.(map[string]any)
					for _, key := range []string{"dev", "resources"} {
						if _, visible := p[key]; visible != (i == owner) {
							t.Fatal("private hand leaked")
						}
					}
				}
			}
			before, _ = json.Marshal(next.rooms[id])
			for _, i := range []int{1, 2, 3} {
				clients[i].command(current(clients[i]), "action", map[string]any{"type": "catan_pirate", "tile": goodTile}, 400)
			}
			clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_pirate", "tile": badTile}, 400)
			after, _ = json.Marshal(next.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid pirate request changed room")
			}
			deadline := next.rooms[id].TurnDeadline
			if autoplay {
				setAutoPlay(clients[0], current(clients[0]), true, 200)
				next.mu.Lock()
				next.rooms[id].BotAt = 0
				next.runBots(time.Now())
				next.mu.Unlock()
			} else {
				clients[0].command(current(clients[0]), "action", map[string]any{"type": "catan_pirate", "tile": goodTile}, 200)
			}
			r = next.rooms[id]
			g = r.Game.Catan
			if r.Game.Phase != "catan_turn" || g.Players[1].Resources[0] != 1 || g.Players[2].Resources[1] != 0 || g.Players[0].Resources[1] != 1 || r.TurnDeadline != deadline {
				t.Fatal("pirate did not steal from unprotected owner and retain clock", r.Game.Phase)
			}
		})
	}
}
