package snooker_test

// Physical and match-boundary scenarios, specified before the remaining referee
// changes. Seeded layouts are labelled; strokes still use production physics.
import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"testing"
	"time"
)

func seedSpecial(t *testing.T, m gameapi.Match, kind string) gameapi.Match {
	s := state(t, m)
	s.InHand = false
	for i := range s.Balls {
		if s.Balls[i].ID >= 2 && s.Balls[i].ID <= 15 {
			s.Balls[i].Down = true
		}
	}
	switch kind {
	case "touch_return":
		s.Balls[0].Pos = game.Vec{X: .4, Y: .8}
		s.Balls[1].Pos = game.Vec{X: .4 + 2*game.BallRadius, Y: .8}
	case "clear_miss":
		s.Balls[0].Pos = game.Vec{X: .5, Y: .5}
		s.Balls[1].Pos = game.Vec{X: 1.5, Y: .5}
	case "bank_miss":
		s.Balls[0].Pos = game.Vec{X: .5, Y: .5}
		s.Balls[1].Pos = game.Vec{X: 1.5, Y: .5}
		s.Balls[21].Pos = game.Vec{X: 1, Y: .5}
	case "inhand_free":
		s.Balls[0].Down = true
		s.Balls[1].Pos = game.Vec{X: 3, Y: game.TableHeight / 2}
		for i := 16; i <= 21; i++ {
			s.Balls[i].Pos = game.Vec{X: 1.25, Y: game.TableHeight/2 + (float64(i-16)-2.5)*.075}
		}
		s.InHand = true
		s.Phase = "foul"
		s.Scores = [2]int{4, 0}
		s.Foul = &game.Foul{Offender: 1, OffenderIndex: 1, Next: 0, Ruling: game.Ruling{Foul: 4}}
	}
	s.Logs = append(s.Logs, "明确球位夹具："+kind+"，不代表自然形成的完整对局")
	m.Data, _ = json.Marshal(s)
	return m
}
func TestSnookerTouchingOnBallMayBeHitLaterAfterPlayingAway(t *testing.T) {
	mod, ctx, m := begin(t, 2)
	m = seedSpecial(t, m, "touch_return")
	before := m
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": math.Pi, "power": .7})
	flight := state(t, m).Flight
	late := false
	for _, event := range flight.Motion.Events {
		if event.Kind == "ball" && ((event.Ball == 0 && event.Other == 1) || (event.Ball == 1 && event.Other == 0)) && event.Time > .2 {
			late = true
		}
	}
	if !late {
		t.Fatal("fixture did not make the cue return to the touching red")
	}
	m = finishStroke(t, mod, ctx, m)
	if state(t, m).Phase == "foul" {
		t.Fatal("later ball movement was incorrectly adjudicated as a push", state(t, m).Foul)
	}
	trace(t, before, m)
}
func TestSnookerMissAlsoChecksFeasibleCushionEscape(t *testing.T) {
	mod, ctx, m := begin(t, 2)
	m = seedSpecial(t, m, "bank_miss")
	before := m
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": math.Pi / 2, "power": .06})
	m = finishStroke(t, mod, ctx, m)
	s := state(t, m)
	if s.Foul == nil || !s.Foul.Ruling.Miss {
		t.Fatal("ignored a feasible bank escape in a clearly insufficient attempt", s.Foul)
	}
	ctx.Actor = s.Turn
	ctx.Now = ctx.Now.Add(time.Minute)
	m = action(t, mod, ctx, m, map[string]any{"type": "foul_choice", "choice": "restore"})
	trace(t, before, m)
}
func TestSnookerInHandFreeBallExaminesTheWholeD(t *testing.T) {
	mod, ctx, m := begin(t, 2)
	m = seedSpecial(t, m, "inhand_free")
	m = action(t, mod, ctx, m, map[string]any{"type": "foul_choice", "choice": "accept"})
	if !state(t, m).FreeBall {
		t.Fatal("white in hand automatically suppressed free-ball eligibility")
	}
	trace(t, m)
}
func TestSnookerThreeMissesWarnBeforeAwardingDoublesFrame(t *testing.T) {
	mod, ctx, m := begin(t, 4)
	m = seedSpecial(t, m, "clear_miss")
	s := state(t, m)
	s.Turn = 2
	s.TurnIndex = 2
	m.Data, _ = json.Marshal(s)
	records := []any{}
	for attempt := 1; attempt <= 3; attempt++ {
		ctx.Actor = 2
		ctx.Now = ctx.Now.Add(time.Minute)
		m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": math.Pi / 2, "power": .06})
		m = finishStroke(t, mod, ctx, m)
		s = state(t, m)
		records = append(records, s)
		if attempt < 3 {
			if s.Foul == nil || s.Foul.Offender != 2 || s.Turn != 3 {
				t.Fatal("wrong offender or next double's player")
			}
			if attempt == 2 && (s.MissRun == nil || !s.MissRun.Warned) {
				t.Fatal("no second-miss warning")
			}
			ctx.Actor = 3
			m = action(t, mod, ctx, m, map[string]any{"type": "foul_choice", "choice": "restore"})
		}
	}
	if m.Result == nil || state(t, m).Results[0].Reason != "three_misses" || len(m.Result.Winners) != 2 || m.Result.Winners[0] != 1 || m.Result.Winners[1] != 3 {
		t.Fatal("third warned miss did not award opponent team", m.Result)
	}
	trace(t, records)
}
func TestSnookerDoublesNextFrameAlternatesOpeningAndCanReorder(t *testing.T) {
	mod, ctx, m := begin(t, 4, 3)
	ctx.Actor = 0
	m = onlyBlack(t, m, [2]int{})
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": -math.Pi / 2, "power": .25})
	m = finishStroke(t, mod, ctx, m)
	if state(t, m).Phase != "frame_end" {
		t.Fatal("best-of-three ended after one frame")
	}
	ctx.Now = ctx.Now.Add(time.Minute)
	ctx.Actor = 2
	m = action(t, mod, ctx, m, map[string]any{"type": "lead", "seat": 2})
	ctx.Actor = 3
	m = action(t, mod, ctx, m, map[string]any{"type": "lead", "seat": 3})
	for p := 0; p < 4; p++ {
		ctx.Actor = p
		m = action(t, mod, ctx, m, map[string]any{"type": "ready"})
	}
	s := state(t, m)
	if s.Frame != 2 || s.OpeningTeam != 1 || s.Turn != 3 || s.Order[0] != 2 || s.Order[1] != 3 || s.FramesWon != [2]int{1, 0} {
		t.Fatal("new frame lost alternation or team order", s)
	}
	trace(t, m)
}
