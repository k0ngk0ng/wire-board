package scoreclock_test

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
)

// A declared late handler entry exercises the coordinator at the prefix's
// boundary, beyond the natural corpus's entry positions. This proves actual
// owners/frame dispatch, not a new native absolute-time oracle.
func TestWorldFlowActualLatePrefixCrossesFrameBeforeTransfers(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	config := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	row := worldClockNatives(t)[0]
	context := row.Context
	context.IRQ.Mask = 4
	context.IRQ.Video.Line = 261
	context.IRQ.Video.DisplayEnabled = true
	clock := scoreclock.Clock{Cycle: 261*3420 + 3000, Refresh: 261*3420 + 3200}
	live, err := session.New(library, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := live.AdvancePCM(261*3420, nil); err != nil {
		t.Fatal(err)
	}
	graphics, flags, world := row.Graphics, row.Flags, row.World
	flags.TransferActive = false
	graphics.CachedDisplay = true
	got, err := live.PrepareBattleWorldIRQLogic(clock, context, scoreclock.BattleWorldInput{DisplayReady: true}, &graphics, &flags, &world, scoreclock.Limits{MaxOperations: 65536})
	if err != nil || got.Prefix.Context.FrameOrigin != 896040 || got.Dispatch.Context.IRQ.FrameOrigin != 896040 || !got.Dispatch.AwaitingLogic || len(got.Dispatch.Frames) != 1 || len(got.Dispatch.Frames[0].PCM) == 0 || got.Dispatch.Context.IRQ.Phase != 0 || graphics.CachedDisplay || !flags.WorldActive || flags.TransferActive || live.Snapshot().Mixed.PCM.Frame != 1 {
		t.Fatal("real late-prefix frame handoff failed", err, got)
	}
}
