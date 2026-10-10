package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"slices"
	"testing"
)

func newPublicFishingSeaTable(t *testing.T, n int, scenario, layout string) (*Server, *httptest.Server, []*testClient, string) {
	return newPublicFishingSeaVariants(t, n, scenario, layout, false, false)
}

func newPublicFishingSeaVariants(t *testing.T, n int, scenario, layout string, friendly, harbors bool, events ...bool) (*Server, *httptest.Server, []*testClient, string) {
	return newPublicFishingSeaConfigured(t, n, scenario, layout, friendly, harbors, len(events) > 0 && events[0], false)
}

func newPublicFishingSeaConfigured(t *testing.T, n int, scenario, layout string, friendly, harbors bool, events, helpers bool) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("海图捕鱼玩家%d", p))
	}
	body := map[string]any{"name": "捕鱼海图完整局", "kind": "catan", "capacity": n, "catanScenario": scenario, "catanFishing": true, "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}}
	if events {
		body["catanEvents"] = game.CatanEventCatalogue
	}
	raw := clients[0].post("/api/rooms", body, 201)
	id := raw["id"].(string)
	if friendly {
		selectCatanFriendlyRobber(clients[0], true, 200)
	}
	if harbors {
		selectCatanHarbors(clients[0], true, 200)
	}
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	selectSeafarers(clients[0], &game.CatanSeafarersSetup{Scenario: scenario, Layout: layout}, 200)
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[0]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	wantScenario := scenario
	if n > 4 && scenario == "islands" {
		wantScenario = "six_islands"
	}
	if g.Fishing == nil || g.Seafarers == nil || g.Seafarers.Scenario != wantScenario || g.Seafarers.Layout != layout || g.CitiesKnights != nil || !s.rooms[id].CatanFishing {
		t.Fatal("wrong public fish sea opening")
	}
	if (g.Paired != nil) != (n > 4) || g.Options.FiveSix != (n > 4) {
		t.Fatal("wrong public fish sea player-count rules")
	}
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, clients, id
}

func TestCatanFishingSeaPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, c := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("海图捕鱼房主")
	c.register("海图捕鱼朋友")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2, "catanScenario": "shores"},
		{"kind": "catan", "capacity": 4, "catanScenario": "pirate_islands"},
		{"kind": "catan", "capacity": 7, "catanScenario": "land-ho"},
		{"kind": "catan", "capacity": 3, "catanScenario": "fog", "catanOptions": game.CatanOptions{AllHelpers: true}, "catanCitiesKnights": game.CatanCitiesKnightsSetup{}},
		{"kind": "catan", "capacity": 4, "catanScenario": "islands", "catanOptions": game.CatanOptions{AllHelpers: true}},
		{"kind": "catan", "capacity": 5, "catanScenario": "cloth", "catanOptions": game.CatanOptions{}},
		{"kind": "splendor", "capacity": 3},
	} {
		body["name"], body["catanFishing"] = "错误捕鱼组合", true
		h.post("/api/rooms", body, 400)
	}
	raw := h.post("/api/rooms", map[string]any{"name": "捕鱼海图设置", "kind": "catan", "capacity": 4, "catanScenario": "islands", "catanFishing": true}, 201)
	id := raw["id"].(string)
	c.command(raw, "join", nil, 200)
	h.command(current(h), "add_bot", nil, 200)
	toggle := func(client *testClient, on bool, status int) {
		client.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": on, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
	ready()
	before, _ := json.Marshal(s.rooms[id])
	toggle(c, false, 400)
	selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{Layout: "fixed"}, 400)
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "desert", Layout: "variable"}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid combo mutated room")
	}
	toggle(h, true, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same choice reset ready")
		}
	}
	toggle(h, false, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("changed combo retained ready")
		}
	}
	toggle(h, true, 200)
	for _, scene := range []string{"fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		ready()
		selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: scene}, 200)
		if !s.rooms[id].CatanFishing || s.rooms[id].CatanScenario != scene {
			t.Fatal("switch lost combination")
		}
		for _, seat := range s.rooms[id].Seats {
			if seat.Ready != seat.Bot {
				t.Fatal("map change kept ready")
			}
		}
	}
	ready()
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 3 || g.Fishing == nil || g.Fishing.WorldSetup == nil {
		t.Fatal("actual new world start")
	}
	toggle(h, false, 400)
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	if !s.rooms[id].CatanFishing || s.rooms[id].CatanNewWorldMap == nil {
		t.Fatal("rematch lost combination")
	}
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "rivers", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	// Fishing is a supported nesting of the standalone river game, so only the
	// sea map has to be dropped when the scenario changes.
	if !s.rooms[id].CatanFishing || s.rooms[id].CatanSeafarers != nil {
		t.Fatal("river switch lost fishing or kept the sea map")
	}
}

func assertPublicFishSeaSpecialInventory(t *testing.T, g *game.Catan) {
	t.Helper()
	if g.Seafarers.Cloth != nil {
		assertPublicClothSupply(t, g)
	}
	if tr := g.Seafarers.Tribe; tr != nil {
		total := len(tr.Tokens)
		for _, n := range tr.Points {
			total += n
		}
		tokens := 8
		if len(g.Players) > 4 {
			tokens = 10
		}
		if total != tokens {
			t.Fatal("tribe points")
		}
		ports := map[int]int{}
		for _, p := range append(slices.Clone(g.Ports), tr.Ports...) {
			ports[p.Resource]++
		}
		for _, hand := range tr.HeldPorts {
			for _, color := range hand {
				ports[color]++
			}
		}
		for color := -1; color < 5; color++ {
			expected := 1
			if color == -1 && len(g.Players) > 4 {
				expected = 3
			}
			if ports[color] != expected {
				t.Fatal("tribe ports", ports)
			}
		}
	}
}

func publicFishingClothWon(t *testing.T, s *game.State) bool {
	t.Helper()
	g := s.Catan
	assertPublicFishSeaSpecialInventory(t, g)
	target := 14
	if g.Fishing.Tokens.BootOwner == s.Turn {
		target++
	}
	// The point victory belongs to the active player against their own goal.
	// A boot handoff can leave the last actor one point short of theirs, so the
	// dried-up villages then decide by score and cloth instead.
	if g.Players[s.Turn].Score >= target && slices.Equal(s.Winners, []int{s.Turn}) {
		return true
	}
	empty := 0
	for _, v := range g.Seafarers.Cloth.Villages {
		if v.Stock == 0 {
			empty++
		}
	}
	if empty < 5 {
		return false
	}
	best, cloth := -1, -1
	winners := []int{}
	for p, seat := range g.Players {
		if seat.Eliminated {
			continue
		}
		n := g.Seafarers.Cloth.Held[p]
		if seat.Score > best || seat.Score == best && n > cloth {
			best, cloth, winners = seat.Score, n, []int{p}
		} else if seat.Score == best && n == cloth {
			winners = append(winners, p)
		}
	}
	return slices.Equal(s.Winners, winners)
}

// A village-exhaustion finish compares total score and cloth once the last
// villages run dry, so the active player's own threshold must not decide the
// result: after a boot handoff the last actor can be one point short of their
// own goal while a same-score opponent holds more cloth and wins instead.
func TestPublicClothExhaustionTiebreakAfterBootHandoff(t *testing.T) {
	s, _, _, id := newPublicFishingSeaConfigured(t, 4, "cloth", "fixed", false, false, false, false)
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.rooms[id]
	g := r.Game.Catan
	c := g.Seafarers.Cloth
	for i := range c.Villages {
		c.Villages[i].Stock = 0
	}
	c.Stock, c.Held = 10, []int{10, 15, 15, 0}
	g.Players[0].Score, g.Players[1].Score, g.Players[2].Score, g.Players[3].Score = 14, 14, 13, 12
	r.Game.Turn, r.Game.Winners, r.Game.Finished, r.Game.Phase = 0, []int{1}, true, "finished"

	// The last actor handed the boot away, so their own threshold is fourteen
	// and the exhausted villages decide by score and cloth.
	g.Fishing.Tokens.BootOwner = 1
	if !publicFishingClothWon(t, r.Game) {
		t.Fatal("village exhaustion must decide by score and cloth")
	}
	assertPublicSeaVictory(t, r.Game, 1)

	// Holding the boot raises the last actor's own goal to fifteen, which the
	// same exhausted table still resolves through the cloth comparison.
	g.Fishing.Tokens.BootOwner = 0
	if !publicFishingClothWon(t, r.Game) {
		t.Fatal("boot holder at fourteen has not reached the point victory")
	}
	assertPublicSeaVictory(t, r.Game, 1)
}

func TestCatanFishingSeaPublicOpeningMatrix(t *testing.T) {
	for _, scene := range []string{"islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		layouts := []string{"fixed", "variable"}
		if scene == "desert" || scene == "tribe" {
			layouts = layouts[:1]
		}
		if scene == "new_world" {
			layouts = []string{"prepared"}
		}
		for _, n := range []int{3, 4} {
			for _, layout := range layouts {
				t.Run(fmt.Sprintf("%s/%d/%s", scene, n, layout), func(t *testing.T) {
					s, _, clients, id := newPublicFishingSeaTable(t, n, scene, layout)
					if current(clients[0])["catanFishing"] != true || s.rooms[id].Game.Catan.Fishing == nil {
						t.Fatal("public choice omitted")
					}
				})
			}
		}
	}
}
