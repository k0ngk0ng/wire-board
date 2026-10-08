package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanTransportPublicSelectionAndRematch(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
			host.register("运输房主")
			guest.register("运输朋友")
			raw := host.post("/api/rooms", map[string]any{"name": "运输设置", "kind": "catan", "capacity": n, "catanScenario": "transport"}, 201)
			id := raw["id"].(string)
			guest.command(current(host), "join", nil, 200)
			change := func(c *testClient, kind, key, value string, status int) {
				c.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			for _, c := range []*testClient{host, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			change(host, "catan_scenario", "catanScenario", "transport", 200)
			if !s.rooms[id].Seats[0].Ready || !s.rooms[id].Seats[1].Ready {
				t.Fatal("same scenario cleared readiness")
			}
			before, _ := json.Marshal(s.rooms[id])
			change(guest, "catan_scenario", "catanScenario", "transport", 400)
			change(host, "catan_scenario", "catanScenario", "unknown-mission", 400)
			host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{Helpers: true}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid command changed room")
			}
			if n == 2 {
				change(host, "catan_two_scenario", "catanTwoScenario", "rivers", 200)
			} else if n > 4 {
				change(host, "catan_scenario", "catanScenario", "spices-for-catan", 200)
			} else {
				change(host, "catan_scenario", "catanScenario", "shores", 200)
			}
			change(host, "catan_scenario", "catanScenario", "transport", 200)
			if s.rooms[id].CatanTwoRules != "" || s.rooms[id].CatanSeafarers != nil || s.rooms[id].Seats[0].Ready || s.rooms[id].Seats[1].Ready {
				t.Fatal("previous recipe or readiness survived")
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
			// A larger room may start with two actual players, using the two-player neutral construction and trade tokens.
			for _, c := range []*testClient{host, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			if len(s.rooms[id].Game.Catan.Players) != 2 || s.rooms[id].Game.Catan.Two == nil || s.rooms[id].Game.Catan.Transport == nil {
				t.Fatal("capacity used instead of players")
			}
			change(host, "catan_scenario", "catanScenario", "transport", 400)
			host.command(current(host), "close", nil, 200)
			host.command(current(host), "rematch", nil, 200)
			if s.rooms[id].CatanScenario != "transport" {
				t.Fatal("rematch lost scenario")
			}
			if n > 4 {
				return
			}
			if n == 2 {
				change(host, "catan_two_scenario", "catanTwoScenario", "", 200)
			} else {
				change(host, "catan_scenario", "catanScenario", "", 200)
				host.command(current(host), "add_bot", nil, 200)
			}
			for _, c := range []*testClient{host, guest} {
				c.command(current(c), "ready", nil, 200)
			}
			host.command(current(host), "start", nil, 200)
			if s.rooms[id].Game.Catan.Transport != nil || (s.rooms[id].Game.Catan.Two != nil) != (n == 2) {
				t.Fatal("rematch used old transport state")
			}
		})
	}
}

func TestCatanTransportPublicRejectsUnsupportedRecipes(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	c := newClient(t, ts.URL)
	c.register("运输组合检查")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2, "catanTwoScenario": "rivers"},
		{"kind": "catan", "capacity": 3, "catanOptions": game.CatanOptions{Helpers: true}},
		{"kind": "catan", "capacity": 5, "catanOptions": game.CatanOptions{FiveSix: true}},
		{"kind": "catan", "capacity": 4, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}},
		{"kind": "splendor", "capacity": 2},
	} {
		body["name"], body["catanScenario"] = "不支持组合", "transport"
		c.post("/api/rooms", body, 400)
	}
}

func assertTransportInventory(t *testing.T, s *game.State) {
	t.Helper()
	g := s.Catan
	tr := g.Transport
	total := tr.GoldBank
	for _, n := range tr.Gold {
		total += n
	}
	gold, resources, tokens := 100, 19, 36
	wantCards := [5]int{16, 3, 3, 0, 3}
	if len(g.Players) > 4 {
		gold, resources, tokens = 152, 24, 54
		wantCards = [5]int{24, 5, 5, 0, 3}
		if tr.DeckRecipe != game.CatanTransportExtendedDeck {
			t.Fatal("missing deck version")
		}
	}
	if total != gold+tr.GoldIssued {
		t.Fatal("transport gold imbalance")
	}
	cards := [5]int{}
	for _, pile := range [][]int{g.DevDeck, g.DevDiscard} {
		for _, card := range pile {
			cards[card]++
		}
	}
	for color, bank := range g.Bank {
		for _, p := range g.Players {
			bank += p.Resources[color]
		}
		if bank != resources {
			t.Fatal("resource imbalance", color, bank)
		}
	}
	for _, p := range g.Players {
		for card, count := range p.Dev {
			cards[card] += count
		}
	}
	if cards != wantCards {
		t.Fatal("transport development deck", cards)
	}
	cargo := map[int]bool{}
	add := func(id int) {
		if id == 0 {
			return
		}
		if cargo[id] {
			t.Fatal("duplicate cargo", id)
		}
		cargo[id] = true
	}
	for _, pile := range tr.Stacks {
		for _, id := range pile {
			add(id)
		}
	}
	for _, w := range tr.Wagons {
		add(w.Cargo)
		for _, id := range w.Delivered {
			add(id)
		}
	}
	if len(cargo) != tokens {
		t.Fatal("lost cargo", len(cargo))
	}
}

func assertTransportHistory(t *testing.T, s *Server, clients []*testClient, id string) {
	t.Helper()
	r := s.rooms[id]
	s.mu.Lock()
	err := s.save(r)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	winner := r.Game.Winners[0]
	status, profile := clients[len(clients)-1].request("GET", "/api/players/"+r.Seats[winner].ID, nil)
	if status != 200 {
		t.Fatal(status)
	}
	history := profile["history"].([]any)
	if len(history) != 1 {
		t.Fatal("duplicate history")
	}
	record := history[0].(map[string]any)
	rules := record["catanExpansionRules"].(map[string]any)
	if record["catanScenario"] != "transport" || rules["transport"] != r.Game.Catan.Transport.Map.Rules {
		t.Fatal("wrong transport history", record)
	}
	if len(r.Seats) > 4 && rules["transport_deck"] != game.CatanTransportExtendedDeck {
		t.Fatal("lost transport deck recipe")
	}
	if len(r.Seats) == 2 && rules["two_player"] != game.CatanTwoRules {
		t.Fatal("lost two player history")
	}
	stats := profile["stats"].(map[string]any)["catan"].(map[string]any)
	if stats["played"] != float64(1) || stats["wins"] != float64(1) {
		t.Fatal("duplicate result")
	}
}
