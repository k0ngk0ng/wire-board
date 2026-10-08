package server

import (
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanHelpersVariantsPublicToggleRestoreAndBaseReset(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, c := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("助手组合房主")
			c.register("助手组合朋友")
			options := game.CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: true}
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "助手组合", "capacity": n, "catanOptions": options}, 201)
			id := raw["id"].(string)
			c.command(raw, "join", nil, 200)
			for i := 2; i < n; i++ {
				h.command(current(h), "add_bot", nil, 200)
			}
			selectCatanFriendlyRobber(h, true, 200)
			selectCatanHarbors(h, true, 200)
			h.command(current(h), "ready", nil, 200)
			c.command(current(c), "ready", nil, 200)
			before, _ := json.Marshal(s.rooms[id])
			c.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: n > 4}, "version": current(c)["version"], "nonce": randomID(12)}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("guest changed options")
			}
			setOptions := func(o game.CatanOptions) {
				h.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": o, "version": current(h)["version"], "nonce": randomID(12)}, 200)
			}
			setOptions(options)
			for _, seat := range s.rooms[id].Seats {
				if !seat.Ready {
					t.Fatal("same options reset readiness")
				}
			}
			setOptions(game.CatanOptions{FiveSix: n > 4})
			for _, seat := range s.rooms[id].Seats {
				if seat.Ready != seat.Bot {
					t.Fatal("changed options retained readiness")
				}
			}
			setOptions(options)
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "desert", "version": current(h)["version"], "nonce": randomID(12)}, 200)
			h.command(current(h), "ready", nil, 200)
			c.command(current(c), "ready", nil, 200)
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if !g.Options.Helpers || !g.Options.AllHelpers || g.Harbors == nil || g.FriendlyRobber == nil || len(g.Players) != n {
				t.Fatal("combined start")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if !s.rooms[id].CatanOptions.Helpers || !s.rooms[id].CatanHarbors.Enabled || !s.rooms[id].CatanFriendlyRobber.Enabled {
				t.Fatal("rematch lost options")
			}
			selectCatanFriendlyRobber(h, false, 200)
			selectCatanHarbors(h, false, 200)
			h.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "", "version": current(h)["version"], "nonce": randomID(12)}, 200)
			setOptions(game.CatanOptions{FiveSix: n > 4})
			h.command(current(h), "ready", nil, 200)
			c.command(current(c), "ready", nil, 200)
			h.command(current(h), "start", nil, 200)
			g = s.rooms[id].Game.Catan
			if g.Seafarers != nil || g.Harbors != nil || g.FriendlyRobber != nil || g.Options.Helpers || len(g.HelperDisplay) != 0 {
				t.Fatal("base retains extensions")
			}
		})
	}
}
