package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/game"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func newExplorerFullHTTP(t *testing.T) (*Server, *httptest.Server, []*testClient, string) {
	t.Helper()
	s, ts, clients, id := newExplorerHTTP(t, 3, false)
	// Normal private full constructor with explicit artificial lair numbers;
	// no injected resources or scores. Public room options remain closed.
	raw, err := os.ReadFile("testdata/catan_explorer_full_setup.json")
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

func TestCatanExplorerFullNaturalHTTPAutoplayMatch(t *testing.T) {
	s, ts, clients, id := newExplorerFullHTTP(t)
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
			t.Fatal("full mission autoplay stalled", step, r.Game.Phase)
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
	if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.TurnDeadline != 0 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 17 {
		t.Fatal("full mission autoplay did not finish")
	}
	resolved := 0
	for _, site := range r.Game.Catan.Explorer.Lairs.Sites {
		if site.Resolved > 0 {
			resolved++
		}
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
	t.Log("round", r.Game.Round, "fish rolls", rolls, "deliveries", deliveries, "spice deliveries", spiceDeliveries, "farms claimed", claimed, "resolved lairs", resolved)
}
