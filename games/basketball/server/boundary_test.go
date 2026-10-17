package basketball_test

// These isolated host-boundary regressions cover newly identified failure
// modes before their fixes: remapping another controller's Start into player
// one's global control, dropping short taps, cancellation leaking across seats,
// unbounded catch-up, and entering original menus after a confirmed result.
import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func hostState(t *testing.T, match gameapi.Match) game.State {
	t.Helper()
	var s game.State
	if err := json.Unmarshal(match.Data, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
func hostStart(t *testing.T) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	module := game.Module()
	ctx := gameapi.Context{Players: 4, Actor: 0, Now: time.UnixMilli(1000000), Automated: make([]bool, 4)}
	m, err := module.Start(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	return module, ctx, m
}
func hostTick(t *testing.T, module gameapi.Module, ctx *gameapi.Context, m gameapi.Match) gameapi.Match {
	t.Helper()
	ctx.Actor = -1
	ctx.Now = ctx.Now.Add(100 * time.Millisecond)
	next, err := module.OnTimer(*ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	if err = module.ValidateMatch(next, 4); err != nil {
		t.Fatal(err)
	}
	return next
}
func hostButtons(t *testing.T, module gameapi.Module, ctx gameapi.Context, m gameapi.Match, seat int, kind string, held engine.Buttons) gameapi.Match {
	t.Helper()
	s := hostState(t, m)
	ctx.Actor = seat
	raw, _ := json.Marshal(map[string]any{"type": kind, "epoch": s.Epoch, "sequence": s.Inputs[seat].Sequence + 1, "buttons": held})
	next, err := module.Apply(ctx, m, raw)
	if err != nil {
		t.Fatal(err)
	}
	return next
}
func hostPlaying(t *testing.T) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	module, ctx, m := hostStart(t)
	for i := 0; i < 50 && hostState(t, m).Runtime.Playing == nil; i++ {
		m = hostTick(t, module, &ctx, m)
	}
	if hostState(t, m).Runtime.Playing == nil {
		t.Fatal("original opening did not finish")
	}
	return module, ctx, m
}

func TestHostPreservesOriginalStartController(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	m = hostButtons(t, module, ctx, m, 2, "buttons", engine.ButtonStart)
	m = hostTick(t, module, &ctx, m)
	if hostState(t, m).Runtime.Playing.Match.Phase != engine.MatchPlaying {
		t.Fatal("seat 3 Start was incorrectly rewritten into controller 0")
	}
	m = hostButtons(t, module, ctx, m, 0, "buttons", engine.ButtonStart)
	m = hostTick(t, module, &ctx, m)
	if hostState(t, m).Runtime.Playing.Match.Phase != engine.MatchPaused {
		t.Fatal("original controller 0 cannot pause")
	}
}

func TestHostShortTapCancellationAndCatchup(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	m = hostButtons(t, module, ctx, m, 0, "buttons", engine.ButtonB)
	m = hostButtons(t, module, ctx, m, 0, "buttons", 0)
	if hostState(t, m).Inputs[0].Pending&engine.ButtonB == 0 {
		t.Fatal("network short tap was dropped")
	}
	m = hostButtons(t, module, ctx, m, 1, "buttons", engine.ButtonRight)
	m = hostButtons(t, module, ctx, m, 0, "release", 0)
	s := hostState(t, m)
	if s.Inputs[0].Pending != 0 || s.Inputs[1].Buttons != engine.ButtonRight {
		t.Fatal("cancel changed another seat or retained old tap")
	}
	m = hostTick(t, module, &ctx, m)
	s = hostState(t, m)
	if s.Runtime.Playing.World.Preparation.Actions.Actors[0].Player.Action != engine.ActionIdle {
		t.Fatal("cancelled tap became an action")
	}
	m = hostButtons(t, module, ctx, m, 0, "buttons", engine.ButtonB)
	m = hostButtons(t, module, ctx, m, 0, "buttons", 0)
	acted := false
	for i := 0; i < 4; i++ {
		m = hostTick(t, module, &ctx, m)
		acted = acted || hostState(t, m).Runtime.Playing.World.Preparation.Actions.Actors[0].Player.Action != engine.ActionIdle
	}
	if !acted {
		t.Fatal("short tap never reached original action logic")
	}
	before := hostState(t, m)
	ctx.Now = ctx.Now.Add(time.Hour)
	m = hostTick(t, module, &ctx, m)
	after := hostState(t, m)
	if after.Runtime.VideoFrames-before.Runtime.VideoFrames > 15 || after.Epoch != before.Epoch {
		t.Fatal("host outage rewound or fast-forwarded the whole game")
	}
	if after.Inputs[1].Buttons != 0 {
		t.Fatal("lost transport left a button held")
	}
}

func TestHostPreservesTwoTurboEdgesWithinOneFrame(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	expected := hostState(t, m).Runtime
	base := ctx.Now
	for _, event := range []struct {
		at   time.Duration
		held engine.Buttons
	}{{10 * time.Millisecond, engine.ButtonB}, {20 * time.Millisecond, 0}, {70 * time.Millisecond, engine.ButtonB}, {80 * time.Millisecond, 0}} {
		ctx.Now = base.Add(event.at)
		m = hostButtons(t, module, ctx, m, 0, "buttons", event.held)
	}
	// Independent video input sequence: a held edge at the first sample, and
	// the second short tap latched at the fifth sample, before the sixth logic
	// sample. Collapsing all packets to one pending bit loses the second tap.
	for _, held := range []engine.Buttons{engine.ButtonB, 0, 0, 0, engine.ButtonB, 0} {
		r, err := engine.StepVideo(expected, [4]engine.Buttons{held})
		if err != nil {
			t.Fatal(err)
		}
		expected = r.State
	}
	ctx.Now = base
	m = hostTick(t, module, &ctx, m)
	actual := hostState(t, m).Runtime
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("network frame collapsed distinct turbo taps: memory got=%+v want=%+v", actual.Playing.World.Input.Memory[0], expected.Playing.World.Input.Memory[0])
	}
}

func TestHostReclaimDoesNotConsumeAnotherSeatPendingHandoff(t *testing.T) {
	module, ctx, m := hostStart(t)
	ctx.Automated[1] = true
	m = hostTick(t, module, &ctx, m)
	m = hostButtons(t, module, ctx, m, 2, "buttons", engine.ButtonRight)
	before := hostState(t, m)
	// Seat 3 has just requested autoplay at the host, but its ordinary frame
	// has not consumed that request yet. Reclaiming seat 2 is not that frame.
	ctx.Actor = 1
	ctx.Automated[2] = true
	next, err := module.Reclaim(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	after := hostState(t, next)
	if !reflect.DeepEqual(after.Inputs[2], before.Inputs[2]) || after.Runtime.Config.Match.ControllerAssignments[2] != before.Runtime.Config.Match.ControllerAssignments[2] {
		t.Fatal("reclaim consumed another seat's pending control change")
	}
	if after.Inputs[1].Automated || after.Runtime.Config.Match.ControllerAssignments[1] != 1 {
		t.Fatal("reclaim failed for its own seat")
	}
}

func TestHostConfirmedResultAndTeamMapping(t *testing.T) {
	module, ctx, m := hostStart(t)
	s := hostState(t, m)
	// Explicit result-boundary fixture, not a natural complete-match claim.
	o := *s.Runtime.Opening
	o.Transition.Match.Phase = 6
	o.Transition.Match.Period = 2
	o.Transition.Scores = [2]uint16{12, 3}
	o.Transition.Match.DisplayedScores = [2][3]uint8{{0, 1, 2}, {0, 0, 3}}
	o.Transition.CameraScroll = [4]uint16{128, 297, 128, 297}
	for i := range o.Transition.Objects {
		o.Transition.Objects[i].Pose = engine.Pose{Frame: 127}
	}
	for i := 0; i < 12; i++ {
		o.Projection.DrawOrder[i] = 255
	}
	o.Projection.DrawCount = 0
	s.Runtime.Opening = nil
	s.Runtime.Transition = &engine.RuntimeTransitionState{Scene: o}
	s.Runtime.LogicSteps = 1
	var err error
	m.Data, err = json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var resultViews []any
	for i := 0; i < 100 && m.Result == nil; i++ {
		v, e := module.View(m, 0)
		if e != nil {
			t.Fatal(e)
		}
		resultViews = append(resultViews, v)
		if len(resultViews) > 8 {
			resultViews = resultViews[len(resultViews)-8:]
		}
		m = hostTick(t, module, &ctx, m)
	}
	if m.Result == nil || !m.Result.Team || !reflect.DeepEqual(m.Result.Winners, []int{0, 1}) || !reflect.DeepEqual(m.Result.Ranks, []int{1, 1, 2, 2}) || m.SystemDeadline != 0 {
		t.Fatal("original two-team outcome was not projected", m.Result)
	}
	s = hostState(t, m)
	if !s.Runtime.Outcome.Confirmed || s.Runtime.Display.Visible.Result == nil {
		t.Fatal("result did not retain original presentation")
	}
	v, e := module.View(m, 0)
	if e != nil {
		t.Fatal(e)
	}
	resultViews = append(resultViews, v)
	writeVideoFixture(t, "host-result-views.json", map[string]any{"scope": "explicit original result-boundary fixture, not a natural complete match", "views": resultViews})
	stable := hostTick(t, module, &ctx, m)
	if !reflect.DeepEqual(stable, m) {
		t.Fatal("completed host session advanced into original menu")
	}
	restored, err := module.Restore(ctx, m)
	if err != nil || !reflect.DeepEqual(restored, m) {
		t.Fatal("restore changed completed result", err)
	}
}
