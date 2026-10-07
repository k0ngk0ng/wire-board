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

// Minimal states deliberately isolate clock transitions; they are not legal
// combined games and are never passed to Apply or claimed as HTTP coverage.
func explorerCityClockRoom(t *testing.T) *Room {
	t.Helper()
	var state game.State
	if err := json.Unmarshal([]byte(`{"phase":"catan_turn","catan":{"explorer":{},"citiesKnights":{}}}`), &state); err != nil {
		t.Fatal(err)
	}
	return &Room{Game: &state, Status: "playing"}
}

func TestCatanExplorerCityResponseClockRestoresActionBudget(t *testing.T) {
	for _, phase := range []string{
		"catan_diplomacy", "catan_espionage", "catan_sabotage", "catan_wedding",
		"catan_treason_remove", "catan_treason_place", "catan_guild_dues", "catan_commercial_harbor",
		"catan_aqueduct", "catan_metropolis", "catan_knight_retreat", "catan_pillage",
		"catan_defender_reward", "catan_progress_discard", "catan_progress_end",
	} {
		t.Run(phase, func(t *testing.T) {
			r := explorerCityClockRoom(t)
			now := time.Unix(1000, 0)
			r.TurnDeadline = now.Add(37 * time.Second).UnixMilli()
			r.Game.Phase = phase
			r.Game.Catan.CitiesKnights.Pending = &game.CatanCityPending{Players: []int{1}}
			if !r.adjustCatanResponseClock("catan_turn", -1, 0, now) || r.CatanTimeLeft != 37000 || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
				t.Fatal("city response did not save action budget")
			}
			deadline := r.TurnDeadline
			now = now.Add(10 * time.Second)
			r.adjustCatanResponseClock(phase, 1, 0, now)
			if r.TurnDeadline != deadline {
				t.Fatal("same responder gained time")
			}
			r.Game.Catan.CitiesKnights.Pending.Players[0] = 2
			r.adjustCatanResponseClock(phase, 1, 0, now)
			if r.TurnDeadline != now.Add(turnLimit).UnixMilli() || r.CatanTimeLeft != 37000 {
				t.Fatal("next responder lost own window or action budget")
			}
			r.Game.Catan.CitiesKnights.Pending = nil
			r.Game.Phase = "catan_explorer_move"
			now = now.Add(20 * time.Second)
			if !r.adjustCatanResponseClock(phase, 2, 0, now) || r.TurnDeadline != now.Add(37*time.Second).UnixMilli() || r.CatanTimeLeft != 0 {
				t.Fatal("sailing did not resume saved budget")
			}
		})
	}
}

func TestCatanExplorerCityResponseClockChainedSeven(t *testing.T) {
	for _, left := range []time.Duration{-time.Second, 45 * time.Second} {
		r := explorerCityClockRoom(t)
		now := time.Unix(1000, 0)
		r.TurnDeadline = now.Add(left).UnixMilli()
		previous := "catan_roll"
		for _, current := range []string{"catan_pillage", "catan_progress_discard", "catan_discard", "catan_explorer_pirate_place", "catan_explorer_pirate_steal"} {
			r.Game.Phase = current
			if !r.adjustCatanResponseClock(previous, -1, 0, now) || r.CatanTimeLeft != max(0, left.Milliseconds()) || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
				t.Fatal("chain lost original clock", previous, current)
			}
			if current == "catan_discard" {
				deadline := r.TurnDeadline
				r.adjustCatanResponseClock(current, 1, 0, now.Add(time.Second))
				if r.TurnDeadline != deadline {
					t.Fatal("simultaneous discarders gained a new deadline")
				}
			}
			previous = current
			now = now.Add(30 * time.Second)
		}
		r.Game.Phase = "catan_turn"
		if !r.adjustCatanResponseClock(previous, -1, 0, now) || r.TurnDeadline != now.Add(max(0, left)).UnixMilli() || r.CatanTimeLeft != 0 {
			t.Fatal("chain exit gained time")
		}
	}
}
