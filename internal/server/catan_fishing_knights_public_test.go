package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanFishingKnightsPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("捕鱼骑士房主")
	guest.register("捕鱼骑士朋友")
	city := game.CatanCitiesKnightsSetup{}
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2},
		{"kind": "catan", "capacity": 5},
		{"kind": "catan", "capacity": 3, "catanOptions": game.CatanOptions{Helpers: true}},
		{"kind": "splendor", "capacity": 3},
	} {
		body["name"], body["catanScenario"], body["catanCitiesKnights"] = "无效捕鱼组合", "fishing", city
		host.post("/api/rooms", body, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"name": "捕鱼城市骑士", "kind": "catan", "capacity": 4, "catanScenario": "fishing", "catanCitiesKnights": city}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	ready := func() {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
	}
	choose := func(scene string) {
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": scene, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	}
	ready()
	before, _ := json.Marshal(s.rooms[id])
	selectCatanCitiesKnights(guest, nil, 400)
	selectCatanCitiesKnights(host, &game.CatanCitiesKnightsSetup{Layout: "fixed"}, 400)
	selectCatanCitiesKnights(host, &game.CatanCitiesKnightsSetup{Rules: game.CatanCitiesKnightsFiveSixRules}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejection mutated room")
	}
	selectCatanCitiesKnights(host, &city, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same combo reset ready")
		}
	}
	selectCatanCitiesKnights(host, nil, 200)
	if s.rooms[id].CatanCitiesKnights != nil || s.rooms[id].CatanScenario != "fishing" {
		t.Fatal("disable lost fishing")
	}
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("change retained ready")
		}
	}
	for _, scene := range []string{"shores", "cities-knights", "transport", "fishing"} {
		choose(scene)
		if scene != "cities-knights" && s.rooms[id].CatanCitiesKnights != nil {
			t.Fatal("old combination survived switch")
		}
	}
	selectCatanCitiesKnights(host, &city, 200)
	ready()
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 3 || g.Fishing == nil || g.CitiesKnights == nil || len(g.Bank) != 8 || len(g.DevDeck) != 0 || g.Paired != nil {
		t.Fatal("wrong actual fishing city start")
	}
	selectCatanCitiesKnights(host, nil, 400)
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if s.rooms[id].CatanScenario != "fishing" || s.rooms[id].CatanCitiesKnights == nil {
		t.Fatal("rematch lost combination")
	}
	selectCatanCitiesKnights(host, nil, 200)
	ready()
	host.command(current(host), "start", nil, 200)
	g = s.rooms[id].Game.Catan
	if g.Fishing == nil || g.CitiesKnights != nil || len(g.Bank) != 5 || len(g.DevDeck) != 25 {
		t.Fatal("disabled knights still active")
	}
}

func assertFishingCityInventory(t *testing.T, g *game.Catan) {
	t.Helper()
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
			t.Fatal("combined resource inventory", color, total)
		}
	}
	f := g.Fishing.Tokens
	count := 30
	if len(g.Players) > 4 {
		count = 44
	}
	seen := make([]bool, count)
	if f.BootOwner >= 0 {
		seen[29] = true
	}
	check := func(ids []int) {
		for _, id := range ids {
			if id < 0 || id >= len(seen) || seen[id] {
				t.Fatal("fish identity", id)
			}
			seen[id] = true
		}
	}
	check(f.DrawPile)
	check(f.Discard)
	for _, hand := range f.Hands {
		if len(hand) > 7 {
			t.Fatal("fish hand cap")
		}
		check(hand)
	}
	for _, present := range seen {
		if !present {
			t.Fatal("lost fish")
		}
	}
	cards := 0
	for _, deck := range g.CitiesKnights.ProgressDecks {
		cards += len(deck)
	}
	for _, p := range g.CitiesKnights.Players {
		cards += len(p.Progress) + len(p.PublicProgress)
	}
	if cards != 54 || len(g.DevDeck) != 0 {
		t.Fatal("progress inventory", cards)
	}
	occupied := map[int]bool{}
	stock := map[[2]int]int{}
	for _, k := range g.CitiesKnights.Knights {
		if k.Strength < 1 || k.Strength > 3 || occupied[k.Vertex] || g.Vertices[k.Vertex].Level > 0 {
			t.Fatal("knight placement")
		}
		occupied[k.Vertex] = true
		key := [2]int{k.Owner, k.Strength}
		stock[key]++
		if stock[key] > 2 {
			t.Fatal("knight inventory")
		}
	}
}

func TestCatanFishingKnightsExtendedToggleAndBaseReset(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h := newClient(t, ts.URL)
	h.register("扩大捕鱼骑士")
	raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "捕鱼骑士人数切换", "capacity": 4, "catanScenario": "fishing", "catanCitiesKnights": game.CatanCitiesKnightsSetup{}}, 201)
	id := raw["id"].(string)
	options := func(extended bool, status int) {
		h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: extended}, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	for _, extended := range []bool{true, false, true} {
		h.command(current(h), "ready", nil, 200)
		options(extended, 200)
		want := game.CatanCitiesKnightsRules
		if extended {
			want = game.CatanCitiesKnightsFiveSixRules
		}
		r := s.rooms[id]
		if r.CatanCitiesKnights == nil || r.CatanCitiesKnights.Rules != want || r.Seats[0].Ready {
			t.Fatal("stale rules or readiness")
		}
	}
	selectCatanCitiesKnights(h, nil, 200)
	if s.rooms[id].CatanScenario != "fishing" || !s.rooms[id].CatanOptions.FiveSix {
		t.Fatal("knights toggle lost fishing extension")
	}
	selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 200)
	for range 4 {
		h.command(current(h), "add_bot", nil, 200)
	}
	options(false, 400)
	h.command(current(h), "ready", nil, 200)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h}, id)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 5 || g.CitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules || g.Fishing.Map.NumberRecipe != game.CatanExtendedNumberRecipe || g.Paired == nil {
		t.Fatal("wrong actual combination")
	}
	assertFishingCityInventory(t, g)
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	if s.rooms[id].CatanCitiesKnights == nil || s.rooms[id].CatanScenario != "fishing" {
		t.Fatal("lost rematch recipe")
	}
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	h.command(current(h), "ready", nil, 200)
	h.command(current(h), "start", nil, 200)
	g = s.rooms[id].Game.Catan
	if g.Fishing != nil || g.CitiesKnights != nil || g.Paired == nil || len(g.DevDeck) != 34 || len(g.Bank) != 5 {
		t.Fatal("base retained combination")
	}
}
