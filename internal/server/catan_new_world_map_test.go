package server

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanNewWorldMapReadinessRestartFreezeAndHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("地图房主")
	guest.register("地图朋友")
	raw := host.post("/api/rooms", map[string]any{"name": "新世界地图", "kind": "catan", "capacity": 3}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	change := func(c *testClient, kind string, layout *game.CatanNewWorldMap, want int) {
		r := current(c)
		c.post("/api/rooms/"+id, map[string]any{"type": kind, "catanNewWorldMap": layout, "version": r["version"], "nonce": randomID(12)}, want)
	}
	layout, err := game.GenerateCatanNewWorldMap(3)
	if err != nil {
		t.Fatal(err)
	}
	// Map commands cannot activate a different scenario by themselves.
	change(host, "catan_world_map", layout, 400)
	change(host, "catan_world_map_shuffle", nil, 400)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": "new_world", "version": current(host)["version"], "nonce": randomID(12)}, 200)
	layout = s.rooms[id].CatanNewWorldMap
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	change(guest, "catan_world_map", layout, 400)
	invalid := &game.CatanNewWorldMap{Hexes: []game.CatanNewWorldHex{{Resource: 6}}}
	change(host, "catan_world_map", invalid, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid edit changed room/readiness")
	}
	change(host, "catan_world_map", layout, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("identical map reset readiness")
		}
	}
	edited := &game.CatanNewWorldMap{Hexes: append([]game.CatanNewWorldHex{}, layout.Hexes...)}
	for i, h := range edited.Hexes {
		if h.Resource < 5 && h.Number != 6 && h.Number != 8 {
			edited.Hexes[i].Resource = game.CatanGold
			break
		}
	}
	stale := current(host)
	change(host, "catan_world_map", edited, 200)
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("map edit failed readiness reset")
		}
	}
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_world_map", "catanNewWorldMap": layout, "version": stale["version"], "nonce": randomID(12)}, 409)
	host.command(current(host), "start", nil, 400)
	host.command(current(host), "ready", nil, 200)
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
		t.Fatal("prepared map/readiness changed on restart")
	}
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	host.base, guest.base = ts2.URL, ts2.URL
	host.command(current(host), "start", nil, 400)
	guest.command(current(guest), "ready", nil, 200)
	host.command(current(host), "start", nil, 200)
	if !reflect.DeepEqual(next.rooms[id].Game.Catan.NewWorldMap(), edited) || next.rooms[id].Game.Phase != "catan_world_ports" {
		t.Fatal("start ignored approved map")
	}
	change(host, "catan_world_map", layout, 400)
	change(host, "catan_world_map_shuffle", nil, 400)
	host.command(current(host), "close", nil, 200)
	code, profile := host.request("GET", "/api/players/"+next.rooms[id].Host, nil)
	if code != 200 {
		t.Fatal("profile unavailable", code)
	}
	history := profile["history"].([]any)
	if len(history) != 1 || history[0].(map[string]any)["catanScenario"] != "new_world" {
		t.Fatal("history lost scenario identity", history)
	}
}

func TestCatanNewWorldMapShuffleAndFiveSixSwitch(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("地图换人数")
	raw := host.post("/api/rooms", map[string]any{"name": "地图换人数", "kind": "catan", "capacity": 4}, 201)
	id := raw["id"].(string)
	layout, err := game.GenerateCatanNewWorldMap(4)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.rooms[id].CatanNewWorldMap = layout
	s.save(s.rooms[id])
	s.mu.Unlock()
	host.command(current(host), "ready", nil, 200)
	r := current(host)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_world_map_shuffle", "version": r["version"], "nonce": randomID(12)}, 200)
	if s.rooms[id].Seats[0].Ready {
		t.Fatal("shuffle did not require approval")
	}
	for _, six := range []bool{true, false} {
		r = current(host)
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: six}, "version": r["version"], "nonce": randomID(12)}, 200)
		room := s.rooms[id]
		n := 4
		count := 42
		if six {
			n = 5
			count = 63
		}
		if len(room.CatanNewWorldMap.Hexes) != count || game.ValidateCatanNewWorldMap(n, room.CatanNewWorldMap) != nil || room.Seats[0].Ready {
			t.Fatal("wrong map after changing player expansion")
		}
	}
}
