package server

import (
	"encoding/json"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanSeaKnightsPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("公开航海骑士房主")
	guest.register("公开航海骑士朋友")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 4, "catanScenario": "shores", "catanOptions": game.CatanOptions{Helpers: true}},
		{"kind": "catan", "capacity": 5, "catanScenario": "shores"},
		{"kind": "catan", "capacity": 3, "catanScenario": "land-ho"},
		{"kind": "splendor", "capacity": 3},
	} {
		body["name"], body["catanCitiesKnights"] = "无效组合", game.CatanCitiesKnightsSetup{}
		host.post("/api/rooms", body, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"name": "公开航海骑士组合", "kind": "catan", "capacity": 4, "catanScenario": "shores", "catanCitiesKnights": game.CatanCitiesKnightsSetup{}}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	city := game.CatanCitiesKnightsSetup{}
	ready := func() {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
	}
	ready()
	before, _ := json.Marshal(s.rooms[id])
	selectCatanCitiesKnights(guest, nil, 400)
	selectCatanCitiesKnights(host, &game.CatanCitiesKnightsSetup{Layout: "fixed"}, 400)
	selectCatanCitiesKnights(host, &game.CatanCitiesKnightsSetup{Rules: game.CatanCitiesKnightsFiveSixRules}, 400)
	scenario := func(value string, status int) {
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("rejected mixed config mutated room")
	}
	selectCatanCitiesKnights(host, &city, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("same combo cleared ready")
		}
	}
	selectCatanCitiesKnights(host, nil, 200)
	if s.rooms[id].CatanCitiesKnights != nil || s.rooms[id].CatanSeafarers == nil {
		t.Fatal("disable changed map")
	}
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("changed combo retained ready")
		}
	}
	selectCatanCitiesKnights(host, &city, 200)
	for _, choice := range []string{"islands", "fog", "desert", "tribe", "cloth", "pirate_islands", "wonders", "new_world"} {
		ready()
		scenario(choice, 200)
		r := s.rooms[id]
		if r.CatanCitiesKnights == nil || r.CatanSeafarers.Scenario != choice {
			t.Fatal("switch lost combination")
		}
		for _, seat := range r.Seats {
			if seat.Ready != seat.Bot {
				t.Fatal("map switch retained ready")
			}
		}
	}
	if s.rooms[id].CatanNewWorldMap == nil {
		t.Fatal("new world map missing")
	}
	ready()
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.CitiesKnights == nil || g.Seafarers.Scenario != "new_world" || g.Seafarers.VictoryPoints != 14 || len(g.Players) != 3 {
		t.Fatal("wrong actual combined opening")
	}
	selectCatanCitiesKnights(host, nil, 400)
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if s.rooms[id].CatanCitiesKnights == nil || s.rooms[id].CatanNewWorldMap == nil {
		t.Fatal("rematch lost combined map")
	}
	scenario("rivers", 200)
	if s.rooms[id].CatanCitiesKnights != nil || s.rooms[id].CatanSeafarers != nil || s.rooms[id].CatanNewWorldMap != nil {
		t.Fatal("independent scenario retained combo")
	}
}
