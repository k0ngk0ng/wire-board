package server

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanOuterSeaEventsToggleRestoreBase(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, c := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("外海事件房主")
	c.register("外海事件玩家")
	r := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "外海事件配置", "capacity": 3, "catanScenario": "tribe", "catanEvents": game.CatanEventCatalogue}, 201)
	id := r["id"].(string)
	c.command(r, "join", nil, 200)
	h.command(current(h), "add_bot", nil, 200)
	change := func(client *testClient, kind, key string, value any, status int) {
		client.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	ready := func() { h.command(current(h), "ready", nil, 200); c.command(current(c), "ready", nil, 200) }
	for _, scene := range []string{"tribe", "pirate_islands"} {
		change(h, "catan_scenario", "catanScenario", scene, 200)
		ready()
		before, _ := json.Marshal(s.rooms[id])
		change(c, "catan_events", "enabled", false, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("guest mutated draft")
		}
		s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
		h.command(current(h), "start", nil, 200)
		g := s.rooms[id].Game.Catan
		if g.EventDeck == nil || g.Seafarers.Scenario != scene || scene == "pirate_islands" && g.EventDeck.FleetRules != game.CatanEventFleetRules {
			t.Fatal("lost saved recipe")
		}
		h.command(current(h), "close", nil, 200)
		h.command(current(h), "rematch", nil, 200)
		if s.rooms[id].CatanEvents != game.CatanEventCatalogue {
			t.Fatal("rematch lost events")
		}
	}
	change(h, "catan_events", "enabled", false, 200)
	change(h, "catan_scenario", "catanScenario", "", 200)
	ready()
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.EventDeck != nil || g.Seafarers != nil || g.RevealedEvent != nil {
		t.Fatal("ordinary base inherited sea events")
	}
}
