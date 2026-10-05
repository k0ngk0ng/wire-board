package server

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanCitiesKnightsSeafarersConfigurationHTTP(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("组合房主")
	guest.register("组合客人")
	city := game.CatanCitiesKnightsSetup{}
	sea := game.CatanSeafarersSetup{Scenario: "shores"}
	host.post("/api/rooms", map[string]any{"kind": "catan", "name": "未开放组合", "capacity": 4, "catanCitiesKnights": city, "catanSeafarers": sea}, 400)
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "内部组合配置", "capacity": 4}, 201)
	id := raw["id"].(string)
	guest.command(current(host), "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	// Single internally provisioned expansions cannot enable the other via HTTP.
	provisionCatanCitiesKnights(t, s, id)
	selectSeafarers(host, &sea, 400)
	provisionSeafarers(t, s, id, sea)
	r := current(host)
	choices := r["catanSeafarersChoices"].([]any)
	if len(choices) != 5 {
		t.Fatal("wrong combination catalog", choices)
	}
	for _, raw := range choices {
		item := raw.(map[string]any)
		if !game.CatanCitiesKnightsSeafarersSupported(item["id"].(string)) {
			t.Fatal("unfinished combination exposed")
		}
		if item["id"] == "shores" && item["victoryPoints"] != float64(16) {
			t.Fatal("wrong combined target")
		}
	}
	host.command(current(host), "ready", nil, 200)
	guest.command(current(host), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	selectSeafarers(guest, &sea, 400)
	selectCatanCitiesKnights(guest, &city, 400)
	for _, scenario := range []string{"tribe", "cloth", "pirate_islands", "wonders"} {
		selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: scenario}, 400)
	}
	options := func(o game.CatanOptions, status int) {
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": o, "version": current(host)["version"], "nonce": randomID(12)}, status)
	}
	options(game.CatanOptions{Helpers: true}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid combination mutated room")
	}
	selectSeafarers(host, &sea, 200)
	selectCatanCitiesKnights(host, &city, 200)
	for _, seat := range s.rooms[id].Seats {
		if !seat.Ready {
			t.Fatal("unchanged config reset readiness")
		}
	}
	options(game.CatanOptions{FiveSix: true}, 200)
	if s.rooms[id].CatanCitiesKnights.Rules != game.CatanCitiesKnightsFiveSixRules || s.rooms[id].CatanSeafarers.Layout != "variable" {
		t.Fatal("size change did not resolve both recipes")
	}
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready != seat.Bot {
			t.Fatal("size change did not reset humans")
		}
	}
	options(game.CatanOptions{}, 200)
	if s.rooms[id].CatanCitiesKnights.Rules != game.CatanCitiesKnightsRules || s.rooms[id].CatanSeafarers.Layout != "fixed" {
		t.Fatal("size decrease did not resolve both recipes")
	}
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "new_world"}, 200)
	shuffle := func(c *testClient, status int) { c.command(current(c), "catan_world_map_shuffle", nil, status) }
	shuffle(guest, 400)
	shuffle(host, 200)
	confirmed := *s.rooms[id].CatanNewWorldMap
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "new_world"}, 200)
	if !reflect.DeepEqual(confirmed, *s.rooms[id].CatanNewWorldMap) {
		t.Fatal("same recipe rerolled prepared map")
	}
	// A 5/6 switch regenerates a valid larger map, then returns to 3/4.
	options(game.CatanOptions{FiveSix: true}, 200)
	if err := game.ValidateCatanNewWorldMap(5, s.rooms[id].CatanNewWorldMap); err != nil {
		t.Fatal(err)
	}
	options(game.CatanOptions{}, 200)
	confirmed = *s.rooms[id].CatanNewWorldMap
	host.command(current(host), "ready", nil, 200)
	guest.command(current(host), "ready", nil, 200)
	before, _ = json.Marshal(s.rooms[id])
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	stopBotTicker(next)
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	host.base, guest.base = ts2.URL, ts2.URL
	after, _ = json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("waiting combo lost on restart")
	}
	host.command(current(host), "start", nil, 200)
	room := next.rooms[id]
	if room.Game.Phase != "catan_world_ports" || room.Game.Catan.CitiesKnights == nil || room.Game.Catan.Seafarers.VictoryPoints != 14 || !reflect.DeepEqual(confirmed, *room.Game.Catan.NewWorldMap()) {
		t.Fatal("formal combo start lost confirmed map or rules")
	}
	selectSeafarers(host, &sea, 400)
	selectCatanCitiesKnights(host, &city, 400)
	shuffle(host, 400)
	// History must come from the game, never stale configuration fields.
	room.CatanSeafarers = &game.CatanSeafarersSetup{Scenario: "tribe", Layout: "stale", Rules: "stale"}
	room.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{Rules: "stale"}
	host.command(current(host), "close", nil, 200)
	_, profile := host.request("GET", "/api/players/"+room.Host, nil)
	match := profile["history"].([]any)[0].(map[string]any)
	versions := match["catanExpansionRules"].(map[string]any)
	if match["catanScenario"] != "new_world" || match["catanLayout"] != "prepared" || match["catanRules"] != game.CatanSeafarersRules || versions["cities_knights"] != game.CatanCitiesKnightsRules || versions["seafarers"] != game.CatanSeafarersRules {
		t.Fatal("wrong combined history", match)
	}
}

func TestCatanCitiesKnightsSeafarersStartRejectsMalformedSetup(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("组合非法设置")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "capacity": 4, "name": "组合校验"}, 201)
	id := raw["id"].(string)
	provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: "shores"})
	selectCatanCitiesKnights(host, &game.CatanCitiesKnightsSetup{}, 400)
	provisionCatanCitiesKnights(t, s, id)
	for range 2 {
		host.command(current(host), "add_bot", nil, 200)
	}
	host.command(current(host), "ready", nil, 200)
	for _, kind := range []string{"city_rules", "sea_rules", "world_missing", "world_extra", "base"} {
		room := s.rooms[id]
		city, sea := *room.CatanCitiesKnights, *room.CatanSeafarers
		switch kind {
		case "city_rules":
			room.CatanCitiesKnights = &game.CatanCitiesKnightsSetup{Rules: game.CatanCitiesKnightsFiveSixRules}
		case "sea_rules":
			room.CatanSeafarers = &game.CatanSeafarersSetup{Scenario: "shores", Rules: "invalid"}
		case "world_missing":
			room.CatanSeafarers = &game.CatanSeafarersSetup{Scenario: "new_world"}
		case "world_extra":
			room.CatanNewWorldMap = &game.CatanNewWorldMap{}
		case "base":
			room.CatanBaseConfiguration = &game.CatanBaseConfiguration{}
		}
		before, _ := json.Marshal(room)
		host.command(current(host), "start", nil, 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal(kind, "invalid start mutated room")
		}
		room.CatanCitiesKnights, room.CatanSeafarers = &city, &sea
		room.CatanNewWorldMap = nil
		room.CatanBaseConfiguration = nil
	}
}
