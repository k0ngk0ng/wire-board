package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// A controlled midgame boundary, not a claim of naturally exhausted supply.
// Move one existing ship to a safe known sea edge next to exactly one ordinary
// hidden land tile. No hidden face is changed and no component is created.
func explorerSupplyApproach(t *testing.T, state *game.State) (game.Action, int) {
	t.Helper()
	g, p := state.Catan, state.Turn
	x := g.Explorer
	contacts := func(edge int) []int {
		e := g.Edges[edge]
		ids := []int{}
		for _, h := range x.Board.Hidden {
			if !h.Revealed && (slices.Contains(g.Tiles[h.Tile].Vertices, e.A) || slices.Contains(g.Tiles[h.Tile].Vertices, e.B)) {
				ids = append(ids, h.Tile)
			}
		}
		return ids
	}
	sea := func(edge int) bool {
		for _, tile := range g.Edges[edge].Tiles {
			if g.Tiles[tile].Resource == game.CatanSea {
				return true
			}
		}
		return false
	}
	free := func(edge int) bool { return !slices.Contains(x.Fleet.Positions, edge) }
	for _, from := range g.Edges {
		if !sea(from.ID) || !free(from.ID) || len(contacts(from.ID)) != 0 {
			continue
		}
		for _, to := range g.Edges {
			if to.ID == from.ID || !sea(to.ID) || !free(to.ID) || (from.A != to.A && from.A != to.B && from.B != to.A && from.B != to.B) {
				continue
			}
			tiles := contacts(to.ID)
			if len(tiles) != 1 {
				continue
			}
			for _, h := range x.Board.Hidden {
				if h.Tile == tiles[0] && h.Resource >= 0 && h.Resource < 5 {
					x.Fleet.Positions[p*3] = from.ID
					return game.Action{Type: "catan_explorer_sail", Slot: p * 3, Targets: []int{to.ID}, Prompt: int(g.TurnSerial)}, h.Tile
				}
			}
		}
	}
	t.Fatal("missing one-step ordinary discovery position")
	return game.Action{}, -1
}

func TestCatanExplorerCityHTTPDiscoveryLimitedSupply(t *testing.T) {
	for _, n := range []int{3, 6} {
		for stock := 0; stock <= 1; stock++ {
			t.Run(fmt.Sprintf("players%d/stock%d", n, stock), func(t *testing.T) {
				s, ts, clients, id := newExplorerCityHTTP(t, n, "action")
				r := s.rooms[id]
				p := r.Game.Turn
				action, tile := explorerSupplyApproach(t, r.Game)
				g, x := r.Game.Catan, r.Game.Catan.Explorer
				color := -1
				for _, h := range x.Board.Hidden {
					if h.Tile == tile {
						color = h.Resource
					}
				}
				// Keep actor's existing hand unchanged; transfer bank stock to an opponent.
				g.Players[(p+1)%n].Resources[color] += g.Bank[color] - stock
				g.Bank[color] = stock
				if err := s.save(r); err != nil {
					t.Fatal(err)
				}
				explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_explorer_begin_move"})
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				r = s.rooms[id]
				g, x = r.Game.Catan, r.Game.Catan.Explorer
				hand, gold := slices.Clone(g.Players[p].Resources), slices.Clone(x.Economy.Gold)
				deadline, version := r.TurnDeadline, r.Version
				assertExplorerCityHTTPPrivacy(t, clients, r.Game)
				before, _ := json.Marshal(r)
				clients[(p+1)%n].command(current(clients[(p+1)%n]), "action", action, 400)
				clients[n].command(current(clients[n]), "action", action, 400)
				after, _ := json.Marshal(s.rooms[id])
				if string(before) != string(after) {
					t.Fatal("rejected discovery changed hidden state")
				}
				body := map[string]any{"type": "action", "action": action, "version": version, "nonce": randomID(12)}
				clients[p].post("/api/rooms/"+id, body, 200)
				r = s.rooms[id]
				g, x = r.Game.Catan, r.Game.Catan.Explorer
				hand[color] += stock
				if g.Tiles[tile].Resource != color || !slices.Equal(g.Players[p].Resources, hand) || g.Bank[color] != 0 || !slices.Equal(x.Economy.Gold, gold) || !x.Fleet.Turn.Ships[action.Slot].Closed || x.Fleet.Positions[action.Slot] != action.Targets[0] || r.TurnDeadline != deadline || r.Version != version+1 {
					t.Fatal("discovery shortage broke reward/stop/clock")
				}
				if stock == 0 && !strings.Contains(strings.Join(r.Game.Log, "\n"), "库存为空，未领取探索资源") {
					t.Fatal("missing explicit empty reward log")
				}
				snapshot, _ := json.Marshal(r)
				clients[p].post("/api/rooms/"+id, body, 200)
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				clients[p].post("/api/rooms/"+id, body, 200)
				clients[p].command(current(clients[p]), "action", action, 400)
				after, _ = json.Marshal(s.rooms[id])
				if string(snapshot) != string(after) {
					t.Fatal("replay repeated reward, number draw or movement")
				}
				assertExplorerCityHTTPPrivacy(t, clients, s.rooms[id].Game)
				explorerCityHTTPAction(t, clients, s, id, p, game.Action{Type: "catan_end"})
				if s.rooms[id].Game.Catan.TurnSerial <= g.TurnSerial {
					t.Fatal("shortage prevented handoff")
				}
			})
		}
	}
}
