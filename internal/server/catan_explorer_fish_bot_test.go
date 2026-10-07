package server

import (
	"github.com/k0ngk0ng/wire-board/internal/game"
	"testing"
	"time"
)

func assertExplorerFishHTTPPrivacy(t *testing.T, clients []*testClient, state *game.State) {
	t.Helper()
	assertExplorerHTTPPrivacy(t, clients)
	revealed := map[int]int{}
	for _, h := range state.Catan.Explorer.Board.Hidden {
		if h.Revealed && h.Fish > 0 {
			revealed[h.Tile] = h.Fish
		}
	}
	for _, c := range clients {
		x := current(c)["game"].(map[string]any)["catan"].(map[string]any)["explorer"].(map[string]any)
		board := x["board"].(map[string]any)
		shoals, _ := board["shoals"].([]any)
		if len(shoals) != len(revealed) {
			t.Fatal("public shoal count disagrees with discoveries")
		}
		seen := map[int]bool{}
		for _, raw := range shoals {
			h := raw.(map[string]any)
			tile, face := int(h["tile"].(float64)), int(h["number"].(float64))
			if seen[tile] || revealed[tile] != face {
				t.Fatal("hidden or incorrect fish face disclosed")
			}
			seen[tile] = true
		}
		wantFish := 6
		if len(state.Catan.Players) > 4 {
			wantFish = 8
		}
		if len(x["cargo"].(map[string]any)["fish"].([]any)) != wantFish {
			t.Fatal("shared fish inventory missing")
		}
	}
}

func TestCatanExplorerFishNaturalHTTPAutoplayMatch(t *testing.T) {
	s, ts, clients, id := newExplorerFishHTTP(t)
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
			t.Fatal("fish action refreshed movement clock")
		}
		s.mu.Unlock()
		if !advanced {
			t.Fatal("fish autoplay stalled", step, r.Game.Phase)
		}
		if step%47 == 0 {
			s, ts = restartRiversHTTP(t, s, ts, clients, id)
			for _, seat := range s.rooms[id].Seats {
				if !seat.AutoPlay {
					t.Fatal("restart lost autoplay")
				}
			}
			assertExplorerFishHTTPPrivacy(t, clients, s.rooms[id].Game)
		}
	}
	r = s.rooms[id]
	if !r.Game.Finished || r.Status != "finished" || len(r.Game.Winners) != 1 || r.TurnDeadline != 0 || r.Game.Catan.Players[r.Game.Winners[0]].Score < 15 {
		t.Fatal("fish autoplay did not finish")
	}
	resolved := 0
	for _, site := range r.Game.Catan.Explorer.Lairs.Sites {
		if site.Resolved > 0 {
			resolved++
		}
	}
	if rolls == 0 || !loaded || deliveries == 0 || resolved == 0 {
		t.Fatal("autoplay skipped fish or lair logistics", rolls, loaded, deliveries, resolved)
	}
	s, ts = restartRiversHTTP(t, s, ts, clients, id)
	assertExplorerFishHTTPPrivacy(t, clients, s.rooms[id].Game)
	if s.rooms[id].Status != "finished" || len(s.rooms[id].Game.Catan.Explorer.Fish.Deliveries) != deliveries {
		t.Fatal("restart lost fish result")
	}
	t.Log("round", r.Game.Round, "fish rolls", rolls, "deliveries", deliveries, "resolved lairs", resolved)
}
