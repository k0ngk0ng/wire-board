package server

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func newPublicFishingTable(t *testing.T, n int) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts, clients, id := newPublicScenarioTable(t, n, "fishing")
	if s.rooms[id].Game.Catan.Fishing == nil {
		t.Fatal("missing public fishing state")
	}
	return s, ts, clients, id
}

func newPublicScenarioTable(t *testing.T, n int, scenario string) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts := setupServer(t)
	stopBotTicker(s)
	clients := make([]*testClient, n+1)
	for p := range clients {
		clients[p] = newClient(t, ts.URL)
		clients[p].register(fmt.Sprintf("公开剧本玩家%d", p))
	}
	raw := clients[0].post("/api/rooms", map[string]any{"name": "公开剧本完整对局", "kind": "catan", "capacity": n, "catanScenario": scenario}, 201)
	id := raw["id"].(string)
	for p := 1; p < n; p++ {
		clients[p].command(current(clients[0]), "join", nil, 200)
	}
	for p := 0; p < n; p++ {
		clients[p].command(current(clients[0]), "ready", nil, 200)
	}
	clients[0].command(current(clients[0]), "start", nil, 200)
	clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
	return s, ts, clients, id
}

func TestCatanFishingPublicConfiguration(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("渔夫房主")
	guest.register("渔夫朋友")
	for _, body := range []map[string]any{
		{"kind": "catan", "capacity": 2},
		{"kind": "catan", "capacity": 5, "catanOptions": game.CatanOptions{FiveSix: true}},
		{"kind": "catan", "capacity": 4, "catanOptions": game.CatanOptions{Helpers: true}},
		{"kind": "catan", "capacity": 3, "catanCitiesKnights": game.CatanCitiesKnightsSetup{Layout: "fixed"}},
		{"kind": "splendor", "capacity": 3},
	} {
		body["name"], body["catanScenario"] = "无效渔夫", "fishing"
		host.post("/api/rooms", body, 400)
	}
	raw := host.post("/api/rooms", map[string]any{"name": "渔夫选择", "kind": "catan", "capacity": 4, "catanScenario": "fishing"}, 201)
	id := raw["id"].(string)
	guest.command(raw, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	choose := func(c *testClient, scenario string, status int) {
		c.post("/api/rooms/"+id, map[string]any{"type": "catan_scenario", "catanScenario": scenario, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	for _, scenario := range []string{"fishing", "shores", "cities-knights", "fishing"} {
		for _, c := range []*testClient{host, guest} {
			c.command(current(c), "ready", nil, 200)
		}
		before, _ := json.Marshal(s.rooms[id])
		choose(guest, scenario, 400)
		choose(host, "invalid", 400)
		after, _ := json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("invalid selection mutated room")
		}
		same := s.rooms[id].CatanScenario == scenario
		choose(host, scenario, 200)
		for _, seat := range s.rooms[id].Seats {
			if seat.Ready != (same || seat.Bot) {
				t.Fatal("incorrect ready reset")
			}
		}
		if same {
			for _, c := range []*testClient{host, guest} {
				c.command(current(c), "ready", nil, 200)
			}
		}
	}
	s, ts = restartRiversHTTP(t, s, ts, []*testClient{host, guest}, id)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.Fishing == nil || len(g.Players) != 3 || g.Seafarers != nil || g.CitiesKnights != nil || len(g.Fishing.Tokens.DrawPile) != 30 {
		t.Fatal("wrong actual fishing opening")
	}
	choose(host, "", 400)
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if s.rooms[id].CatanScenario != "fishing" {
		t.Fatal("rematch lost recipe")
	}
	choose(host, "", 200)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	if s.rooms[id].Game.Catan.Fishing != nil {
		t.Fatal("base retained fishing")
	}
}
