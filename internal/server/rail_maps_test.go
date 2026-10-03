package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func TestRailMapRoomSelectionPermissionsAndPersistence(t *testing.T) {
	s, ts := setupServer(t)
	host := newClient(t, ts.URL)
	guest := newClient(t, ts.URL)
	host.register("MapHost")
	guest.register("MapGuest")
	host.post("/api/rooms", map[string]any{"name": "invalid", "kind": "rail", "railMap": "switzerland", "capacity": 4}, 400)
	host.post("/api/rooms", map[string]any{"name": "invalid", "kind": "rail", "railMap": "unknown", "capacity": 2}, 400)
	room := host.post("/api/rooms", map[string]any{"name": "欧洲之旅", "kind": "rail", "railMap": "europe", "capacity": 5}, 201)
	guest.command(room, "join", nil, 200)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	change := func(c *testClient, id string, want int) {
		r := current(c)
		c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "rail_map", "railMap": id, "version": r["version"], "nonce": randomID(12)}, want)
	}
	change(guest, "india", 400)
	change(host, "switzerland", 200)
	room = current(host)
	if room["railMap"] != "switzerland" || room["capacity"] != float64(3) {
		t.Fatal("map limits", room)
	}
	for _, seat := range room["seats"].([]any) {
		if seat.(map[string]any)["ready"] == true {
			t.Fatal("map switch did not reset readiness")
		}
	}
	host.command(room, "start", nil, 400)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	host.command(current(host), "start", nil, 200)
	room = current(host)
	rail := room["game"].(map[string]any)["rail"].(map[string]any)
	if rail["map"] != "switzerland" || len(rail["pending"].([]any)) != 5 {
		t.Fatal("wrong map started")
	}
	change(host, "usa", 400)
	s.mu.Lock()
	saved := s.rooms[room["id"].(string)]
	raw, e := json.Marshal(saved)
	var restored Room
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &restored); e != nil {
		t.Fatal(e)
	}
	s.mu.Unlock()
	if restored.RailMap != "switzerland" || restored.Game.Rail.Map != "switzerland" {
		t.Fatal("map lost on persistence")
	}
	for _, id := range []string{"usa", "europe", "india", "switzerland", "nordiccountries", "legendaryasia"} {
		status, catalog := host.request("GET", "/api/catalog?map="+id, nil)
		if status != 200 || catalog["map"].(map[string]any)["id"] != id {
			t.Fatal("catalog", id, status)
		}
	}
	code, _ := host.request("GET", "/api/catalog?map=missing", nil)
	if code != 400 {
		t.Fatal("unknown catalog accepted")
	}
}
func TestRailMapCannotShrinkOccupiedRoom(t *testing.T) {
	_, ts := setupServer(t)
	c := newClient(t, ts.URL)
	c.register("MapCapacity")
	c.post("/api/rooms", map[string]any{"name": "四人桌", "kind": "rail", "capacity": 5}, 201)
	for i := 0; i < 3; i++ {
		c.command(current(c), "add_bot", nil, 200)
	}
	room := current(c)
	c.post("/api/rooms/"+room["id"].(string), map[string]any{"type": "rail_map", "railMap": "nordiccountries", "version": room["version"], "nonce": randomID(12)}, 400)
	if len(current(c)["seats"].([]any)) != 4 {
		t.Fatal("map change removed a player")
	}
}
func TestRailTunnelKeepsClockAndMapHistory(t *testing.T) {
	s := &Room{Kind: "rail", RailMap: "europe", Status: "playing", TurnDeadline: 123456, Seats: []Seat{{User: User{ID: "a"}}, {User: User{ID: "b"}}}}
	g, e := game.NewRailMap("europe", 2)
	if e != nil {
		t.Fatal(e)
	}
	g.AutoChooseRailSetup()
	s.Game = g
	catalog, _ := game.RailCatalog("europe")
	routes := catalog["routes"].([]game.Route)
	var route game.Route
	for _, r := range routes {
		if r.Tunnel {
			route = r
			break
		}
	}
	c := max(0, route.Color)
	g.Rail.Players[0].Hand[c] = route.Length + 3
	g.Rail.Deck = []int{c, c, 8}
	g.Rail.Discard = nil
	if e = s.applyGameAction(0, game.Action{Type: "claim", Route: route.ID, Color: c}, time.Now()); e != nil {
		t.Fatal(e)
	}
	if g.Phase != "tunnel" || s.TurnDeadline != 123456 {
		t.Fatal("tunnel incorrectly restarted turn timer")
	}
	if e = s.applyGameAction(0, game.Action{Type: "tunnel_cancel"}, time.Now()); e != nil {
		t.Fatal(e)
	}
	if s.TurnDeadline == 123456 {
		t.Fatal("new player's clock did not start")
	}
}
