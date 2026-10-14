package snooker_test

// Authored before the snooker Module implementation. These tests use its real
// command/clock/view boundary; deterministic ball layouts are explicitly marked.
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/snooker/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func state(t *testing.T, m gameapi.Match) game.State {
	t.Helper()
	var s game.State
	if e := json.Unmarshal(m.Data, &s); e != nil {
		t.Fatal(e)
	}
	return s
}
func action(t *testing.T, mod gameapi.Module, ctx gameapi.Context, m gameapi.Match, a map[string]any) gameapi.Match {
	t.Helper()
	a["window"] = state(t, m).Window
	raw, _ := json.Marshal(a)
	next, e := mod.Apply(ctx, m, raw)
	if e != nil {
		t.Fatalf("%v: %v", a, e)
	}
	if e = mod.ValidateMatch(next, ctx.Players); e != nil {
		t.Fatal(e)
	}
	return next
}
func begin(t *testing.T, n int, series ...int) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	mod := game.Module()
	ctx := gameapi.Context{Players: n, Now: time.Unix(100000, 0)}
	var opts json.RawMessage
	if len(series) > 0 {
		opts = mustAction(map[string]any{"bestOf": series[0]})
	}
	m, e := mod.Start(ctx, opts)
	if e != nil {
		t.Fatal(e)
	}
	// Rule fixtures use a fixed opener; production random opening has separate
	// real HTTP coverage. This does not send a removed break-choice command.
	s := state(t, m)
	s.Phase = "aim"
	s.OpeningTeam = 0
	s.OpeningSeat = 0
	s.Turn = 0
	s.TurnIndex = 0
	s.Due = make([]int64, n)
	s.Due[0] = ctx.Now.Add(120 * time.Second).UnixMilli()
	m.Data, _ = json.Marshal(s)
	m.Actors = make([]int, n)
	for p := range m.Actors {
		m.Actors[p] = p
	}
	m.Attention = []int{0}
	m.TimeoutActors = []int{0}
	m.Deadline = s.Due[0]
	ctx.Actor = 0
	return mod, ctx, m
}
func finishStroke(t *testing.T, mod gameapi.Module, ctx gameapi.Context, m gameapi.Match) gameapi.Match {
	t.Helper()
	if m.SystemDeadline <= ctx.Now.UnixMilli() || len(m.Actors) != 0 {
		t.Fatal("stroke not exclusively timed")
	}
	ctx.Now = time.UnixMilli(m.SystemDeadline)
	ctx.Actor = -1
	next, e := mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	if e = mod.ValidateMatch(next, ctx.Players); e != nil {
		t.Fatal(e)
	}
	return next
}
func trace(t *testing.T, records ...any) {
	t.Helper()
	path := filepath.Join("../../../.local/snooker/verification", t.Name()+".json")
	if e := os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "test": t.Name(), "records": records}, "", "  ")
	if e := os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestSnookerRealModuleShotAndClockLifecycle(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			mod, ctx, m := begin(t, n)
			s := state(t, m)
			if len(s.Balls) != 22 || s.Phase != "aim" || s.Turn != 0 || !s.InHand || m.Deadline != ctx.Now.Add(120*time.Second).UnixMilli() {
				t.Fatal("incorrect standard start", s)
			}
			if _, e := mod.Configure(3, nil); e == nil {
				t.Fatal("three-player snooker allowed")
			}
			// A partner/opponent/spectator cannot submit the active player's stroke.
			for who := -1; who < n; who++ {
				if who == s.Turn {
					continue
				}
				bad := ctx
				bad.Actor = who
				b, _ := json.Marshal(map[string]any{"type": "shoot", "window": s.Window, "angle": 0, "power": .4})
				got, e := mod.Apply(bad, m, b)
				if e == nil || !reflect.DeepEqual(got, m) {
					t.Fatal("unauthorised stroke mutated state")
				}
			}
			before := m
			m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": 0.0, "power": .4})
			moving := state(t, m)
			if moving.Phase != "rolling" || moving.Scores != s.Scores {
				t.Fatal("score changed before impact timeline")
			}
			early := ctx
			early.Actor = -1
			early.Now = time.UnixMilli(m.SystemDeadline - 1)
			unchanged, e := mod.OnTimer(early, m)
			if e != nil || !reflect.DeepEqual(unchanged, m) {
				t.Fatal("timer settled early")
			}
			settled := finishStroke(t, mod, ctx, m)
			ss := state(t, settled)
			if ss.Phase == "rolling" {
				t.Fatal("motion never settled")
			}
			again, e := mod.OnTimer(gameapi.Context{Players: n, Actor: -1, Now: time.UnixMilli(m.SystemDeadline + 1000)}, settled)
			if e != nil || !reflect.DeepEqual(again, settled) {
				t.Fatal("motion settled twice")
			}
			for p := -1; p < n; p++ {
				if _, e := mod.View(settled, p); e != nil {
					t.Fatal("view not restorable", p, e)
				}
			}
			trace(t, before, m, settled)
		})
	}
}
func onlyBlack(t *testing.T, m gameapi.Match, scores [2]int) gameapi.Match {
	t.Helper()
	s := state(t, m)
	for i := range s.Balls {
		s.Balls[i].Down = s.Balls[i].ID != 0 && s.Balls[i].ID != 21
	}
	s.Balls[0].Pos = game.Vec{X: game.TableWidth / 2, Y: .6}
	s.Balls[21].Pos = game.Vec{X: game.TableWidth / 2, Y: .2}
	s.Target = "clear"
	s.Clear = 7
	s.Scores = scores
	s.InHand = false
	s.Logs = append(s.Logs, "验收定点：只剩黑球，非自然形成的整局")
	m.Data, _ = json.Marshal(s)
	return m
}
func TestSnookerBlackPotEndsFrameAndTeamsShareVictory(t *testing.T) {
	for _, n := range []int{2, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			mod, ctx, m := begin(t, n)
			m = onlyBlack(t, m, [2]int{0, 0})
			before := m
			m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": -1.5707963267948966, "power": .25})
			m = finishStroke(t, mod, ctx, m)
			if m.Result == nil || m.Result.Team != (n == 4) {
				t.Fatal("match did not finish", state(t, m))
			}
			for p := 0; p < n; p++ {
				expected := n/2 + 1
				if p%2 == 0 {
					expected = 1
				}
				if m.Result.Ranks[p] != expected {
					t.Fatal("wrong team result", m.Result)
				}
			}
			trace(t, before, m)
		})
	}
}
func TestSnookerTiedBlackRequiresChoiceAndDPlacement(t *testing.T) {
	mod, ctx, m := begin(t, 4)
	m = onlyBlack(t, m, [2]int{0, 7})
	m = action(t, mod, ctx, m, map[string]any{"type": "shoot", "angle": -1.5707963267948966, "power": .25})
	m = finishStroke(t, mod, ctx, m)
	s := state(t, m)
	if m.Result != nil || s.Phase != "black_choice" || s.Scores != [2]int{7, 7} {
		t.Fatal("tied black did not respot", s)
	}
	ctx.Now = time.UnixMilli(m.SystemDeadline + 100000001)
	ctx.Actor = m.Actors[0]
	m = action(t, mod, ctx, m, map[string]any{"type": "black_choice", "team": 0})
	ctx.Actor = m.Actors[0]
	m = action(t, mod, ctx, m, map[string]any{"type": "black_player", "seat": 2})
	s = state(t, m)
	if s.Turn != 2 || !s.InHand || s.Phase != "aim" || s.Balls[21].Down {
		t.Fatal("black choice lost selected teammate", s)
	}
	ctx.Actor = 2
	raw, _ := json.Marshal(map[string]any{"type": "place", "window": s.Window, "x": 2., "y": .8})
	bad, e := mod.Apply(ctx, m, raw)
	if e == nil || !reflect.DeepEqual(bad, m) {
		t.Fatal("placed white outside D")
	}
	m = action(t, mod, ctx, m, map[string]any{"type": "place", "x": game.BaulkX - .15, "y": game.TableHeight / 2})
	trace(t, m)
}
