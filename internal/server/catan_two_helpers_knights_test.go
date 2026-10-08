package server

import (
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanTwoHelpersKnightsNaturalHTTP(t *testing.T) {
	for _, fish := range []bool{false, true} {
		t.Run(fmt.Sprint("fish", fish), func(t *testing.T) {
			runTwoVariantsFishingHTTPGames(t, "cities-knights", fish, true, true, fish, game.CatanOptions{Helpers: true, AllHelpers: true})
		})
	}
}
func TestCatanTwoSeaHelpersKnightsNaturalHTTP(t *testing.T) {
	for _, fish := range []bool{false, true} {
		t.Run(fmt.Sprint("fish", fish), func(t *testing.T) { runTwoSeafarersCompleteHTTPGames(t, fish, true, true) })
	}
}
func TestCatanTwoHelpersKnightsPublicConfiguration(t *testing.T) {
	for _, scene := range []string{"cities-knights", "shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		for _, fish := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/fish%t", scene, fish), func(t *testing.T) {
				s, ts, clients, id := newTwoCombinationFullTable(t, scene, true, true, true, fish, scene != "cities-knights", game.CatanOptions{Helpers: true, AllHelpers: true})
				g := s.rooms[id].Game.Catan
				if g.Two.Helpers != game.CatanTwoHelpersRules || g.CitiesKnights.Helpers == nil || fish && g.Fishing.Helpers != game.CatanFishingHelpersRules {
					t.Fatal("missing markers")
				}
				h := clients[0]
				h.command(current(h), "close", nil, 200)
				h.command(current(h), "rematch", nil, 200)
				id = current(h)["id"].(string)
				for _, on := range []bool{false, true, false} {
					body := map[string]any{"type": "catan_options", "version": s.rooms[id].Version, "nonce": randomID(12), "catanOptions": game.CatanOptions{Helpers: on, AllHelpers: on}}
					clients[1].post("/api/rooms/"+id, body, 400)
					h.post("/api/rooms/"+id, body, 200)
				}
				s, ts = restartRiversHTTP(t, s, ts, clients, id)
				for p := range 2 {
					clients[p].command(current(clients[p]), "ready", nil, 200)
				}
				h.command(current(h), "start", nil, 200)
				g = s.rooms[id].Game.Catan
				if g.Options.Helpers || g.Two.Helpers != "" || g.CitiesKnights.Helpers != nil || g.HelperPending != nil {
					t.Fatal("disabled helpers persisted")
				}
				h.command(current(h), "close", nil, 200)
				h.command(current(h), "rematch", nil, 200)
				id = current(h)["id"].(string)
				h.post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": "", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
				for p := range 2 {
					clients[p].command(current(clients[p]), "ready", nil, 200)
				}
				h.command(current(h), "start", nil, 200)
				g = s.rooms[id].Game.Catan
				if g.CitiesKnights != nil || g.Fishing != nil || g.Options.Helpers {
					t.Fatal("base isolation")
				}
			})
		}
	}
}

func TestCatanTwoHelpersKnightsProgressClock(t *testing.T) {
	for _, fish := range []bool{false, true} {
		t.Run(fmt.Sprint(fish), func(t *testing.T) {
			s, ts, clients, id := newTwoCombinationFullTable(t, "cities-knights", false, false, false, fish, false, game.CatanOptions{Helpers: true, AllHelpers: true})
			testHelpersKnightsHTTPProgressClock(t, s, ts, clients, id)
		})
	}
}
