package server

import (
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func assertPirateKnightInventory(t *testing.T, g *game.Catan) {
	t.Helper()
	if g.Seafarers.PirateIslands.KnightsRules != game.CatanPirateKnightsRules || g.Robber != -1 || len(g.DevDeck)+len(g.DevDiscard) != 0 {
		t.Fatal("pirate knight recipe")
	}
	rules := game.CatanProgressRules()
	counts := make([]int, len(rules))
	add := func(card int) {
		if card < 0 || card >= len(rules) {
			t.Fatal("invalid progress card")
		}
		counts[card]++
	}
	for track, deck := range g.CitiesKnights.ProgressDecks {
		for _, card := range deck {
			add(card)
			if rules[card].Track != track {
				t.Fatal("wrong progress track")
			}
		}
	}
	for _, p := range g.CitiesKnights.Players {
		for _, card := range p.Progress {
			add(card)
			if rules[card].Victory {
				t.Fatal("private VP")
			}
		}
		for _, card := range p.PublicProgress {
			add(card)
			if !rules[card].Victory {
				t.Fatal("public non-VP")
			}
		}
	}
	for card, r := range rules {
		if counts[card] != r.Count {
			t.Fatal("progress inventory")
		}
	}
	for color, total := range g.Bank {
		for _, p := range g.Players {
			total += p.Resources[color]
		}
		want := 19
		if len(g.Players) > 4 {
			want = 24
		}
		if color >= 5 {
			want = 12
			if len(g.Players) > 4 {
				want = 18
			}
		}
		if total != want {
			t.Fatal("resource inventory", color, total, want)
		}
	}
}

func TestCatanPirateKnightsToggleRestoreOrdinary(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, c := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("海盗骑士房主")
	c.register("海盗骑士玩家")
	r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "海盗骑士切换", "capacity": 3, "catanScenario": "pirate_islands", "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "catanEvents": game.CatanEventCatalogue}, 201)
	id := r["id"].(string)
	c.command(r, "join", nil, 200)
	h.command(current(h), "add_bot", nil, 200)
	ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
	ready()
	selectCatanCitiesKnights(c, nil, 400)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	assertPirateKnightInventory(t, g)
	if g.EventDeck == nil || g.Seafarers.VictoryPoints != 12 || g.Seafarers.Pirate != -1 {
		t.Fatal("combined opening")
	}
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	selectCatanCitiesKnights(h, nil, 200)
	ready()
	h.command(current(h), "start", nil, 200)
	g = s.rooms[id].Game.Catan
	if g.CitiesKnights != nil || g.Seafarers.PirateIslands.KnightsRules != "" || g.Seafarers.PirateIslands.CityFleet != nil || g.Seafarers.VictoryPoints != 10 || len(g.DevDeck) != 20 || g.Seafarers.Pirate < 0 {
		t.Fatal("ordinary pirate inherited knights")
	}
}
