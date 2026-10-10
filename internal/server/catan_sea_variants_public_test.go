package server

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanSeaVariantsPublicResizeRestoreAndDisable(t *testing.T) {
	for _, friendly := range []bool{false, true} {
		t.Run(fmt.Sprintf("friendly=%v", friendly), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("航海变体房主")
			guest.register("航海变体朋友")
			body := map[string]any{"kind": "catan", "name": "航海变体人数", "capacity": 4, "catanScenario": "shores", "catanHarbors": game.CatanHarborsSetup{Enabled: true}}
			if friendly {
				body["catanFriendlyRobber"] = game.CatanFriendlyRobberSetup{Enabled: true}
			} else {
				body["catanCitiesKnights"] = game.CatanCitiesKnightsSetup{}
			}
			raw := h.post("/api/rooms", body, 201)
			id := raw["id"].(string)
			guest.command(raw, "join", nil, 200)
			change := func(c *testClient, kind, key string, value any, status int) {
				c.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
			}
			ready := func() {
				for _, c := range []*testClient{h, guest} {
					c.command(current(c), "ready", nil, 200)
				}
			}
			ready()
			before, _ := json.Marshal(s.rooms[id])
			change(guest, "catan_options", "catanOptions", game.CatanOptions{FiveSix: true}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("unauthorized resize changed room")
			}
			for _, extended := range []bool{true, false, true} {
				change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: extended}, 200)
				r := s.rooms[id]
				if (r.Capacity == 5) != extended || !r.CatanHarbors.Enabled || r.friendlyRobberEnabled() != friendly || (r.CatanCitiesKnights != nil) == friendly || r.Seats[0].Ready || r.Seats[1].Ready {
					t.Fatal("resize lost variants or kept ready")
				}
				if !r.publicCatanHarborsAvailable() || (friendly && !r.publicCatanFriendlyAvailable()) {
					t.Fatal("public controls disappeared")
				}
				ready()
				change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: extended}, 200)
				if !s.rooms[id].Seats[0].Ready || !s.rooms[id].Seats[1].Ready {
					t.Fatal("same setting cleared ready")
				}
			}
			before, _ = json.Marshal(s.rooms[id])
			// Helpers are supported on the sea maps now; allHelpers alone is not.
			change(h, "catan_options", "catanOptions", game.CatanOptions{FiveSix: true, AllHelpers: true}, 400)
			if friendly {
				change(h, "catan_scenario", "catanScenario", "unknown", 400)
			}
			after, _ = json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("incompatible combination changed room")
			}
			// Events are supported on the sea maps, including the extended seats.
			change(h, "catan_events", "enabled", true, 200)
			ready()
			// A five-seat draft must not start the extended map with only four people.
			for i := 0; i < 2; i++ {
				h.command(current(h), "add_bot", nil, 200)
			}
			h.command(current(h), "start", nil, 400)
			h.command(current(h), "add_bot", nil, 200)
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if len(g.Players) != 5 || g.Paired == nil || g.Harbors == nil || (g.FriendlyRobber != nil) != friendly || (g.CitiesKnights != nil) == friendly || g.Seafarers.NumberRecipe != game.CatanExtendedNumberRecipe {
				t.Fatal("actual game lost expanded recipe")
			}
			selectCatanHarbors(h, false, 400)
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if !s.rooms[id].CatanHarbors.Enabled || s.rooms[id].friendlyRobberEnabled() != friendly {
				t.Fatal("rematch lost variants")
			}
			selectCatanHarbors(h, false, 200)
			if friendly {
				selectCatanFriendlyRobber(h, false, 200)
			}
			change(h, "catan_events", "enabled", false, 200)
			change(h, "catan_scenario", "catanScenario", "", 200)
			ready()
			h.command(current(h), "start", nil, 200)
			g = s.rooms[id].Game.Catan
			if g.Seafarers != nil || g.Harbors != nil || g.FriendlyRobber != nil || g.CitiesKnights != nil || g.EventDeck != nil || len(g.Bank) != 5 {
				t.Fatal("base game retained disabled variants")
			}
		})
	}
}
