package server

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanEventKnightsPublicFullHTTPGames(t *testing.T) {
	t.Run("standalone", func(t *testing.T) {
		testCatanCitiesKnightsEventsFullHTTPGames(t, "", false, false, true, 3, 4, 5, 6)
	})
	for i, scene := range []string{"shores", "islands", "fog", "desert"} {
		t.Run(scene, func(t *testing.T) {
			testCatanCitiesKnightsEventsFullHTTPGames(t, scene, false, false, true, 3+i)
		})
	}
}

func TestCatanEventKnightsPublicToggleRestartAndBase(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("骑士事件房主")
	guest.register("骑士事件客人")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "骑士事件", "capacity": 3, "catanScenario": "cities-knights"}, 201)
	id := raw["id"].(string)
	guest.command(current(host), "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	change := func(c *testClient, kind, key string, value any, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	ready := func() {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
	}
	ready()
	change(guest, "catan_events", "enabled", true, 400)
	change(host, "catan_events", "enabled", true, 200)
	if s.rooms[id].Seats[0].Ready || s.rooms[id].Seats[1].Ready || !s.rooms[id].Seats[2].Ready {
		t.Fatal("event toggle did not reset human ready")
	}
	// Both toggle directions preserve the selected event deck on supported maps.
	change(host, "catan_scenario", "catanScenario", "fog", 200)
	change(host, "catan_cities_knights", "catanCitiesKnights", game.CatanCitiesKnightsSetup{}, 200)
	change(host, "catan_cities_knights", "catanCitiesKnights", nil, 200)
	change(host, "catan_cities_knights", "catanCitiesKnights", game.CatanCitiesKnightsSetup{}, 200)
	if s.rooms[id].CatanEvents != game.CatanEventCatalogue {
		t.Fatal("compatible toggle lost events")
	}

	for _, scene := range []string{"cloth", "wonders", "new_world"} {
		change(host, "catan_scenario", "catanScenario", scene, 200)
		change(host, "catan_cities_knights", "catanCitiesKnights", game.CatanCitiesKnightsSetup{}, 200)
		if s.rooms[id].CatanEvents != game.CatanEventCatalogue {
			t.Fatal("newly supported map lost events", scene)
		}
	}
	change(host, "catan_world_map_shuffle", "catanNewWorldMap", nil, 200)
	expectedMap := *s.rooms[id].CatanNewWorldMap
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	ready()
	host.command(current(host), "start", nil, 200)
	if !slices.Equal(s.rooms[id].Game.Catan.NewWorldMap().Hexes, expectedMap.Hexes) {
		t.Fatal("event game changed approved map")
	}
	if d := s.rooms[id].Game.Catan.EventDeck; d == nil || d.Knights != game.CatanEventKnightsRules {
		t.Fatal("missing live combo")
	}
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if s.rooms[id].CatanEvents != game.CatanEventCatalogue || s.rooms[id].Game != nil {
		t.Fatal("rematch did not preserve draft/reset state")
	}
	change(host, "catan_events", "enabled", false, 200)
	change(host, "catan_scenario", "catanScenario", "", 200)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	ready()
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.EventDeck != nil || g.CardEvent != nil || g.RevealedEvent != nil || g.CitiesKnights != nil || g.Seafarers != nil || len(g.Tiles) != 19 || len(g.Bank) != 5 {
		t.Fatal("base still contains expansion state")
	}
}

func TestCatanEventKnightsAlchemyHTTPRecovery(t *testing.T) {
	s, ts, clients, id := newPublicEventsHTTP(t, 6, "cities-knights", false, false)
	for i := 0; i < 100 && s.rooms[id].Game.Phase != "catan_roll"; i++ {
		state := s.rooms[id].Game
		p := state.Turn
		if actor := state.CatanPendingActor(); actor >= 0 {
			p = actor
		}
		a, err := state.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		clients[p].command(current(clients[p]), "action", a, 200)
	}
	r := s.rooms[id]
	if r.Game.Phase != "catan_roll" {
		t.Fatal("setup did not finish")
	}
	p := r.Game.Turn
	// Explicit mid-game fixture: move one real Alchemy card from its deck to the actor.
	// All actions, response views and restart below use production HTTP/storage.
	k := r.Game.Catan.CitiesKnights
	at := slices.Index(k.ProgressDecks[0], 0)
	if at < 0 {
		t.Fatal("Alchemy unavailable")
	}
	k.ProgressDecks[0] = slices.Delete(k.ProgressDecks[0], at, at+1)
	k.Players[p].Progress = append(k.Players[p].Progress, 0)
	before, _ := json.Marshal(r.Game.Catan.EventDeck.Deck)
	if err := s.save(r); err != nil {
		t.Fatal(err)
	}
	action := game.Action{Type: "catan_progress", Card: 0, Tokens: []int{1, 1}}
	clients[(p+1)%6].command(current(clients[(p+1)%6]), "action", action, 400)
	clients[6].command(current(clients[6]), "action", action, 400)
	clients[p].command(current(clients[p]), "action", action, 200)
	g := s.rooms[id].Game.Catan
	after, _ := json.Marshal(g.EventDeck.Deck)
	if string(before) != string(after) || g.EventDeck.AlchemyRolls != 1 || g.RollID != 1 || g.RevealedEvent != nil {
		t.Fatal("Alchemy consumed card or wrong counter")
	}
	assertPublicEventsPrivacy(t, clients)
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	for _, c := range clients {
		deck := current(c)["game"].(map[string]any)["catan"].(map[string]any)["eventDeck"].(map[string]any)
		if deck["alchemy"] != true || deck["alchemyRolls"] != nil || deck["lastAlchemyRoll"] != nil {
			t.Fatal("Alchemy public view", deck)
		}
	}
	// Six-player secondary action must not draw or roll again.
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_end"}, 200)
	g = s.rooms[id].Game.Catan
	if !g.Paired.Second || g.RollID != 1 {
		t.Fatal("secondary repeated production")
	}
	p = s.rooms[id].Game.Turn
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_roll"}, 400)
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_end"}, 200)
	p = s.rooms[id].Game.Turn
	clients[p].command(current(clients[p]), "action", game.Action{Type: "catan_roll", Card: 999, Color: 999}, 200)
	g = s.rooms[id].Game.Catan
	if g.RollID != 2 || len(g.EventDeck.Deck.Discard) != 1 || g.EventDeck.AlchemyRolls != 1 || g.RevealedEvent == nil {
		t.Fatal("normal draw after Alchemy")
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	assertPublicEventsPrivacy(t, clients)
}

func TestCatanEventKnightsEndgameMapsFullHTTPGames(t *testing.T) {
	for _, scene := range []string{"cloth", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) {
			testCatanCitiesKnightsEventsFullHTTPGames(t, scene, false, false, true, 3, 6)
		})
	}
}
