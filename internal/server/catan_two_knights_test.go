package server

import (
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanTwoKnightsCompleteHTTPGames(t *testing.T) { runTwoCompleteHTTPGames(t, "cities-knights") }
func TestCatanTwoKnightsEventsCompleteHTTPGames(t *testing.T) {
	runTwoCompleteHTTPGames(t, "cities-knights", true)
}
func TestCatanTwoKnightsPublicRematchAndBaseReset(t *testing.T) {
	s, ts, clients, id := newTwoScenarioFullTable(t, "cities-knights", true)
	for _, scene := range []string{"", "cities-knights"} {
		clients[0].command(current(clients[0]), "close", nil, 200)
		clients[0].command(current(clients[0]), "rematch", nil, 200)
		clients[0].post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": scene, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
		s, ts = restartRiversHTTP(t, s, ts, clients, id)
		for p := range 2 {
			clients[p].command(current(clients[p]), "ready", nil, 200)
		}
		clients[0].command(current(clients[0]), "start", nil, 200)
		g := s.rooms[id].Game.Catan
		if (g.CitiesKnights != nil) != (scene == "cities-knights") || g.EventDeck == nil {
			t.Fatal("rematch isolation")
		}
		if scene == "" && (g.Two.Knights != "" || len(g.Bank) != 5 || len(g.DevDeck) != 25) {
			t.Fatal("base city state leak")
		}
		if scene == "cities-knights" && (g.Two.Knights != game.CatanTwoKnightsRules || len(g.Bank) != 8 || len(g.DevDeck) != 0) {
			t.Fatal("city recipe missing")
		}
	}
}
