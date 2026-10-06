package server

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanExplorerLairsNaturalHTTPAutoplayMatch(t *testing.T) {
	s, ts, clients, id := newExplorerHTTP(t, 3, false)
	// Exact private setup constructor snapshot: synthetic token inventory,
	// but no additional hand, buildings, crew, gold or points injected.
	raw, err := os.ReadFile("testdata/catan_explorer_lairs_setup.json")
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
	for p := 0; p < 3; p++ {
		setAutoPlay(clients[p], current(clients[p]), true, 200)
	}
	for step := 0; step < 6000 && s.rooms[id].Status == "playing"; step++ {
		s.mu.Lock()
		r = s.rooms[id]
		before := r.Version
		r.BotAt = 0
		s.runBots(time.Now())
		advanced := s.rooms[id].Version > before
		s.mu.Unlock()
		if !advanced {
			t.Fatal("autoplay stopped", step, s.rooms[id].Game.Phase)
		}
		if step%47 == 0 {
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			for _, seat := range s.rooms[id].Seats {
				if !seat.AutoPlay {
					t.Fatal("restart lost autoplay")
				}
			}
			assertExplorerHTTPPrivacy(t, clients)
		}
	}
	r = s.rooms[id]
	if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.TurnDeadline != 0 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 12 {
		t.Fatal("natural autoplay match did not finish")
	}
	resolved := 0
	for _, site := range r.Game.Catan.Explorer.Lairs.Sites {
		if site.Resolved > 0 {
			resolved++
		}
	}
	if resolved == 0 {
		t.Fatal("autoplay skipped mission entirely")
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	assertExplorerHTTPPrivacy(t, clients)
	if s.rooms[id].Status != "finished" {
		t.Fatal("restart lost final result")
	}
	t.Log("round", r.Game.Round, "resolved lairs", resolved)
}
