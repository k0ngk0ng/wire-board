package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestCatanSevenResponseClockPreservesActionBudget(t *testing.T) {
	for _, remaining := range []time.Duration{0, 37 * time.Second} {
		for _, helpers := range []bool{false, true} {
			s, err := game.NewCatan(3, game.CatanOptions{Helpers: helpers})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			r := Room{Status: "playing", Game: s, TurnDeadline: now.Add(remaining).UnixMilli()}
			previous := "catan_roll"
			if helpers {
				s.Phase = "catan_helper"
				s.Catan.HelperPending = &game.CatanHelperPending{Player: 1, Kind: "exchange", Resume: "catan_discard"}
				if !r.adjustCatanResponseClock(previous, -1, 0, now) {
					t.Fatal("helper clock missing")
				}
				previous = s.Phase
				now = now.Add(15 * time.Second)
				s.Catan.HelperPending = nil
			}
			s.Phase = "catan_discard"
			if !r.adjustCatanResponseClock(previous, 1, 0, now) || r.CatanTimeLeft != remaining.Milliseconds() || r.TurnDeadline != now.Add(turnLimit).UnixMilli() {
				t.Fatal("discard did not pause original action time")
			}
			deadline := r.TurnDeadline
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var restored Room
			if err = json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			r = restored
			s = r.Game
			if !r.CatanDiscardPaused {
				t.Fatal("saved discard lost paused-budget marker")
			}
			for i := 0; i < 2; i++ {
				now = now.Add(20 * time.Second)
				if !r.adjustCatanResponseClock("catan_discard", i, 0, now) || r.TurnDeadline != deadline {
					t.Fatal("discarders did not share deadline")
				}
			}
			s.Phase = "catan_robber"
			now = now.Add(20 * time.Second)
			if !r.adjustCatanResponseClock("catan_discard", -1, 0, now) || r.TurnDeadline != now.Add(remaining).UnixMilli() {
				t.Fatal("last discard refilled turn instead of restoring remaining time")
			}
		}
	}
}

func TestCatanLegacyDiscardClockStillContinues(t *testing.T) {
	s, err := game.NewCatan(3, game.CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s.Phase = "catan_robber"
	now := time.Now()
	r := Room{Status: "playing", Game: s}
	if !r.adjustCatanResponseClock("catan_discard", -1, 0, now) || r.TurnDeadline != now.Add(turnLimit).UnixMilli() || r.CatanDiscardPaused {
		t.Fatal("legacy discard without saved action budget cannot resume")
	}
}
