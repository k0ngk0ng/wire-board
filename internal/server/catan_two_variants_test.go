package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func assertTwoVariantsHTTP(t *testing.T, s *game.State, friendly, harbors bool) {
	t.Helper()
	g := s.Catan
	if (g.FriendlyRobber != nil) != friendly || (g.Harbors != nil) != harbors || (g.Two.Variants != "") != (friendly || harbors) {
		t.Fatal("variant recipe lost")
	}
	if harbors {
		points := []int{0, 0}
		seen := map[int]bool{}
		for _, port := range g.Ports {
			e := g.Edges[port.Edge]
			for _, id := range []int{e.A, e.B} {
				if seen[id] {
					continue
				}
				seen[id] = true
				v := g.Vertices[id]
				if v.Owner >= 0 {
					points[v.Owner] += v.Level
				}
			}
		}
		view := s.View(-1)["catan"].(map[string]any)["harbors"].(map[string]any)
		if !slices.Equal(view["points"].([]int), points) {
			t.Fatal("wrong public harbor points")
		}
		if g.Harbors.Owner >= 0 && (points[g.Harbors.Owner] < 3 || points[1-g.Harbors.Owner] > points[g.Harbors.Owner]) {
			t.Fatal("wrong harbor owner")
		}
	}
	if friendly {
		protected := s.View(-1)["catan"].(map[string]any)["friendlyRobber"].(map[string]any)["protectedPlayers"].([]int)
		for p, seat := range g.Players {
			if slices.Contains(protected, p) != (seat.Score-seat.Dev[4] < 3) {
				t.Fatal("public protection leaked hidden score")
			}
		}
	}
}

func TestCatanTwoVariantsCompleteHTTPGames(t *testing.T) {
	for _, recipe := range []struct {
		scenario                  string
		events, friendly, harbors bool
	}{
		{"", false, true, false}, {"fishing", false, false, true}, {"cities-knights", false, true, true},
		{"", true, true, true}, {"fishing", true, true, false}, {"cities-knights", true, false, true},
	} {
		t.Run(fmt.Sprintf("%s/events=%v/friendly=%v/harbors=%v", recipe.scenario, recipe.events, recipe.friendly, recipe.harbors), func(t *testing.T) {
			runTwoVariantsHTTPGames(t, recipe.scenario, recipe.events, recipe.friendly, recipe.harbors)
		})
	}
}

func TestCatanTwoVariantsPublicToggleRestoreRematchAndIsolation(t *testing.T) {
	s, ts, clients, id := newTwoVariantsFullTable(t, "", true, true, true)
	host, guest := clients[0], clients[1]
	set := func(c *testClient, kind string, enabled bool, status int) {
		field := "catanHarbors"
		if kind == "catan_friendly_robber" {
			field = "catanFriendlyRobber"
		}
		c.post("/api/rooms/"+id, map[string]any{"type": kind, field: map[string]any{"enabled": enabled}, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	scene := func(value string, status int) {
		host.post("/api/rooms/"+id, map[string]any{"type": "catan_two_scenario", "catanTwoScenario": value, "version": s.rooms[id].Version, "nonce": randomID(12)}, status)
	}
	before, _ := json.Marshal(s.rooms[id])
	set(host, "catan_harbors", false, 400)
	after, _ := json.Marshal(s.rooms[id])
	if string(before) != string(after) {
		t.Fatal("playing setup mutated")
	}
	host.command(current(host), "close", nil, 200)
	host.command(current(host), "rematch", nil, 200)
	for _, scenario := range []string{"fishing", "cities-knights", ""} {
		scene(scenario, 200)
		for _, c := range clients[:2] {
			c.command(current(c), "ready", nil, 200)
		}
		before, _ = json.Marshal(s.rooms[id])
		set(guest, "catan_harbors", false, 400)
		// Rivers is a supported two-player scenario now; an unknown one is not.
		scene("unknown", 400)
		after, _ = json.Marshal(s.rooms[id])
		if string(before) != string(after) {
			t.Fatal("rejection mutated recipe/readiness")
		}
		set(host, "catan_harbors", true, 200)
		for _, seat := range s.rooms[id].Seats {
			if !seat.Ready {
				t.Fatal("same setting reset readiness")
			}
		}
		set(host, "catan_harbors", false, 200)
		for _, seat := range s.rooms[id].Seats {
			if seat.Ready {
				t.Fatal("changed setting retained readiness")
			}
		}
		set(host, "catan_harbors", true, 200)
		s, ts = restartRiversHTTP(t, s, ts, clients, id)
		availability := current(host)["catanFriendlyRobberAvailability"].(map[string]any)
		if availability["allowed"] != true || availability["minPlayers"] != float64(2) {
			t.Fatal("wrong two-player availability", availability)
		}
		for _, c := range clients[:2] {
			c.command(current(c), "ready", nil, 200)
		}
		host.command(current(host), "start", nil, 200)
		assertTwoVariantsHTTP(t, s.rooms[id].Game, true, true)
		host.command(current(host), "close", nil, 200)
		host.command(current(host), "rematch", nil, 200)
	}
	set(host, "catan_friendly_robber", false, 200)
	set(host, "catan_harbors", false, 200)
	scene("rivers", 200)
	scene("", 200)
	for _, c := range clients[:2] {
		c.command(current(c), "ready", nil, 200)
	}
	host.command(current(host), "start", nil, 200)
	g := s.rooms[id].Game.Catan
	if g.Two.Variants != "" || g.FriendlyRobber != nil || g.Harbors != nil || g.CitiesKnights != nil || g.Fishing != nil || len(g.DevDeck) != 25 || len(g.Bank) != 5 {
		t.Fatal("base recipe contaminated")
	}
}
