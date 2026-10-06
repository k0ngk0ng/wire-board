package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func newExplorerSpiceHTTP(t *testing.T) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts, clients, id := newExplorerHTTP(t, 3, false)
	// Exact private three-player constructor; no artificial mission numbers,
	// injected resources or altered scores. Public room options remain closed.
	raw, err := os.ReadFile("testdata/catan_explorer_spice_setup.json")
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	r := s.rooms[id]
	r.Game = &state
	r.startTurnClock(time.Now())
	if err = s.save(r); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	return s, ts, clients, id
}

func assertExplorerSpiceHTTPPrivacy(t *testing.T, clients []*testClient, state *game.State) {
	t.Helper()
	assertExplorerFishHTTPPrivacy(t, clients, state)
	revealed := map[int]string{}
	dice := map[int]int{}
	for _, h := range state.Catan.Explorer.Board.Hidden {
		if h.Revealed && h.Farm != "" {
			revealed[h.Tile] = h.Farm
			dice[h.Tile] = h.PirateDie
		}
	}
	for _, c := range clients {
		x := current(c)["game"].(map[string]any)["catan"].(map[string]any)["explorer"].(map[string]any)
		board := x["board"].(map[string]any)
		farms, _ := board["farms"].([]any)
		if len(farms) != len(revealed) {
			t.Fatal("public farm count disagrees with discoveries")
		}
		seen := map[int]bool{}
		for _, raw := range farms {
			farm := raw.(map[string]any)
			tile := int(farm["tile"].(float64))
			die, _ := farm["pirateDie"].(float64)
			if seen[tile] || revealed[tile] != farm["ability"] || dice[tile] != int(die) {
				t.Fatal("hidden or incorrect farm disclosed")
			}
			seen[tile] = true
		}
		sacks := x["cargo"].(map[string]any)["spice"].([]any)
		if len(sacks) != 24 {
			t.Fatal("shared spice inventory missing")
		}
		for _, raw := range sacks {
			sack := raw.(map[string]any)
			origin := int(sack["origin"].(float64))
			if origin >= 0 && revealed[origin] == "" {
				t.Fatal("sack exposed hidden farm")
			}
		}
		if x["lairs"] != nil || x["spice"] == nil {
			t.Fatal("wrong missions in spice view")
		}
	}
}

func TestCatanExplorerSpiceNaturalHTTPAutoplayMatch(t *testing.T) {
	s, ts, clients, id := newExplorerSpiceHTTP(t)
	r := s.rooms[id]
	for p := 0; p < 3; p++ {
		setAutoPlay(clients[p], current(clients[p]), true, 200)
	}
	rolls, deliveries := 0, 0
	loaded := false
	for step := 0; step < 9000 && s.rooms[id].Status == "playing"; step++ {
		s.mu.Lock()
		r = s.rooms[id]
		before, serial, phase, deadline := r.Version, r.Game.Catan.TurnSerial, r.Game.Phase, r.TurnDeadline
		oldRoll := uint64(0)
		if r.Game.Catan.Explorer.Fish.LastRoll != nil {
			oldRoll = r.Game.Catan.Explorer.Fish.LastRoll.Sequence
		}
		r.BotAt = 0
		s.runBots(time.Now())
		r = s.rooms[id]
		advanced := r.Version > before
		if roll := r.Game.Catan.Explorer.Fish.LastRoll; roll != nil && roll.Sequence != oldRoll {
			rolls++
		}
		for _, loc := range r.Game.Catan.Explorer.Cargo.Fish {
			loaded = loaded || loc.Kind == "ship"
		}
		deliveries = len(r.Game.Catan.Explorer.Fish.Deliveries)
		if phase == "catan_explorer_move" && r.Game.Phase == phase && r.Game.Catan.TurnSerial == serial && r.TurnDeadline != deadline {
			t.Fatal("spice/fish action refreshed movement clock")
		}
		s.mu.Unlock()
		if !advanced {
			t.Fatal("spice autoplay stalled", step, r.Game.Phase)
		}
		if step%47 == 0 {
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			for _, seat := range s.rooms[id].Seats {
				if !seat.AutoPlay {
					t.Fatal("restart lost autoplay")
				}
			}
			assertExplorerSpiceHTTPPrivacy(t, clients, s.rooms[id].Game)
		}
	}
	r = s.rooms[id]
	if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.TurnDeadline != 0 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 15 {
		t.Fatal("spice autoplay did not finish")
	}
	spiceDeliveries := len(r.Game.Catan.Explorer.Spice.Deliveries)
	claimed := 0
	for _, sack := range r.Game.Catan.Explorer.Cargo.Spice {
		if sack.Owner >= 0 {
			claimed++
		}
	}
	if rolls == 0 || !loaded || deliveries == 0 || spiceDeliveries == 0 || claimed == 0 {
		t.Fatal("autoplay skipped fish or spice logistics", rolls, loaded, deliveries, spiceDeliveries, claimed)
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	assertExplorerSpiceHTTPPrivacy(t, clients, s.rooms[id].Game)
	if s.rooms[id].Status != "finished" || len(s.rooms[id].Game.Catan.Explorer.Fish.Deliveries) != deliveries || len(s.rooms[id].Game.Catan.Explorer.Spice.Deliveries) != spiceDeliveries {
		t.Fatal("restart lost fish result")
	}
	t.Log("round", r.Game.Round, "fish rolls", rolls, "deliveries", deliveries, "spice deliveries", spiceDeliveries, "farms claimed", claimed)
}
