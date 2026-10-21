package basketball_test

// Failure inventory before implementation: idle deadline survives migration;
// ordinary frames force durable writes; period/phase boundaries are not durable;
// restore changes game identity/progress/system deadline or retains old controls;
// a single seat reclaim clears another seat or cancels explicit autoplay.
import (
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
)

func TestBasketballRealtimeIdleAndCheckpointBoundaries(t *testing.T) {
	module, ctx, m := hostStart(t)
	if m.Deadline != 0 || len(m.TimeoutActors) != 0 {
		t.Fatal("realtime game exposes idle takeover deadline")
	}
	if m.CheckpointKey == "" {
		t.Fatal("missing initial durable phase")
	}
	openingKey := m.CheckpointKey
	for n := 0; n < 50 && hostState(t, m).Runtime.Playing == nil; n++ {
		m = hostTick(t, module, &ctx, m)
		if hostState(t, m).Runtime.Playing == nil && m.CheckpointKey != openingKey {
			t.Fatal("ordinary opening frame forces checkpoint")
		}
	}
	if m.CheckpointKey == openingKey {
		t.Fatal("playing boundary is not durable")
	}
	playingKey := m.CheckpointKey
	m = hostTick(t, module, &ctx, m)
	if m.CheckpointKey != playingKey {
		t.Fatal("ordinary playing frame forces checkpoint")
	}
	m = hostButtons(t, module, ctx, m, 0, "buttons", engine.ButtonStart)
	m = hostTick(t, module, &ctx, m)
	if m.CheckpointKey == playingKey {
		t.Fatal("pause boundary is not durable")
	}
	if m.Deadline != 0 || len(m.TimeoutActors) != 0 {
		t.Fatal("paused game exposes idle deadline")
	}
}

func TestBasketballRealtimeRestoreClearsControlsPreservesDeadline(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	ctx.Automated[1] = true
	m = hostTick(t, module, &ctx, m)
	m = hostButtons(t, module, ctx, m, 0, "buttons", engine.ButtonRight)
	before := hostState(t, m)
	deadline := m.SystemDeadline
	ctx.Now = ctx.Now.Add(30 * time.Second)
	next, err := module.Restore(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	after := hostState(t, next)
	if next.SystemDeadline != deadline || after.ServerAt != before.ServerAt {
		t.Fatal("restore shifted original system deadline")
	}
	if after.Epoch != before.Epoch || after.Runtime.VideoFrames != before.Runtime.VideoFrames || !reflect.DeepEqual(after.Runtime.Config, before.Runtime.Config) {
		t.Fatal("restore changed identity/progress/automation")
	}
	if after.Inputs[0].Buttons != 0 || after.Inputs[0].Pending != 0 || len(after.Inputs[0].Changes) != 0 {
		t.Fatal("restore retained stale manual controls")
	}
	if !after.Inputs[1].Automated {
		t.Fatal("restore cancelled explicit autoplay")
	}
}
