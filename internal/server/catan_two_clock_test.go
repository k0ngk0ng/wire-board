package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

// Clock-only fixture, not a public two-player lobby or complete HTTP game.
func TestCatanTwoResponseClockPauseAndRestore(t *testing.T) {
	for _, resume := range []string{"catan_roll", "catan_turn", "catan_roads"} {
		t.Run(resume, func(t *testing.T) {
			now := time.Unix(2000000000, 0)
			r := Room{Status: "playing", TurnDeadline: now.Add(45 * time.Second).UnixMilli(), Game: &game.State{Kind: "catan", Turn: 0, Phase: "catan_two_build", Catan: &game.Catan{SetupStep: 4, Two: &game.CatanTwo{Sequence: 1, Pending: &game.CatanTwoPending{Kind: "road", Resume: resume}}}}}
			if !r.adjustCatanResponseClock(resume, -1, 4, now) || r.TurnDeadline != now.Add(120*time.Second).UnixMilli() || r.CatanTimeLeft != 45000 {
				t.Fatal("response did not pause action")
			}
			if !r.adjustCatanResponseClock("catan_two_build", 0, 4, now.Add(10*time.Second)) || r.TurnDeadline != now.Add(120*time.Second).UnixMilli() {
				t.Fatal("same response renewed clock")
			}
			b, _ := json.Marshal(r)
			var restored Room
			if err := json.Unmarshal(b, &restored); err != nil {
				t.Fatal(err)
			}
			r = restored
			r.Game.Catan.Two.Pending = nil
			r.Game.Phase = resume
			at := now.Add(20 * time.Second)
			if !r.adjustCatanResponseClock("catan_two_build", 0, 4, at) || r.TurnDeadline != at.Add(45*time.Second).UnixMilli() {
				t.Fatal("restored response lost remaining action time")
			}
			// A second Road Building response gets a distinct full window, then
			// returns to the action's remaining time rather than granting a turn.
			at = at.Add(time.Second)
			r.Game.Phase = "catan_two_build"
			r.Game.Catan.Two.Sequence++
			r.Game.Catan.Two.Pending = &game.CatanTwoPending{Kind: "road", Resume: resume}
			if !r.adjustCatanResponseClock(resume, -1, 4, at) || r.TurnDeadline != at.Add(120*time.Second).UnixMilli() || r.CatanTimeLeft != 44000 {
				t.Fatal("second response timing")
			}
		})
	}
}
