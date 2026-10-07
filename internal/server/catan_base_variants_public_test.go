package server

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanBaseExtendedVariantsPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	h.register("六人变体房主")
	guest.register("六人变体朋友")
	raw := h.post("/api/rooms", map[string]any{
		"kind": "catan", "name": "基础变体", "capacity": 6,
		"catanOptions":           game.CatanOptions{FiveSix: true},
		"catanBaseConfiguration": game.CatanBaseConfiguration{Layout: "fixed"},
		"catanHarbors":           game.CatanHarborsSetup{Enabled: true},
		"catanFriendlyRobber":    game.CatanFriendlyRobberSetup{Enabled: true},
	}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	selectCatanHarbors(guest, false, 400)
	selectCatanFriendlyRobber(guest, false, 400)
	h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: true, Helpers: true}, "version": s.rooms[id].Version, "nonce": randomID(12)}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejected choice changed room")
	}
	selectCatanHarbors(h, true, 200)
	selectCatanFriendlyRobber(h, true, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same settings cleared readiness")
		}
	}
	selectCatanHarbors(h, false, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready {
			t.Fatal("change retained readiness")
		}
	}
	selectCatanHarbors(h, true, 200)
	selectCatanFriendlyRobber(h, false, 200)
	selectCatanFriendlyRobber(h, true, 200)
	changeCount := func(extended bool, status int) {
		h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: extended}, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	changeCount(false, 200)
	if r := s.rooms[id]; r.Capacity != 4 || r.CatanBaseConfiguration.Layout != "variable" || !r.CatanHarbors.Enabled || !r.CatanFriendlyRobber.Enabled {
		t.Fatal("downgrade lost variants or retained fixed layout")
	}
	changeCount(true, 200)
	if r := s.rooms[id]; r.Capacity != 5 || !r.publicCatanHarborsAvailable() || !r.publicCatanFriendlyAvailable() {
		t.Fatal("upgrade lost variant choices")
	}
	selectCatanBase(h, &game.CatanBaseConfiguration{Layout: "fixed"}, 200)
	for range 3 {
		h.command(current(h), "add_bot", nil, 200)
	}
	changeCount(false, 400)
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
	h.command(current(h), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	h.command(current(h), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if len(g.Players) != 5 || g.BaseSetup.Layout != "fixed" || g.Harbors == nil || g.FriendlyRobber == nil {
		t.Fatal("actual five-player combined start")
	}
	assertPublicHarborPoints(t, s.rooms[id].Game)
	assertPublicFriendlyProtection(t, s.rooms[id].Game)
	selectCatanHarbors(h, false, 400)
	selectCatanFriendlyRobber(h, false, 400)
	h.command(current(h), "close", nil, 200)
	h.command(current(h), "rematch", nil, 200)
	if r := s.rooms[id]; r.CatanBaseConfiguration.Layout != "fixed" || !r.CatanHarbors.Enabled || !r.CatanFriendlyRobber.Enabled {
		t.Fatal("rematch lost configuration")
	}
}
