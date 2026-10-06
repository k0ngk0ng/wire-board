package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanTransportResponseClock(t *testing.T) {
	// Clock-only fixture: no claim of a valid playable board or HTTP match.
	var state game.State
	if e := json.Unmarshal([]byte(`{"kind":"catan","phase":"catan_turn","turn":0,"catan":{"transport":{}}}`), &state); e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1000, 0)
	r := &Room{Status: "playing", Game: &state, TurnDeadline: now.Add(37 * time.Second).UnixMilli()}
	r.Game.Phase = "catan_transport_barbarian"
	if !r.adjustCatanResponseClock("catan_turn", -1, 0, now) || r.CatanTimeLeft != 37000 || r.TurnDeadline != now.Add(120*time.Second).UnixMilli() {
		t.Fatal("barbarian response must pause remaining turn and grant 120s")
	}
	r.Game.Phase = "catan_turn"
	later := now.Add(13 * time.Second)
	if !r.adjustCatanResponseClock("catan_transport_barbarian", 0, 0, later) || r.TurnDeadline != later.Add(37*time.Second).UnixMilli() {
		t.Fatal("barbarian choice lost remaining turn")
	}
	r.Game.Phase = "catan_transport_move"
	if !r.adjustCatanResponseClock("catan_turn", -1, 0, later) || r.TurnDeadline != later.Add(120*time.Second).UnixMilli() {
		t.Fatal("end movement gets separate 120s")
	}
	deadline := r.TurnDeadline
	for step := 1; step <= 5; step++ {
		// Moving, driving, relocating, arriving and Swift Journey all share this
		// same end-of-turn window; none refreshes it per click/choice/sequence.
		r.Game.Catan.Transport.Sequence = uint64(step)
		if !r.adjustCatanResponseClock("catan_transport_move", 0, 0, later.Add(time.Duration(step)*time.Second)) || r.TurnDeadline != deadline {
			t.Fatal("movement refreshed deadline", step)
		}
	}
	data, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	var restored Room
	if e = json.Unmarshal(data, &restored); e != nil {
		t.Fatal(e)
	}
	if restored.TurnDeadline != deadline || restored.CatanTimeLeft != 37000 {
		t.Fatal("room save lost deadline")
	}
	restored.Game.Phase = "catan_roll"
	restored.Game.Turn = 1
	end := later.Add(25 * time.Second)
	if !restored.adjustCatanResponseClock("catan_transport_move", 0, 0, end) || restored.CatanTimeLeft != 0 || restored.TurnDeadline != end.Add(120*time.Second).UnixMilli() {
		t.Fatal("next player must get full turn, not resume previous player's time")
	}
}
