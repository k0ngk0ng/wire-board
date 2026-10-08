package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanLandExtendedPublicConfiguration(t *testing.T) {
	for _, scene := range []string{"rivers", "caravans"} {
		t.Run(scene, func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, guest := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("扩大剧本房主")
			guest.register("扩大剧本朋友")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "河流商队五六人", "capacity": 6, "catanScenario": scene, "catanOptions": game.CatanOptions{FiveSix: true}}, 201)
			id := raw["id"].(string)
			guest.command(raw, "join", nil, 200)
			command := func(c *testClient, kind string, fields map[string]any, status int) {
				fields["type"] = kind
				fields["version"] = s.rooms[id].Version
				fields["nonce"] = randomID(12)
				c.post("/api/rooms/"+id, fields, status)
			}
			options := func(o game.CatanOptions, status int) {
				command(h, "catan_options", map[string]any{"catanOptions": o}, status)
			}
			change := func(scene string, status int) {
				command(h, "catan_scenario", map[string]any{"catanScenario": scene}, status)
			}
			h.command(current(h), "ready", nil, 200)
			guest.command(current(guest), "ready", nil, 200)
			before, _ := json.Marshal(s.rooms[id])
			command(guest, "catan_scenario", map[string]any{"catanScenario": ""}, 400)
			options(game.CatanOptions{FiveSix: true, Helpers: true}, 400)
			selectCatanBase(h, &game.CatanBaseConfiguration{Layout: "fixed"}, 400)
			selectCatanHarbors(h, true, 400)
			selectCatanFriendlyRobber(h, true, 400)
			selectCatanCitiesKnights(h, &game.CatanCitiesKnightsSetup{}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("rejected change mutated room")
			}
			change(scene, 200)
			for _, seat := range s.rooms[id].Seats {
				if !seat.Ready {
					t.Fatal("same scene cleared ready")
				}
			}
			options(game.CatanOptions{}, 200)
			if r := s.rooms[id]; r.Capacity != 4 || r.CatanScenario != scene || r.CatanOptions.FiveSix {
				t.Fatal("incorrect downgrade")
			}
			for _, seat := range s.rooms[id].Seats {
				if seat.Ready {
					t.Fatal("changed recipe retained ready")
				}
			}
			options(game.CatanOptions{FiveSix: true}, 200)
			if s.rooms[id].Capacity != 5 {
				t.Fatal("incorrect upgrade")
			}
			change("", 200)
			selectCatanBase(h, &game.CatanBaseConfiguration{Layout: "fixed"}, 200)
			change(scene, 200)
			if s.rooms[id].CatanBaseConfiguration != nil {
				t.Fatal("base layout remained")
			}
			h.command(current(h), "ready", nil, 200)
			guest.command(current(guest), "ready", nil, 200)
			h.command(current(h), "start", nil, 400)
			for range 3 {
				h.command(current(h), "add_bot", nil, 200)
			}
			options(game.CatanOptions{}, 400)
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, guest}, id)
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if len(g.Players) != 5 || len(g.Tiles) != 30 || g.Paired == nil || g.Options.Helpers || len(g.DevDeck) != 34 {
				t.Fatal("wrong actual-player opening")
			}
			if scene == "rivers" {
				if g.Rivers == nil || g.Rivers.Map.NumberRecipe != game.CatanExtendedNumberRecipe || g.Rivers.Bank != 152 || len(g.Rivers.Map.Channels) != 3 || g.Rivers.Map.DoubleNumberTile != -1 {
					t.Fatal("wrong river inventory")
				}
			} else {
				if g.Caravans == nil || g.Caravans.Map.NumberRecipe != game.CatanExtendedNumberRecipe || g.Caravans.Map.Supply != 33 || len(g.Caravans.Map.WateringHoles) != 2 {
					t.Fatal("wrong caravan inventory")
				}
			}
			change("", 400)
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if r := s.rooms[id]; r.CatanScenario != scene || !r.CatanOptions.FiveSix {
				t.Fatal("rematch lost scenario")
			}
		})
	}
}
