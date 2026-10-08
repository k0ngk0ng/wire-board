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

func TestCatanTwoTradeClockPauseAndRestore(t *testing.T) {
	for _, resume := range []string{"catan_roll", "catan_turn"} {
		t.Run(resume, func(t *testing.T) {
			now := time.Unix(2000000000, 0)
			r := Room{Status: "playing", TurnDeadline: now.Add(45 * time.Second).UnixMilli(), Game: &game.State{Kind: "catan", Turn: 1, Phase: "catan_two_trade", Catan: &game.Catan{SetupStep: 4, Two: &game.CatanTwo{Sequence: 1, Spent: true, Trade: &game.CatanTwoTrade{Resume: resume, Drawn: []int{0, 1, 0, 0, 1}}}}}}
			if !r.adjustCatanResponseClock(resume, -1, 4, now) || r.TurnDeadline != now.Add(120*time.Second).UnixMilli() || r.CatanTimeLeft != 45000 || r.Game.CatanPendingActor() != 1 {
				t.Fatal("trade choice did not pause original clock")
			}
			b, _ := json.Marshal(r)
			var restored Room
			if err := json.Unmarshal(b, &restored); err != nil {
				t.Fatal(err)
			}
			r = restored
			if !r.adjustCatanResponseClock("catan_two_trade", 1, 4, now.Add(30*time.Second)) || r.TurnDeadline != now.Add(120*time.Second).UnixMilli() {
				t.Fatal("restore renewed trade response")
			}
			r.Game.Catan.Two.Trade = nil
			r.Game.Phase = resume
			at := now.Add(120 * time.Second)
			if !r.adjustCatanResponseClock("catan_two_trade", 1, 4, at) || r.TurnDeadline != at.Add(45*time.Second).UnixMilli() || r.Game.CatanPendingActor() != -1 {
				t.Fatal("trade response did not restore remaining time")
			}
		})
	}
}

func TestCatanTwoHelpersDeferredBuildClock(t *testing.T) {
	now := time.Unix(2000000000, 0)
	r := Room{Status: "playing", TurnDeadline: now.Add(45 * time.Second).UnixMilli(), Game: &game.State{Kind: "catan", Turn: 0, Phase: "catan_helper", Catan: &game.Catan{SetupStep: 4, Two: &game.CatanTwo{AfterHelper: "road"}, HelperPending: &game.CatanHelperPending{Player: 0, Kind: "exchange", Resume: "catan_turn"}}}}
	if !r.adjustCatanResponseClock("catan_turn", -1, 4, now) || r.CatanTimeLeft != 45000 || r.TurnDeadline != now.Add(120*time.Second).UnixMilli() {
		t.Fatal("helper did not pause")
	}
	raw, _ := json.Marshal(r)
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	at := now.Add(30 * time.Second)
	r.Game.Catan.HelperPending = nil
	r.Game.Catan.Two.AfterHelper = ""
	r.Game.Catan.Two.Pending = &game.CatanTwoPending{Kind: "road", Resume: "catan_turn"}
	r.Game.Phase = "catan_two_build"
	if !r.adjustCatanResponseClock("catan_helper", 0, 4, at) || r.CatanTimeLeft != 45000 || r.TurnDeadline != at.Add(120*time.Second).UnixMilli() {
		t.Fatal("neutral response lost action budget")
	}
	raw, _ = json.Marshal(r)
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	at = at.Add(10 * time.Second)
	r.Game.Catan.Two.Pending = nil
	r.Game.Phase = "catan_turn"
	if !r.adjustCatanResponseClock("catan_two_build", 0, 4, at) || r.TurnDeadline != at.Add(45*time.Second).UnixMilli() {
		t.Fatal("chain did not restore original budget")
	}
}
