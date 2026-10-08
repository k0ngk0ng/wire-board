package server

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanSeaExtendedPublicToggleActualPlayersAndRematch(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("扩大航海房主")
	guest.register("扩大航海客人")
	raw := host.post("/api/rooms", map[string]any{"name": "扩大地图配置", "kind": "catan", "capacity": 4, "catanScenario": "shores"}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	change := func(c *testClient, kind, key string, value any, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	ready := func() {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
	}
	ready()
	before, _ := json.Marshal(s.rooms[id])
	change(guest, "catan_options", "catanOptions", game.CatanOptions{FiveSix: true}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("guest changed expansion")
	}
	change(host, "catan_options", "catanOptions", game.CatanOptions{FiveSix: true}, 200)
	r := s.rooms[id]
	if r.Capacity != 5 || r.CatanSeafarers.Layout != "variable" || r.Seats[0].Ready || r.Seats[1].Ready {
		t.Fatal("wrong larger recipe")
	}
	if len(summary(r)["catanSeafarersChoices"].([]game.CatanSeafarersScenario)) != 9 {
		t.Fatal("larger catalogue omitted scenarios")
	}
	ready()
	change(host, "catan_options", "catanOptions", game.CatanOptions{FiveSix: true}, 200)
	if !s.rooms[id].Seats[0].Ready {
		t.Fatal("same options cleared ready")
	}
	selectCatanCitiesKnights(host, &game.CatanCitiesKnightsSetup{}, 200)
	if s.rooms[id].CatanCitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules {
		t.Fatal("wrong paired knights version")
	}
	change(host, "catan_options", "catanOptions", game.CatanOptions{}, 200)
	if s.rooms[id].CatanCitiesKnights.Rules != game.CatanCitiesKnightsRules || s.rooms[id].CatanSeafarers.Layout != "fixed" {
		t.Fatal("shrinking retained large rules")
	}
	change(host, "catan_options", "catanOptions", game.CatanOptions{FiveSix: true}, 200)
	for _, scenario := range []string{"islands", "fog", "desert", "cloth", "wonders", "new_world", "shores"} {
		change(host, "catan_scenario", "catanScenario", scenario, 200)
		r = s.rooms[id]
		if r.CatanCitiesKnights == nil || (r.CatanNewWorldMap != nil) != (scenario == "new_world") {
			t.Fatal("lost knights or stale world")
		}
		if scenario == "new_world" && len(r.CatanNewWorldMap.Hexes) != 63 {
			t.Fatal("wrong world size")
		}
	}
	// Four actual players cannot start a five-to-six-player recipe. Five actual
	// players must receive the five-player game and its matching rules.
	for i := 0; i < 2; i++ {
		host.command(current(host), "add_bot", nil, 200)
	}
	ready()
	host.command(current(host), "start", nil, 400)
	host.command(current(host), "add_bot", nil, 200)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 5 || g.Paired == nil || g.Seafarers.NumberRecipe != game.CatanExtendedNumberRecipe || g.CitiesKnights == nil {
		t.Fatal("actual players or recipe ignored")
	}
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if s.rooms[id].CatanSeafarers.Scenario != "shores" || s.rooms[id].CatanCitiesKnights == nil {
		t.Fatal("rematch lost recipe")
	}
	change(host, "catan_scenario", "catanScenario", "", 200)
	ready()
	host.command(current(host), "start", nil, 200)
	g = s.rooms[id].Game.Catan
	if len(g.Players) != 5 || g.Seafarers != nil || g.CitiesKnights != nil || g.EventDeck != nil || len(g.Bank) != 5 {
		t.Fatal("base inherited sea rules")
	}
}

func TestCatanSeaExtendedFishingToggleKeepsMap(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, scenario := range []string{"fog", "wonders", "new_world"} {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				h := newClient(t, ts.URL)
				h.register("扩大渔夫切换")
				raw := h.post("/api/rooms", map[string]any{"name": "保留海图", "kind": "catan", "capacity": n, "catanScenario": scenario, "catanOptions": game.CatanOptions{FiveSix: true}, "catanFishing": true}, 201)
				id := raw["id"].(string)
				original, _ := json.Marshal(s.rooms[id].CatanNewWorldMap)
				for _, enabled := range []bool{false, true, false} {
					h.post("/api/rooms/"+id, map[string]any{"type": "catan_fishing", "enabled": enabled, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
					r := s.rooms[id]
					after, _ := json.Marshal(r.CatanNewWorldMap)
					if r.CatanFishing != enabled || r.Capacity != n || !r.CatanOptions.FiveSix || r.CatanScenario != scenario || string(after) != string(original) {
						t.Fatal("toggle changed map or count")
					}
				}
				for i := 1; i < n; i++ {
					h.command(current(h), "add_bot", nil, 200)
				}
				h.command(current(h), "ready", nil, 200)
				h.command(current(h), "start", nil, 200)
				if s.rooms[id].Game.Catan.Fishing != nil || s.rooms[id].Game.Catan.Paired == nil {
					t.Fatal("disabled fishing persisted")
				}
			})
		}
	}
}
