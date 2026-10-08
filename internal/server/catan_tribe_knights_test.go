package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func assertTribeProgressInventory(t *testing.T, g *game.Catan) {
	t.Helper()
	rules := game.CatanProgressRules()
	counts := make([]int, len(rules))
	add := func(card int) {
		if card < 0 || card >= len(rules) {
			t.Fatal("bad progress card")
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
	tr := g.Seafarers.Tribe
	if tr.ProgressRules != game.CatanTribeProgressRules {
		t.Fatal("tribe recipe lost")
	}
	for _, reward := range tr.Development {
		add(reward.Card)
	}
	for card, rule := range rules {
		if counts[card] != rule.Count {
			t.Fatal("progress supply", card, counts[card])
		}
	}
	for _, p := range g.Players {
		for _, n := range p.Dev {
			if n != 0 {
				t.Fatal("old development mixed in")
			}
		}
	}
}

func TestCatanTribeKnightsNaturalHTTP(t *testing.T) {
	for _, events := range []bool{false, true} {
		t.Run(fmt.Sprintf("events=%v", events), func(t *testing.T) {
			testCatanCitiesKnightsEventsFullHTTPGames(t, "tribe", true, true, events, 3, 4, 5, 6)
		})
	}
}

func TestCatanTribeKnightsToggleRestoreOrdinary(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, c := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("部落组合房主")
	c.register("部落组合玩家")
	r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "部落奖励切换", "capacity": 3, "catanScenario": "tribe", "catanCitiesKnights": game.CatanCitiesKnightsSetup{}, "catanEvents": game.CatanEventCatalogue}, 201)
	id := r["id"].(string)
	c.command(r, "join", nil, 200)
	h.command(current(h), "add_bot", nil, 200)
	ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
	ready()
	selectCatanCitiesKnights(c, nil, 400)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	assertTribeProgressInventory(t, g)
	if g.EventDeck == nil || g.Seafarers.VictoryPoints != 15 {
		t.Fatal("combo lost")
	}
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	selectCatanCitiesKnights(h, nil, 200)
	ready()
	h.command(current(h), "start", nil, 200)
	g = s.rooms[id].Game.Catan
	if g.CitiesKnights != nil || g.Seafarers.Tribe.ProgressRules != "" || g.Seafarers.VictoryPoints != 13 || len(g.DevDeck)+len(g.Seafarers.Tribe.Development) != 25 {
		t.Fatal("ordinary tribe inherited progress")
	}
	for _, reward := range g.Seafarers.Tribe.Development {
		if reward.Card < 0 || reward.Card >= 5 {
			t.Fatal("progress card in ordinary")
		}
	}
}
