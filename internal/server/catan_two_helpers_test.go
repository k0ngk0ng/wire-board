package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanTwoHelpersNaturalHTTP(t *testing.T) {
	for _, scenario := range []string{"", "fishing"} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/events=%t", scenario, events), func(t *testing.T) {
				runTwoVariantsHTTPGames(t, scenario, events, true, true, game.CatanOptions{Helpers: true, AllHelpers: true})
			})
		}
	}
}
func TestCatanTwoHelpersPublicSelection(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("双人助手房主")
	guest.register("双人助手客人")
	r := h.post("/api/rooms", map[string]any{"kind": "catan", "capacity": 2, "name": "双人助手切换", "catanOptions": game.CatanOptions{Helpers: true}}, 201)
	id := r["id"].(string)
	guest.command(current(h), "join", nil, 200)
	options := func(c *testClient, o game.CatanOptions, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": o, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	for _, o := range []game.CatanOptions{{Helpers: true, AllHelpers: true}, {}, {Helpers: true}} {
		h.command(current(h), "ready", nil, 200)
		guest.command(current(guest), "ready", nil, 200)
		options(guest, o, 400)
		options(h, o, 200)
		if s.rooms[id].Capacity != 2 {
			t.Fatal("helpers changed capacity")
		}
		for _, p := range s.rooms[id].Seats {
			if p.Ready {
				t.Fatal("changed rules retained readiness")
			}
		}
	}
	before, _ := json.Marshal(s.rooms[id])
	options(h, game.CatanOptions{Helpers: true, FiveSix: true}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid options mutated room")
	}
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": "rivers", "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": "fishing", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
	h.command(current(h), "start", nil, 200)
	if s.rooms[id].Game.Catan.Two.Helpers != game.CatanTwoHelpersRules || s.rooms[id].Game.Catan.Fishing.Helpers != game.CatanFishingHelpersRules {
		t.Fatal("start missing recipe")
	}
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	id = current(h)["id"].(string)
	if !s.rooms[id].CatanOptions.Helpers {
		t.Fatal("rematch lost helpers")
	}
	options(h, game.CatanOptions{}, 200)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": "", "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
	guest.command(current(guest), "ready", nil, 200)
	h.command(current(h), "ready", nil, 200)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.Options.Helpers || g.Two.Helpers != "" || len(g.HelperDisplay) > 0 || g.Fishing != nil {
		t.Fatal("return to base leaked expansion")
	}
}
