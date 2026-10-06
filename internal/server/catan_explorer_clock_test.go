package server

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanExplorerExpiredActionClockCannotGainTimeFromPirate(t *testing.T) {
	raw, err := os.ReadFile("testdata/catan_explorer_lairs_chase.json")
	if err != nil {
		t.Fatal(err)
	}
	var state game.State
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	r := Room{Game: &state, Status: "playing", TurnDeadline: now.Add(-time.Second).UnixMilli()}
	if !r.adjustCatanResponseClock("catan_explorer_move", -1, 6, now) || r.CatanTimeLeft != 0 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
		t.Fatal("late response must clamp to zero")
	}
	for step := 0; r.Game.CatanPendingActor() >= 0; step++ {
		if step > 3 {
			t.Fatal("pirate response stalled")
		}
		a, err := r.Game.BotAction(r.Game.Turn)
		if err != nil {
			t.Fatal(err)
		}
		now = now.Add(10 * time.Second)
		if err = r.applyGameAction(r.Game.Turn, a, now); err != nil {
			t.Fatal(err)
		}
	}
	if r.Game.Phase != "catan_explorer_move" || r.TurnDeadline != now.UnixMilli() || r.CatanTimeLeft != 0 {
		t.Fatal("expired action gained time")
	}
}
