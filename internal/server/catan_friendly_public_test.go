package server

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func assertPublicFriendlyProtection(t *testing.T, s *game.State) {
	t.Helper()
	g := s.Catan
	expected := []int{}
	for p, seat := range g.Players {
		hidden := seat.Dev[4]
		// Pirate Islands turns VP cards into knights, so they add no hidden points.
		if g.Seafarers != nil && g.Seafarers.Scenario == "pirate_islands" {
			hidden = 0
		}
		if !seat.Eliminated && seat.Score-hidden < 3 {
			expected = append(expected, p)
		}
	}
	for viewer := -1; viewer < len(g.Players); viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)
		raw, _ := json.Marshal(v["friendlyRobber"])
		var f struct {
			ProtectedPlayers []int `json:"protectedPlayers"`
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(f.ProtectedPlayers, expected) {
			t.Fatal("protection leaks hidden VP or has wrong eligibility", viewer, expected, f.ProtectedPlayers)
		}
	}
}

func TestCatanFriendlyPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, c := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("公开友善房主")
	c.register("公开友善朋友")
	for _, body := range []map[string]any{
		{"kind": "splendor", "capacity": 3}, {"kind": "catan", "capacity": 2},
		{"kind": "catan", "capacity": 5, "catanScenario": "shores", "catanOptions": game.CatanOptions{}},
		{"kind": "catan", "capacity": 3, "catanScenario": "rivers"},
		{"kind": "catan", "capacity": 3, "catanScenario": "cities-knights"},
		{"kind": "catan", "capacity": 3, "catanScenario": "desert", "catanFishing": true},
	} {
		body["name"], body["catanFriendlyRobber"] = "非法组合", game.CatanFriendlyRobberSetup{Enabled: true}
		h.post("/api/rooms", body, 400)
	}
	raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "四人友善新海岸", "capacity": 4, "catanScenario": "shores", "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}, "catanHarbors": game.CatanHarborsSetup{Enabled: true}}, 201)
	id := raw["id"].(string)
	c.command(raw, "join", nil, 200)
	h.command(current(h), "add_bot", nil, 200)
	ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
	ready()
	before, _ := json.Marshal(s.rooms[id])
	selectCatanFriendlyRobber(c, false, 400)
	selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 400)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "unknown", "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid selection changed room")
	}
	selectCatanFriendlyRobber(h, true, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same option cleared ready")
		}
	}
	selectCatanFriendlyRobber(h, false, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("changed option kept ready")
		}
	}
	// Disabled draft does not lock players out of other supported combinations.
	selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 200)
	selectCatanFriendlyRobber(h, true, 400)
	selectCatanCitiesKnights(h, nil, 200)
	selectCatanHarbors(h, false, 200)
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "desert"}, 200)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": true, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	selectCatanFriendlyRobber(h, true, 400)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": false, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	selectCatanFriendlyRobber(h, true, 200)
	selectCatanHarbors(h, true, 200)
	ready()
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
	h.command(current(h), "start", nil, 200)
	if len(s.rooms[id].Game.Catan.Players) != 3 || s.rooms[id].Game.Catan.FriendlyRobber == nil || s.rooms[id].Game.Catan.Harbors == nil {
		t.Fatal("wrong actual public start")
	}
	selectCatanFriendlyRobber(h, false, 400)
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	if !s.rooms[id].CatanFriendlyRobber.Enabled {
		t.Fatal("rematch lost friendly")
	}
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "transport", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	if s.rooms[id].CatanFriendlyRobber != nil || s.rooms[id].CatanHarbors != nil {
		t.Fatal("standalone retained unsupported variant")
	}
}
