package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanFishingExtendedPublicConfiguration(t *testing.T) {
	for _, scene := range []string{"fog", "wonders", "new_world"} {
		t.Run(scene, func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h, c := newClient(t, ts.URL), newClient(t, ts.URL)
			h.register("扩大捕鱼房主")
			c.register("扩大捕鱼朋友")
			raw := h.post("/api/rooms", map[string]any{"name": "扩大捕鱼", "kind": "catan", "capacity": 4, "catanScenario": scene, "catanFishing": true}, 201)
			id := raw["id"].(string)
			c.command(raw, "join", nil, 200)
			command := func(client *testClient, kind string, extra map[string]any, status int) {
				extra["type"], extra["version"], extra["nonce"] = kind, s.rooms[id].Version, randomID(12)
				client.post("/api/rooms/"+id, extra, status)
			}
			options := func(client *testClient, extended bool, status int) {
				command(client, "catan_options", map[string]any{"catanOptions": game.CatanOptions{FiveSix: extended}}, status)
			}
			h.command(current(h), "ready", nil, 200)
			c.command(current(c), "ready", nil, 200)
			before, _ := json.Marshal(s.rooms[id])
			options(c, true, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("unauthorized change mutated room")
			}
			options(h, true, 200)
			r := s.rooms[id]
			if r.Capacity != 5 || !r.CatanOptions.FiveSix || !r.CatanFishing {
				t.Fatal("extension not retained")
			}
			for _, seat := range r.Seats {
				if seat.Ready {
					t.Fatal("extension retained human ready")
				}
			}
			choices := current(h)["catanSeafarersChoices"].([]any)
			if len(choices) != 3 {
				t.Fatal("wrong extended catalogue", choices)
			}
			for _, raw := range choices {
				choice := raw.(map[string]any)
				if !slices.Contains([]string{"fog", "wonders", "new_world"}, choice["id"].(string)) {
					t.Fatal("unsupported expanded map")
				}
			}
			if scene == "new_world" && len(r.CatanNewWorldMap.Hexes) != 63 {
				t.Fatal("world was not enlarged")
			}
			command(h, "catan_fishing", map[string]any{"enabled": false}, 200)
			if s.rooms[id].CatanFishing {
				t.Fatal("fishing was not disabled")
			}
			command(h, "catan_fishing", map[string]any{"enabled": true}, 200)
			before, _ = json.Marshal(s.rooms[id])
			selectSeafarers(h, &game.CatanSeafarersSetup{Scenario: "cloth"}, 400)
			command(h, "catan_options", map[string]any{"catanOptions": game.CatanOptions{FiveSix: true, Helpers: true}}, 400)
			after, _ = json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid configuration mutated room")
			}
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h, c}, id)
			h.command(current(h), "ready", nil, 200)
			c.command(current(c), "ready", nil, 200)
			h.command(current(h), "start", nil, 400)
			for len(s.rooms[id].Seats) < 5 {
				h.command(current(h), "add_bot", nil, 200)
			}
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if len(g.Players) != 5 || g.Paired == nil || g.Fishing == nil || len(g.Fishing.Tokens.DrawPile) != 44 {
				t.Fatal("wrong enlarged opening")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			if !s.rooms[id].CatanFishing || !s.rooms[id].CatanOptions.FiveSix {
				t.Fatal("rematch lost extension")
			}
			options(h, false, 400)
			bot := s.rooms[id].Seats[4].ID
			command(h, "remove_bot", map[string]any{"target": bot}, 200)
			options(h, false, 200)
			r = s.rooms[id]
			if r.Capacity != 4 || r.CatanOptions.FiveSix || !r.CatanFishing {
				t.Fatal("extension did not reduce")
			}
			if scene == "new_world" && len(r.CatanNewWorldMap.Hexes) != 42 {
				t.Fatal("world was not reduced", len(r.CatanNewWorldMap.Hexes))
			}
			for _, seat := range r.Seats {
				if seat.Ready != seat.Bot {
					t.Fatal("reduced recipe kept ready")
				}
			}
		})
	}
}

func TestCatanFishingExtendedPublicRejectsMismatchedCounts(t *testing.T) {
	for _, scene := range []string{"islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"} {
		for _, n := range []int{3, 4, 5, 6} {
			for _, extended := range []bool{false, true} {
				if extended == (n > 4) && (n <= 4 || publicCatanFishingSeaExtended(scene)) {
					continue
				}
				t.Run(fmt.Sprintf("%s/%d/%t", scene, n, extended), func(t *testing.T) {
					s, ts := setupServer(t)
					stopBotTicker(s)
					h := newClient(t, ts.URL)
					h.register("错误捕鱼人数")
					h.post("/api/rooms", map[string]any{"name": "错误组合", "kind": "catan", "capacity": n, "catanScenario": scene, "catanFishing": true, "catanOptions": game.CatanOptions{FiveSix: extended}}, 400)
					if len(s.rooms) != 0 {
						t.Fatal("invalid room persisted")
					}
				})
			}
		}
	}
}
