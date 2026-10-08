package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanFriendlyOutsideHTTPRestoreManualAutoplayTimeout(t *testing.T) {
	for _, mode := range []string{"manual", "autoplay", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			s, ts, clients, id := newCatanTable(t)
			clients[3].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
			state, err := game.NewCatanFriendlySeafarers(3, game.CatanOptions{}, game.CatanSeafarersSetup{Scenario: "shores"})
			if err != nil {
				t.Fatal(err)
			}
			g := state.Catan
			// Explicit minimal response topology: two protected land tiles, no
			// desert or sea. This is not claimed as a reachable official board.
			g.Tiles = []game.CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0}}, {ID: 1, Resource: 1, Number: 8, Vertices: []int{1}}}
			g.Vertices = []game.CatanVertex{{ID: 0, Owner: 1, Level: 1}, {ID: 1, Owner: 2, Level: 1}}
			g.Edges, g.Ports = []game.CatanEdge{}, []game.CatanPort{}
			g.Robber, g.SetupStep, g.TurnSerial = 0, g.SetupLimit(), 1
			g.Seafarers.Pirate = -1
			for p := range g.Players {
				g.Players[p].Score = 2
				g.Players[p].Resources[0] = 1
				g.Bank[0]--
			}
			state.Turn, state.Phase, g.ResumePhase = 0, "catan_robber", "catan_turn"
			r := s.rooms[id]
			r.Game = state
			r.TurnDeadline = time.Now().Add(45 * time.Second).UnixMilli()
			deadline := r.TurnDeadline
			if err := s.save(r); err != nil {
				t.Fatal(err)
			}
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			v := current(clients[0])["game"].(map[string]any)["catan"].(map[string]any)
			legal := v["legal"].(map[string]any)["robber"].([]any)
			if len(legal) != 1 || legal[0] != float64(-1) || v["friendlyRobber"].(map[string]any)["fallback"] != game.CatanFriendlySeaFallbackRules {
				t.Fatal("lost public fallback")
			}
			before, _ := json.Marshal(s.rooms[id])
			for _, c := range []*testClient{clients[1], clients[3]} {
				c.command(current(c), "action", game.Action{Type: "catan_robber", Tile: -1}, 400)
			}
			clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_robber", Tile: 1}, 400)
			after, _ := json.Marshal(s.rooms[id])
			if string(before) != string(after) {
				t.Fatal("invalid move changed game")
			}
			switch mode {
			case "manual":
				clients[0].command(current(clients[0]), "action", game.Action{Type: "catan_robber", Tile: -1}, 200)
			case "autoplay":
				setAutoPlay(clients[0], current(clients[0]), true, 200)
				s.mu.Lock()
				s.rooms[id].BotAt = 0
				s.runBots(time.Now())
				s.mu.Unlock()
			case "timeout":
				s.mu.Lock()
				s.expireSetups(time.UnixMilli(deadline))
				s.mu.Unlock()
			}
			r = s.rooms[id]
			if r.Game.Phase != "catan_turn" || len(r.Game.Catan.Victims) != 0 {
				t.Fatal("response stalled", r.Game.Phase)
			}
			if mode == "manual" && r.Game.Catan.Robber != -1 {
				t.Fatal("manual retreat failed")
			}
			for _, p := range r.Game.Catan.Players {
				if !slices.Equal(p.Resources, []int{1, 0, 0, 0, 0}) {
					t.Fatal("retreat stole resources")
				}
			}
			if mode == "manual" && r.TurnDeadline != deadline {
				t.Fatal("response refreshed clock")
			}
			if mode == "timeout" && !r.Seats[0].TimeoutAutoPlay {
				t.Fatal("timeout did not persist takeover")
			}
		})
	}
}

func TestCatanFriendlyWorldPublicEditAndBaseReset(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, ts := setupServer(t)
			stopBotTicker(s)
			h := newClient(t, ts.URL)
			h.register("友善新世界")
			raw := h.post("/api/rooms", map[string]any{"kind": "catan", "name": "友善地图", "capacity": n, "catanScenario": "new_world", "catanOptions": game.CatanOptions{FiveSix: n > 4}, "catanFriendlyRobber": game.CatanFriendlyRobberSetup{Enabled: true}}, 201)
			id := raw["id"].(string)
			change := func(kind, key string, value any) {
				h.post("/api/rooms/"+id, map[string]any{"type": kind, key: value, "version": s.rooms[id].Version, "nonce": randomID(12)}, 200)
			}
			change("catan_world_map_shuffle", "catanNewWorldMap", nil)
			layout := *s.rooms[id].CatanNewWorldMap
			layout.Hexes = slices.Clone(layout.Hexes)
			first, second := -1, -1
			for i, h := range layout.Hexes {
				if h.Resource < 5 && h.Number != 6 && h.Number != 8 {
					if first < 0 {
						first = i
					} else if layout.Hexes[first].Resource != h.Resource {
						second = i
						break
					}
				}
			}
			if second < 0 {
				t.Fatal("no terrain swap")
			}
			layout.Hexes[first].Resource, layout.Hexes[second].Resource = layout.Hexes[second].Resource, layout.Hexes[first].Resource
			change("catan_world_map", "catanNewWorldMap", &layout)
			for i := 1; i < n; i++ {
				h.command(current(h), "add_bot", nil, 200)
			}
			h.command(current(h), "ready", nil, 200)
			s, ts = restartRiversHTTP(t, s, ts, []*testClient{h}, id)
			h.command(current(h), "start", nil, 200)
			g := s.rooms[id].Game.Catan
			if g.FriendlyRobber.Fallback != game.CatanFriendlySeaFallbackRules {
				t.Fatal("missing actual fallback")
			}
			got, _ := json.Marshal(g.NewWorldMap())
			want, _ := json.Marshal(layout)
			if string(got) != string(want) {
				t.Fatal("confirmed map changed")
			}
			h.command(current(h), "close", nil, 200)
			h.command(current(h), "rematch", nil, 200)
			selectCatanFriendlyRobber(h, false, 200)
			change("catan_scenario", "catanScenario", "")
			h.command(current(h), "ready", nil, 200)
			h.command(current(h), "start", nil, 200)
			if s.rooms[id].Game.Catan.FriendlyRobber != nil || s.rooms[id].Game.Catan.Seafarers != nil {
				t.Fatal("base inherited fallback")
			}
		})
	}
}
