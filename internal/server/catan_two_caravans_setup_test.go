package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
)

func TestCatanTwoCaravansPrivateRecipeReadinessAndActualHistory(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := []*testClient{newClient(t, ts.URL), newClient(t, ts.URL)}
	clients[0].register("双人商队房主")
	clients[1].register("双人商队朋友")
	clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "不能注入内部商队", "capacity": 3, "catanTwoScenario": "caravans"}, 400)
	raw := clients[0].post("/api/rooms", map[string]any{"kind": "catan", "name": "商队配置", "capacity": 3}, 201)
	id := raw["id"].(string)
	clients[1].command(current(clients[0]), "join", nil, 200)
	// A verified internal river draft may change to merchant trains while waiting;
	// that change still clears both players' already-confirmed readiness.
	if e := s.rooms[id].setCatanTwoScenario("rivers"); e != nil {
		t.Fatal(e)
	}
	if e := s.save(s.rooms[id]); e != nil {
		t.Fatal(e)
	}
	for _, c := range clients {
		c.command(current(c), "ready", nil, 200)
	}
	r := s.rooms[id]
	before, _ := json.Marshal(r)
	if r.setCatanTwoScenario("unverified") == nil {
		t.Fatal("unknown recipe")
	}
	after, _ := json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("rejected recipe mutated readiness")
	}
	if e := r.setCatanTwoScenario("caravans"); e != nil {
		t.Fatal(e)
	}
	if e := s.save(r); e != nil {
		t.Fatal(e)
	}
	for _, seat := range r.Seats {
		if seat.Ready {
			t.Fatal("recipe did not clear readiness")
		}
	}
	clients[0].command(current(clients[0]), "start", nil, 400)
	clients[0].post("/api/rooms/"+id, map[string]any{"type": "catan_options", "version": r.Version, "nonce": randomID(12), "catanOptions": game.CatanOptions{Helpers: true}}, 400)
	clients[1].post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "version": r.Version, "nonce": randomID(12), "catanTwoScenario": "caravans"}, 400)
	for _, bad := range []func(*Room){func(r *Room) { r.CatanTwoRules = "" }, func(r *Room) { r.Capacity = 3 }, func(r *Room) { r.CatanTwoScenario = "unknown" }, func(r *Room) { r.CatanSeafarers = &game.CatanSeafarersSetup{} }} {
		trial := *r
		bad(&trial)
		if trial.validateCatanTwoSetup() == nil {
			t.Fatal("bad river recipe accepted")
		}
	}
	for _, c := range clients {
		c.command(current(c), "ready", nil, 200)
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	waiting := current(clients[0])
	if waiting["catanTwoScenario"] != "caravans" {
		t.Fatal("recipe view lost on restart")
	}
	clients[0].command(waiting, "start", nil, 200)
	r = s.rooms[id]
	if r.Game.Catan.Caravans == nil || r.Game.Catan.Caravans.Rules != game.CatanCaravansRules || r.Game.Phase != "catan_setup_settlement" {
		t.Fatal("formal start constructed wrong game")
	}
	before, _ = json.Marshal(r)
	if r.setCatanTwo() == nil {
		t.Fatal("changed active recipe")
	}
	after, _ = json.Marshal(r)
	if string(before) != string(after) {
		t.Fatal("running recipe changed")
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	// Archive actual saved versions even if the waiting draft is stale.
	s.rooms[id].CatanTwoScenario = "stale"
	s.rooms[id].CatanTwoRules = "stale"
	clients[0].command(current(clients[0]), "close", nil, 200)
	code, profile := clients[0].request("GET", "/api/players/"+s.rooms[id].Seats[0].ID, nil)
	if code != 200 {
		t.Fatal("missing profile")
	}
	record := profile["history"].([]any)[0].(map[string]any)
	versions := record["catanExpansionRules"].(map[string]any)
	if record["catanScenario"] != "caravans" || record["catanRules"] != game.CatanTwoRules || versions["two_player"] != game.CatanTwoRules || versions["caravans"] != game.CatanCaravansRules {
		t.Fatal("archive read stale draft", record)
	}
}

func TestCatanCaravansHistoryUsesActualRules(t *testing.T) {
	s, _, clients, id := newCaravansFullTable(t, 3)
	// Old internal ordinary-train snapshots omitted the explicit version field.
	// Their actual map is still the pinned 2025 scenario, not the room draft.
	s.mu.Lock()
	s.rooms[id].Game.Catan.Caravans.Rules = ""
	s.rooms[id].CatanTwoScenario = "stale"
	s.mu.Unlock()
	clients[0].command(current(clients[0]), "close", nil, 200)
	_, profile := clients[0].request("GET", "/api/players/"+s.rooms[id].Seats[0].ID, nil)
	record := profile["history"].([]any)[0].(map[string]any)
	versions := record["catanExpansionRules"].(map[string]any)
	if record["catanRules"] != game.CatanCaravansRules || versions["caravans"] != game.CatanCaravansRules || versions["two_player"] != nil || record["catanScenario"] != "caravans" {
		t.Fatal("wrong actual merchant train history", record)
	}
}
