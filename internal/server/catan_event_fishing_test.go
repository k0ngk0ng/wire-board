package server

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanEventFishingNaturalHTTP(t *testing.T) {
	t.Run("standalone", func(t *testing.T) {
		testFishingEventsFullHTTP(t, "", false, false, true, 3, 4, 5, 6)
	})
	for _, scene := range []string{"islands", "fog", "desert", "cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) {
			n := 6
			if !publicCatanFishingSeaExtended(scene) {
				n = 4
			}
			testFishingEventsFullHTTP(t, scene, true, true, true, 3, n)
		})
	}
}

func TestCatanEventFishingKnightsNaturalHTTP(t *testing.T) {
	testCatanCitiesKnightsEventsFullHTTPGames(t, "fishing", true, true, true, 3, 4, 5, 6)
}

func TestCatanEventFishingTribeNaturalHTTP(t *testing.T) {
	testFishingEventsFullHTTP(t, "tribe", true, true, true, 3, 4)
}

func TestCatanEventFishingToggleRestoreBase(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, c := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("捕鱼事件房主")
			c.register("捕鱼事件朋友")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "捕鱼事件切换", "capacity": n, "catanScenario": "fishing", "catanOptions": game.CatanOptions{FiveSix: n > 4}, "catanEvents": game.CatanEventCatalogue}, 201)
			id := raw["id"].(string)
			c.command(raw, "join", nil, 200)
			for i := 2; i < n; i++ {
				h.command(current(h), "add_bot", nil, 200)
			}
			change := func(client *testClient, kind, key string, value any, status int) {
				client.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
			ready()
			before, _ := json.Marshal(s.rooms[id])
			change(c, "catan_events", "enabled", false, 400)
			selectCatanCitiesKnights(c, &game.CatanCitiesKnightsSetup{}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("guest changed draft")
			}
			selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 200)
			for _, seat := range s.rooms[id].Seats {
				if seat.Ready != seat.Bot {
					t.Fatal("new combo retained ready")
				}
			}
			if s.rooms[id].CatanEvents == "" {
				t.Fatal("adding cities cleared events")
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
			ready()
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if g.Fishing == nil || g.EventDeck == nil || g.CitiesKnights == nil {
				t.Fatal("wrong saved fishing city event recipe")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			selectCatanCitiesKnights(h, nil, 200)
			change(h, "catan_scenario", "catanScenario", "new_world", 200)
			change(h, "catan_fishing", "enabled", true, 200)
			if s.rooms[id].CatanEvents == "" {
				t.Fatal("sea fish toggle lost event deck")
			}
			ready()
			change(h, "catan_fishing", "enabled", true, 200)
			for _, seat := range s.rooms[id].Seats {
				if !seat.Ready {
					t.Fatal("no-op toggle cleared readiness")
				}
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
			h.command(current(h), "start", nil, 200)
			g = s.rooms[id].Game.Catan
			if g.Fishing == nil || g.EventDeck == nil || g.Seafarers == nil || g.Seafarers.NewWorld == nil {
				t.Fatal("sea fish event start")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			change(h, "catan_events", "enabled", false, 200)
			change(h, "catan_fishing", "enabled", false, 200)
			change(h, "catan_scenario", "catanScenario", "", 200)
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
			ready()
			h.command(current(h), "start", nil, 200)
			g = s.rooms[id].Game.Catan
			if g.Fishing != nil || g.EventDeck != nil || g.CitiesKnights != nil || g.Seafarers != nil {
				t.Fatal("base game inherited extensions")
			}
		})
	}
}
