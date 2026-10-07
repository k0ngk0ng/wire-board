package server

import (
	"encoding/json"
	"testing"
	"time"
)

// Older releases could remove an expired seat. Those saved games must remain
// readable and playable, although no current HTTP command can create them.
// Construct that historical boundary explicitly, retaining the engine's cargo,
// ownership and remaining-player assertions in the recovery scenarios.
func legacyCatanDeparture(t *testing.T, s *Server, id string, actor int) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(s.rooms[id])
	if err != nil {
		t.Fatal(err)
	}
	var next Room
	if err = json.Unmarshal(data, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.Game.EliminateCatan(actor); err != nil {
		t.Fatal(err)
	}
	next.Seats[actor].Left = true
	next.Seats[actor].AutoPlay = false
	next.Seats[actor].TimeoutAutoPlay = false
	if next.Host == next.Seats[actor].ID {
		for _, seat := range next.Seats {
			if !seat.Left && !seat.Bot {
				next.Host = seat.ID
				break
			}
		}
	}
	if next.Game.Finished {
		next.Status = "finished"
	}
	next.startTurnClock(time.Now())
	next.Version++
	if err = s.save(&next); err != nil {
		t.Fatal(err)
	}
	s.rooms[id] = &next
}
