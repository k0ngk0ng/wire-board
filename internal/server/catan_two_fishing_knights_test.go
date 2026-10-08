package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanTwoFishingKnightsCompleteHTTPGames(t *testing.T) {
	for _, events := range []bool{false, true} {
		name := "dice"
		if events {
			name = "events"
		}
		t.Run(name, func(t *testing.T) { runTwoVariantsFishingHTTPGames(t, "cities-knights", events, true, true, true) })
	}
}
func TestCatanTwoFishingKnightsConfiguration(t *testing.T) {
	s, ts, clients, id := newTwoVariantsFishingFullTable(t, "cities-knights", true, true, true, true)
	for _, scene := range []string{"cities-knights", "shores", "cities-knights", ""} {
		clients[0].command(current(clients[0]), "close", nil, 200)
		clients[0].command(current(clients[0]), "rematch", nil, 200)
		id = current(clients[0])["id"].(string)
		set := func(c *testClient, enabled bool, status int) {
			c.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": enabled, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
		}
		before, _ := json.Marshal(s.rooms[id])
		set(clients[1], false, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("nonhost mutation")
		}
		clients[0].post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": scene, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
		if scene != "" {
			set(clients[0], false, 200)
			set(clients[0], true, 200)
		} else {
			set(clients[0], true, 400)
		}
		for p := range 2 {
			clients[p].command(current(clients[p]), "ready", nil, 200)
		}
		s, ts = restartRiversHTTP(t, s, ts, clients, id)
		clients[0].command(current(clients[0]), "start", nil, 200)
		g := s.rooms[id].Game.Catan
		if (g.Fishing != nil) != (scene != "") || (g.CitiesKnights != nil) != (scene == "cities-knights") {
			t.Fatal("recipe isolation")
		}
		if scene == "cities-knights" && g.Fishing.TwoKnights != game.CatanTwoFishingKnightsRules {
			t.Fatal("lost combination marker")
		}
		if scene == "" && (len(g.Bank) != 5 || len(g.DevDeck) != 25 || g.Two.Bank != 10) {
			t.Fatal("base economy changed")
		}
	}
}
