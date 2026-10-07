package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"testing"
)

func TestCatanFriendlySeaConfigurationCountsReadinessRestartHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("友善航海房主")
	guest.register("友善航海客人")
	raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "友善航海", "capacity": 4}, 201)
	id := raw["id"].(string)
	guest.command(current(h), "join", nil, 200)
	h.command(current(h), "add_bot", nil, 200)
	provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: "shores"})
	provisionCatanFriendlyRobber(t, s, id)
	v := current(h)
	availability := v["catanFriendlyRobberAvailability"].(map[string]any)
	if availability["allowed"] != true || availability["minPlayers"] != float64(4) {
		t.Fatal("count guidance", availability)
	}
	for _, raw := range v["catanSeafarersChoices"].([]any) {
		info := raw.(map[string]any)
		if !game.CatanFriendlySeafarersSupported(4, info["id"].(string)) {
			t.Fatal("unsupported choice exposed")
		}
	}
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	h.command(current(h), "start", nil, 400) // Capacity 4 cannot disguise actual 3-player recipe.
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "fog"}, 400)
	selectCatanFriendlyRobber(guest, false, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejected settings changed readiness or draft")
	}
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "shores"}, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same setup reset readiness")
		}
	}
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "desert"}, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("new map did not reset readiness")
		}
	}
	selectCatanFriendlyRobber(h, false, 200)
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "fog"}, 200)
	if current(h)["catanFriendlyRobberAvailability"].(map[string]any)["allowed"] != false {
		t.Fatal("unsupported map not explained")
	}
	selectCatanFriendlyRobber(h, true, 400)
	selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "shores"}, 200)
	selectCatanFriendlyRobber(h, true, 200)
	selectCatanFriendlyRobber(h, false, 200)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: true}, "version": current(h)["version"], "nonce": randomID(12)}, 200)
	if s.rooms[id].Capacity != 5 || s.rooms[id].CatanSeafarers.Layout != "variable" || current(h)["catanFriendlyRobberAvailability"].(map[string]any)["minPlayers"] != float64(5) {
		t.Fatal("5/6 recipe switch")
	}
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{}, "version": current(h)["version"], "nonce": randomID(12)}, 200)
	if s.rooms[id].Capacity != 4 || s.rooms[id].CatanSeafarers.Layout != "fixed" {
		t.Fatal("return to four-player recipe")
	}
	selectCatanFriendlyRobber(h, true, 200)
	h.command(current(h), "add_bot", nil, 200)
	provisionCatanHarbors(t, s, id)
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ = json.Marshal(s.rooms[id])
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	after, _ = json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("combined waiting draft lost on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	h.base, guest.base = ts2.URL, ts2.URL
	h.command(current(h), "start", nil, 200)
	r := next.rooms[id]
	if r.Game.Catan.FriendlyRobber == nil || r.Game.Catan.Harbors == nil || r.Game.Catan.Seafarers.Scenario != "shores" || r.Game.View(0)["catan"].(map[string]any)["victoryTarget"] != 15 {
		t.Fatal("formal combined start")
	}
	selectCatanFriendlyRobber(h, false, 400)
	r.CatanFriendlyRobber = nil
	r.CatanHarbors = nil
	r.CatanSeafarers = nil
	h.command(current(h), "close", nil, 200)
	_, profile := h.request("GET", "/api/players/"+r.Host, nil)
	record := profile["history"].([]any)[0].(map[string]any)
	rules := record["catanExpansionRules"].(map[string]any)
	if rules["friendly_robber"] != game.CatanFriendlyRobberRules || rules["harbors"] != game.CatanHarborsRules || rules["seafarers"] != game.CatanSeafarersRules || record["catanScenario"] != "shores" {
		t.Fatal("combined history uses draft", record)
	}
}

func TestCatanFriendlySeaThreePlayerCapacityCannotSelectShores(t *testing.T) {
	r := &Room{Kind: "catan", Status: "waiting", Capacity: 3}
	if err := r.setCatanFriendlyRobber(game.CatanFriendlyRobberSetup{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.setCatanSeafarers(game.CatanSeafarersSetup{Scenario: "shores"}); err == nil {
		t.Fatal("3-player shores accepted")
	}
	if err := r.setCatanSeafarers(game.CatanSeafarersSetup{Scenario: "desert"}); err != nil {
		t.Fatal(err)
	}
	for _, info := range summary(r)["catanSeafarersChoices"].([]game.CatanSeafarersScenario) {
		if info.ID == "shores" {
			t.Fatal("3-player shores in choices")
		}
	}
}
