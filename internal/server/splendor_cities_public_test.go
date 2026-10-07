package server

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// All starts use public room commands; no initial snapshots or granted cards.
func TestSplendorPublicCitiesHTTPCombinationGames(t *testing.T) {
	for n := 2; n <= 4; n++ {
		for mask := 0; mask < 8; mask++ {
			t.Run(fmt.Sprintf("players%d/options%d", n, mask), func(t *testing.T) {
				s, ts := setupServer(t)
				stopBotTicker(s)
				clients := make([]*testClient, n+1)
				for i := range clients {
					clients[i] = newClient(t, ts.URL)
					clients[i].register(fmt.Sprintf("城市正式开局%d", i))
				}
				options := game.SplendorOptions{Cities: true, Orient: mask&1 != 0, TradingPosts: mask&2 != 0, Strongholds: mask&4 != 0, ExtraNobles: true}
				host := clients[0]
				room := host.post("/api/rooms", map[string]any{"name": "城市组合", "kind": "splendor", "capacity": n, "splendorOptions": options}, 201)
				id := room["id"].(string)
				for _, c := range clients[1:n] {
					c.command(current(host), "join", nil, 200)
				}
				for _, c := range clients[:n] {
					c.command(current(c), "ready", nil, 200)
				}
				host.command(current(host), "start", nil, 200)
				clients[n].post("/api/rooms/"+id+"/watch", map[string]any{}, 200)
				options.Rules, options.ExtraNobles = game.SplendorExpansionRules, false
				g := s.rooms[id].Game.Splendor
				if g.Options != options || s.rooms[id].SplendorOptions != options || g.Catalog != "2025-cities-bga-v1" || len(g.Cities) != 3 || len(g.Nobles) != 0 {
					t.Fatal("public city setup lost rules", g)
				}
				cities := g.Cities
				runSplendorHTTPGame(t, s, ts, clients, id)
				// The shared runner restarts the service; inspect its final public view.
				raw, _ := json.Marshal(current(host)["game"].(map[string]any)["splendor"].(map[string]any)["cities"])
				var final []game.GemCity
				if err := json.Unmarshal(raw, &final); err != nil || !reflect.DeepEqual(final, cities) {
					t.Fatal("city goals changed during match", err)
				}
			})
		}
	}
}

func TestSplendorPublicCitiesWaitingTimeoutAndRematch(t *testing.T) {
	s, ts := setupServer(t)
	stopBotTicker(s)
	host, guest := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("城市切换房主")
	guest.register("城市切换朋友")
	r := host.post("/api/rooms", map[string]any{"name": "城市规则切换", "kind": "splendor", "capacity": 2, "splendorOptions": game.SplendorOptions{ExtraNobles: true}}, 201)
	id := r["id"].(string)
	guest.command(r, "join", nil, 200)
	change := func(c *testClient, o game.SplendorOptions, status int) {
		r := current(c)
		c.post("/api/rooms/"+id, map[string]any{"type": "splendor_options", "splendorOptions": o, "version": r["version"], "nonce": randomID(12)}, status)
	}
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	options := game.SplendorOptions{Cities: true, ExtraNobles: true, Orient: true, TradingPosts: true, Strongholds: true}
	change(guest, options, 400)
	change(host, options, 200)
	if s.rooms[id].SplendorOptions.ExtraNobles {
		t.Fatal("city retained extra nobles")
	}
	for _, seat := range s.rooms[id].Seats {
		if seat.Ready {
			t.Fatal("rule change retained readiness")
		}
	}
	host.command(current(host), "start", nil, 400)
	for _, c := range []*testClient{host, guest} {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	change(host, game.SplendorOptions{}, 400)
	actor := s.rooms[id].Game.Turn
	clients := []*testClient{host, guest}
	cities := s.rooms[id].Game.Splendor.Cities
	expireTurn(t, s, id)
	now := time.Now()
	s.mu.Lock()
	s.expireSetups(now)
	s.runBots(now)
	s.mu.Unlock()
	if !s.rooms[id].Seats[actor].AutoPlay || !s.rooms[id].Seats[actor].TimeoutAutoPlay {
		t.Fatal("timeout did not persist takeover")
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	if !reflect.DeepEqual(s.rooms[id].Game.Splendor.Cities, cities) || !s.rooms[id].Seats[actor].TimeoutAutoPlay {
		t.Fatal("restart changed cities or takeover")
	}
	setAutoPlay(clients[actor], current(clients[actor]), false, 200)
	if s.rooms[id].Seats[actor].AutoPlay {
		t.Fatal("reclaim failed")
	}
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	if !s.rooms[id].SplendorOptions.Cities || s.rooms[id].Game != nil {
		t.Fatal("rematch lost city recipe")
	}
	change(host, game.SplendorOptions{ExtraNobles: true}, 200)
	for _, c := range clients {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Splendor
	if g.Options.Cities || !g.Options.ExtraNobles || len(g.Cities) != 0 || len(g.Nobles) != 3 || len(g.Decks) != 3 {
		t.Fatal("base rematch retained city state")
	}

}
