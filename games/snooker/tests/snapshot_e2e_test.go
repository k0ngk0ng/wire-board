package snooker_test

// Before snapshot hardening: corrupt trajectories or foul restores are exposed
// as trusted saved games; impossible phases and inconsistent team wins restore.
import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"testing"
)

func TestSnookerRejectsInconsistentSavedState(t *testing.T) {
	mod, ctx, m := begin(t, 4)
	mutations := map[string]func(*game.State){
		"missing colour during reds": func(s *game.State) { s.Balls[19].Down = true },
		"red target without reds": func(s *game.State) {
			for i := 1; i <= 15; i++ {
				s.Balls[i].Down = true
			}
		},
		"claimed frame wins without results": func(s *game.State) { s.FramesWon[0] = 1 },
		"zero miss sequence":                 func(s *game.State) { s.MissRun = &game.MissSequence{Offender: 0, Count: 0} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			s := state(t, m)
			mutate(&s)
			bad := m
			bad.Data, _ = json.Marshal(s)
			if _, e := mod.View(bad, 0); e == nil {
				t.Fatal("accepted inconsistent snapshot")
			}
		})
	}
	moving := action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": 0, "power": .06})
	for name, mutate := range map[string]func(*game.State){
		"future ball position": func(s *game.State) { s.Flight.Motion.Balls[0].Pos.X += .1 },
		"first collision":      func(s *game.State) { s.Flight.Motion.First = []int{21} },
		"stroke owner":         func(s *game.State) { s.Flight.Seat = 2 },
		"negative duration":    func(s *game.State) { s.Flight.Motion.Duration = -1 },
		"event beyond duration": func(s *game.State) {
			s.Flight.Motion.Events = append(s.Flight.Motion.Events, game.Impact{Time: 100, Kind: "pot", Ball: 21})
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := state(t, moving)
			mutate(&s)
			bad := moving
			bad.Data, _ = json.Marshal(s)
			if _, e := mod.View(bad, 0); e == nil {
				t.Fatal("accepted modified stored stroke")
			}
		})
	}
}
