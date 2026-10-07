package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Controlled midgame with real catalog cards and conserved inventories. This
// covers a specific nested chain, not a claim of natural play reaching it.
func orientChainTable(t *testing.T, cities bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	mask := 11
	if cities {
		mask = 15
	}
	s, ts, clients, id := modernSplendorTable(t, 3, mask)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	st, g := r.Game, r.Game.Splendor
	all := map[int]game.Card{}
	for _, rows := range [][][]game.Card{g.Decks, g.Market} {
		for _, row := range rows {
			for _, c := range row {
				all[c.ID] = c
			}
		}
	}
	take := func(id int) game.Card {
		c, ok := all[id]
		if !ok {
			t.Fatal("missing card", id)
		}
		delete(all, id)
		return c
	}
	// Preserve a reproducible deck order and exactly one copy of every card.
	g.Market = make([][]game.Card, 6)
	g.Decks = make([][]game.Card, 6)
	g.Market[3] = []game.Card{take(1006), take(1001)}
	g.Market[4] = []game.Card{take(1016), take(1014)}
	g.Market[5] = []game.Card{take(1021), take(1026)}
	p := &g.Players[0]
	p.Cards = []game.Card{take(1013)}
	for id := 1; id <= 90; id++ {
		if c, ok := all[id]; ok && c.Color == 2 && c.Points == 0 {
			p.Cards = append(p.Cards, take(id))
			break
		}
	}
	p.Score = 1
	p.Bonus = []int{2, 0, 1, 0, 0}
	p.TradingPosts = []int{game.GemPostPurchaseToken, game.GemPostBlindReserve}
	p.Tokens = []int{1, 0, 5, 0, 1, 0}
	for i, n := range p.Tokens {
		g.Bank[i] -= n
	}
	ids := make([]int, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		c := all[id]
		row := c.Tier - 1
		if c.Orient != "" {
			row += 3
		}
		if row < 3 && len(g.Market[row]) < 4 {
			g.Market[row] = append(g.Market[row], c)
		} else {
			g.Decks[row] = append(g.Decks[row], c)
		}
	}
	g.Strongholds = map[int]game.GemStronghold{1026: {Player: 0, Count: 2}, g.Market[0][0].ID: {Player: 1, Count: 1}}
	st.Turn = 0
	st.Phase = "turn"
	r.Version++
	r.startTurnClock(time.Now())
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	return s, ts, clients, id
}

func TestSplendorOrientCatalogChainHTTP(t *testing.T) {
	for _, cities := range []bool{false, true} {
		t.Run(fmt.Sprint("cities=", cities), func(t *testing.T) {
			s, ts, clients, id := orientChainTable(t, cities)
			defer ts.Close()
			supply := []int{5, 5, 5, 5, 5, 5}
			initial := s.rooms[id].Game.Splendor
			refills := []struct{ row, slot, after, card, count int }{
				{5, 0, 0, initial.Decks[5][0].ID, len(initial.Decks[5])},
				{4, 0, 1, initial.Decks[4][0].ID, len(initial.Decks[4])},
				{3, 0, 3, initial.Decks[3][0].ID, len(initial.Decks[3])},
				{5, 1, 7, initial.Decks[5][1].ID, len(initial.Decks[5])},
			}
			steps := []struct {
				action game.Action
				phase  string
			}{
				{game.Action{Type: "buy", Card: 1021}, "gem_free_card"},
				{game.Action{Type: "gem_free_card", Card: 1016}, "gem_copy"},
				{game.Action{Type: "gem_copy", Card: 1013}, "gem_free_card"},
				{game.Action{Type: "gem_free_card", Card: 1006}, "gem_copy"},
				{game.Action{Type: "gem_copy", Card: 1016}, "gem_token"},
				{game.Action{Type: "gem_token", Color: 2}, "gem_stronghold"},
				{game.Action{Type: "gem_stronghold", Choice: "place", Card: 1026}, "gem_conquest"},
				{game.Action{Type: "gem_conquest", Card: 1026, Cards: []int{1006, 1016}}, "gem_token"},
				{game.Action{Type: "gem_token", Color: 4}, "gem_stronghold"},
			}
			for i, step := range steps {
				before := current(clients[0])
				clients[0].command(before, "action", step.action, 200)
				st := s.rooms[id].Game
				if st.Phase != step.phase {
					t.Fatalf("step %d: %s", i, st.Phase)
				}
				assertModernComponents(t, st, supply)
				assertModernPrivacy(t, clients, st)
				// Chain holes must remain until all purchase and conquest effects finish.
				for _, hole := range refills {
					if i >= hole.after && (st.Splendor.Market[hole.row][hole.slot].ID != 0 || len(st.Splendor.Decks[hole.row]) != hole.count) {
						t.Fatal("early refill", i, hole)
					}
				}
				if i == 4 && !slices.Equal(st.Splendor.Players[0].Bonus, []int{6, 1, 1, 0, 0}) {
					t.Fatal("copy of copied double bonus")
				}
				clients[0].command(before, "action", step.action, 409)
				raw, err := json.Marshal(s.rooms[id])
				if err != nil {
					t.Fatal(err)
				}
				restored, err := New(s.cfg, s.files)
				if err != nil {
					t.Fatal(err)
				}
				after, _ := json.Marshal(restored.rooms[id])
				if string(raw) != string(after) {
					t.Fatal("recovery mismatch")
				}
			}
			// Remove the lone opponent tower; only then refill all four card holes.
			blocked := s.rooms[id].Game.Splendor.Market[0][0].ID
			clients[0].command(current(clients[0]), "action", game.Action{Type: "gem_stronghold", Choice: "remove", Card: blocked}, 200)
			st := s.rooms[id].Game
			g := st.Splendor
			if st.Turn != 1 || st.Phase != "turn" || len(g.Refills) != 0 {
				t.Fatal("chain did not finish", st.Phase, st.Turn)
			}
			assertModernComponents(t, st, supply)
			for _, hole := range refills {
				if g.Market[hole.row][hole.slot].ID != hole.card {
					t.Fatal("wrong refill position or deck order", hole)
				}
			}
			if len(g.Exiled) != 2 || !slices.Equal(g.Players[0].Bonus, []int{2, 2, 1, 0, 0}) || g.Players[0].Score != 5 || !slices.Equal(g.Players[0].Tokens, []int{0, 0, 1, 0, 1, 0}) {
				t.Fatal("wrong final inventory", g.Players[0])
			}
			actions := []string{}
			for _, ev := range g.CardEvents {
				actions = append(actions, ev.Action)
			}
			if !slices.Equal(actions, []string{"buy", "free", "free", "buy"}) {
				t.Fatal("missing chain events", actions)
			}
		})
	}
}

func orientChainGold(g *game.Splendor) {
	// Transfer an existing zero-point gold card from the market to the controlled
	// player's collection; refill from its own deck without duplicating a card.
	g.Players[0].Cards = append(g.Players[0].Cards, g.Market[3][1])
	g.Market[3][1] = g.Decks[3][0]
	g.Decks[3] = g.Decks[3][1:]
}

func TestSplendorOrientCatalogGoldAndBlindReserveHTTP(t *testing.T) {
	for _, gold := range []bool{false, true} {
		t.Run(fmt.Sprint("gold=", gold), func(t *testing.T) {
			s, ts, clients, id := orientChainTable(t, false)
			defer ts.Close()
			r := s.rooms[id]
			g := r.Game.Splendor
			supply := []int{5, 5, 5, 5, 5, 5}
			if gold {
				orientChainGold(g)
				clients[0].command(current(clients[0]), "action", game.Action{Type: "buy", Card: 1021, Cards: []int{1001}, Tokens: []int{0, 0, 5, 0, 0, 0}}, 200)
				g = s.rooms[id].Game.Splendor
				if len(g.Exiled) != 1 || g.Exiled[0].ID != 1001 || !slices.Equal(g.Players[0].Tokens, []int{1, 0, 0, 0, 1, 0}) || g.Bank[5] != 5 || s.rooms[id].Game.Phase != "gem_free_card" {
					t.Fatal("gold card payment mismatch")
				}
			} else {
				choices := slices.Clone(g.Decks[3][:2])
				before := len(g.Decks[3])
				clients[0].command(current(clients[0]), "action", game.Action{Type: "reserve", Tier: 1, Choice: "orient"}, 200)
				st := s.rooms[id].Game
				assertModernPrivacy(t, clients, st)
				if st.Phase != "gem_reserve" || len(st.Splendor.ReserveChoice) != 2 {
					t.Fatal("missing private selection")
				}
				clients[0].command(current(clients[0]), "action", game.Action{Type: "gem_reserve", Card: choices[1].ID}, 200)
				g = s.rooms[id].Game.Splendor
				if len(g.Decks[3]) != before-1 || g.Decks[3][len(g.Decks[3])-1].ID != choices[0].ID || g.Players[0].Reserved[0].ID != choices[1].ID {
					t.Fatal("wrong orient reserve/return")
				}
				if g.CardEvents[len(g.CardEvents)-1].Card != nil || !g.CardEvents[len(g.CardEvents)-1].Orient {
					t.Fatal("blind reservation event exposed face")
				}
			}
			assertModernComponents(t, s.rooms[id].Game, supply)
			assertModernPrivacy(t, clients, s.rooms[id].Game)
		})
	}
}
