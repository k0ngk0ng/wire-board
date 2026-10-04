package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func provisionSeafarers(t *testing.T, s *Server, id string, setup game.CatanSeafarersSetup) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rooms[id].setCatanSeafarers(setup); err != nil {
		t.Fatal(err)
	}
	if err := s.save(s.rooms[id]); err != nil {
		t.Fatal(err)
	}
}

func selectSeafarers(c *testClient, setup *game.CatanSeafarersSetup, status int) {
	r := current(c)
	c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "catan_seafarers", "catanSeafarers": setup, "version": r["version"], "nonce": randomID(12)}, status)
}

func TestCatanSeafarersConfigurationPermissionsReadinessAndRestart(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("航海房主")
	guest.register("航海朋友")
	host.post("/api/rooms", map[string]any{"kind": "catan", "name": "未开放选项", "capacity": 4, "catanSeafarers": game.CatanSeafarersSetup{Scenario: "shores"}}, 400)
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "航海剧本配置", "capacity": 4}, 201)
	if _, exposed := raw["catanSeafarersChoices"]; exposed || raw["catanSeafarers"] != nil {
		t.Fatal("unfinished public choices exposed")
	}
	id := raw["id"].(string)
	guest.command(current(host), "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	shores := game.CatanSeafarersSetup{Scenario: "shores"}
	selectSeafarers(host, &shores, 400) // no public activation
	provisionSeafarers(t, s, id, shores)
	host.command(current(host), "ready", nil, 200)
	guest.command(current(guest), "ready", nil, 200)
	before, _ := json.Marshal(s.rooms[id])
	selectSeafarers(guest, &shores, 400)
	selectSeafarers(host, nil, 400)
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "unknown"}, 400)
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "pirate_islands", Layout: "variable"}, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("invalid config mutated room")
	}
	selectSeafarers(host, &shores, 200)
	for _, p := range s.rooms[id].Seats {
		if !p.Ready {
			t.Fatal("same configuration reset readiness")
		}
	}
	stale := current(host)
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "new_world"}, 200)
	r := s.rooms[id]
	if r.CatanSeafarers.Layout != "prepared" || r.CatanNewWorldMap == nil {
		t.Fatal("missing map draft")
	}
	for _, p := range r.Seats {
		if p.Ready != p.Bot {
			t.Fatal("changed config did not reset humans")
		}
	}
	confirmed := *r.CatanNewWorldMap
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "new_world"}, 200)
	if !reflect.DeepEqual(*s.rooms[id].CatanNewWorldMap, confirmed) {
		t.Fatal("same scenario re-rolled map")
	}
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_seafarers", "catanSeafarers": shores, "version": stale["version"], "nonce": randomID(12)}, 409)
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
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	host.base, guest.base = ts2.URL, ts2.URL
	after, _ = json.Marshal(next.rooms[id])
	if string(before) != string(after) {
		t.Fatal("waiting config/map/readiness changed on restart")
	}
	guest.command(current(guest), "ready", nil, 200)
	host.command(current(host), "start", nil, 200)
	r = next.rooms[id]
	if r.Game.Phase != "catan_world_ports" || !reflect.DeepEqual(*r.Game.Catan.NewWorldMap(), confirmed) {
		t.Fatal("start discarded confirmation")
	}
	if remaining := r.TurnDeadline - time.Now().UnixMilli(); remaining < 118000 || remaining > 120000 {
		t.Fatal("incorrect initial clock", remaining)
	}
	selectSeafarers(host, &shores, 400)
	host.command(current(host), "close", nil, 200)
	_, profile := host.request("GET", "/api/players/"+r.Host, nil)
	match := profile["history"].([]any)[0].(map[string]any)
	if match["catanRules"] != game.CatanSeafarersRules || match["catanLayout"] != "prepared" || match["catanScenario"] != "new_world" {
		t.Fatal("archived configuration missing", match)
	}
}

func TestCatanSeafarersConfigurationChangesMapAndPlayerRecipe(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host := newClient(t, ts.URL)
	host.register("航海换人数")
	raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "航海人数", "capacity": 4}, 201)
	id := raw["id"].(string)
	provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: "shores", Layout: "fixed"})
	changeOptions := func(six bool) {
		r := current(host)
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_options", "catanOptions": game.CatanOptions{FiveSix: six, Helpers: true}, "version": r["version"], "nonce": randomID(12)}, 200)
	}
	changeOptions(true)
	if s.rooms[id].CatanSeafarers.Layout != "variable" || s.rooms[id].Capacity != 5 {
		t.Fatal("five/six shores recipe not selected")
	}
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "shores", Layout: "fixed"}, 400)
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "new_world"}, 200)
	if len(s.rooms[id].CatanNewWorldMap.Hexes) != 63 {
		t.Fatal("wrong extension map size")
	}
	changeOptions(false)
	if len(s.rooms[id].CatanNewWorldMap.Hexes) != 42 || s.rooms[id].CatanSeafarers.Layout != "prepared" {
		t.Fatal("wrong smaller map")
	}
	selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "fog", Layout: "variable"}, 200)
	if s.rooms[id].CatanNewWorldMap != nil {
		t.Fatal("stray new world map retained")
	}
	r := current(host)
	host.post("/api/rooms/"+id, map[string]any{"type": "catan_world_map_shuffle", "version": r["version"], "nonce": randomID(12)}, 400)
	changeOptions(true)
	if s.rooms[id].CatanSeafarers.Layout != "fixed" {
		t.Fatal("unsupported five/six variable fog persisted")
	}
	if current(host)["catanSeafarers"].(map[string]any)["scenario"] != "fog" {
		t.Fatal("selected scenario missing from room view")
	}
}

// Provision only the waiting-room selection, then use the real ready/start
// endpoints. No game state is swapped in after starting.
func TestCatanSeafarersConfiguredHTTPSetup(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, info := range game.CatanSeafarersScenarios(n) {
			for _, layout := range info.Layouts {
				t.Run(fmt.Sprintf("%d/%s/%s", n, info.ID, layout), func(t *testing.T) {
					s, ts := setupServer(t)
					stopBotTicker(s)
					host := newClient(t, ts.URL)
					host.register("航海开局")
					raw := host.post("/api/rooms", map[string]any{"kind": "catan", "name": "正式开局", "capacity": n, "catanOptions": game.CatanOptions{FiveSix: n > 4, Helpers: true}}, 201)
					id := raw["id"].(string)
					for i := 1; i < n; i++ {
						host.command(current(host), "add_bot", nil, 200)
					}
					provisionSeafarers(t, s, id, game.CatanSeafarersSetup{Scenario: info.ID, Layout: layout})
					host.command(current(host), "start", nil, 400)
					host.command(current(host), "ready", nil, 200)
					host.command(current(host), "start", nil, 200)
					g := s.rooms[id].Game.Catan
					if g.Seafarers.Layout != layout || g.Seafarers.Rules != game.CatanSeafarersRules {
						t.Fatal("wrong frozen recipe")
					}
					selectSeafarers(host, &game.CatanSeafarersSetup{Scenario: "shores"}, 400)
					// Both bots and absent humans must complete the entire setup via
					// the production timeout handler, with a fresh clock each step.
					for step := 0; step < 120 && s.rooms[id].Game.Phase != "catan_roll"; step++ {
						s.mu.Lock()
						r := s.rooms[id]
						version := r.Version
						r.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
						s.expireSetups(time.Now())
						s.mu.Unlock()
						if s.rooms[id].Version <= version {
							t.Fatal("timeout stuck", s.rooms[id].Game.Phase)
						}
					}
					r := s.rooms[id]
					if r.Game.Phase != "catan_roll" || r.Game.Catan.SetupStep != r.Game.Catan.SetupLimit() {
						t.Fatal("setup did not finish")
					}
					if left := r.TurnDeadline - time.Now().UnixMilli(); left < 118000 || left > 120000 {
						t.Fatal("first roll did not receive full clock", left)
					}
				})
			}
		}
	}
}
