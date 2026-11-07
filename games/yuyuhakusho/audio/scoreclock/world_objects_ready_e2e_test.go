package scoreclock_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
)

func TestWorldObjectsReadyActualOneTransactionAndWholeBudget(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	config := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	for _, indices := range [][3]int{{0, 0, 0}, {7, 5, 8}} {
		prefix := battleWorldNatives(t)[indices[0]]
		wrapper := worldClockNatives(t)[indices[1]]
		logic := worldLogicNatives(t)[indices[2]]
		if prefix.ID != wrapper.ID {
			t.Fatal("original chain IDs differ", prefix.ID, wrapper.ID)
		}
		live, err := session.New(library, config)
		if err != nil {
			t.Fatal(err)
		}
		start := prefix.Context.FrameOrigin + uint64(prefix.Context.Video.Line)*3420
		if _, err := live.AdvancePCM(start, nil); err != nil {
			t.Fatal(err)
		}
		initial := live.Snapshot()
		direct, err := session.Restore(library, config, initial)
		if err != nil {
			t.Fatal(err)
		}
		graphics, flags, world := logic.Graphics, prefix.Flags, logic.World
		graphics.Pending = wrapper.Graphics.Pending
		graphics.CachedDisplay = prefix.Context.Video.DisplayEnabled
		context := scoreclock.BattleMainContext{IRQ: prefix.Context, NormalizedDevices: logic.Context.NormalizedDevices}
		physical := scoreclock.BattleWorldInput{DisplayReady: prefix.DisplayReady}
		wantGraphics, wantFlags, wantWorld := graphics, flags, world
		first, err := direct.PrepareBattleWorldIRQLogic(prefix.Clock, context, physical, &wantGraphics, &wantFlags, &wantWorld, scoreclock.Limits{MaxOperations: 65536})
		if err != nil {
			t.Fatal(err)
		}
		last, err := direct.BeginBattleWorldLogic(first.Dispatch.Clock, first.Dispatch.Context, &wantGraphics, &wantFlags, &wantWorld, logic.Input, scoreclock.Limits{MaxOperations: 65536 - first.Dispatch.Operations})
		if err != nil {
			t.Fatal(err)
		}
		beforeGraphics, beforeFlags, beforeWorld := graphics, flags, world
		_, err = live.PrepareBattleWorldObjects(prefix.Clock, context, physical, &graphics, &flags, &world, logic.Input, scoreclock.Limits{MaxOperations: first.Dispatch.Operations + last.Operations - 1})
		if !errors.Is(err, scoreclock.ErrBudget) || !reflect.DeepEqual(live.Snapshot(), initial) || !reflect.DeepEqual(graphics, beforeGraphics) || flags != beforeFlags || !reflect.DeepEqual(world, beforeWorld) {
			t.Fatal("late combined pre-object failure leaked earlier IRQ/audio", err)
		}
		got, err := live.PrepareBattleWorldObjects(prefix.Clock, context, physical, &graphics, &flags, &world, logic.Input, scoreclock.Limits{MaxOperations: 65536})
		if err != nil || got.Clock != last.Clock || !reflect.DeepEqual(got.Context, last.Context) || got.Operations != first.Dispatch.Operations+last.Operations || !got.AwaitingObjects || got.BusyReturned || got.Outcome != last.Outcome || !reflect.DeepEqual(live.Snapshot(), direct.Snapshot()) || !reflect.DeepEqual(graphics, wantGraphics) || flags != wantFlags || !reflect.DeepEqual(world, wantWorld) {
			t.Fatal("one-budget whole pre-object flow differs", err, got, last)
		}
		if len(got.Frames) != len(first.Dispatch.Frames)+len(last.Frames) {
			t.Fatal("new audio frames lost or duplicated")
		}
		for i, f := range append(first.Dispatch.Frames, last.Frames...) {
			if !reflect.DeepEqual(got.Frames[i], f) {
				t.Fatal("combined frame output differs", i)
			}
		}
	}
}
