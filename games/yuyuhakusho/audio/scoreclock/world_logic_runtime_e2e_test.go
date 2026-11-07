package scoreclock_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func TestWorldLogicBeginActualMixedLiveFieldsAndAtomicRestore(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	config := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	for _, index := range []int{0, 1, 8} {
		row := worldLogicNatives(t)[index]
		t.Run(row.ID, func(t *testing.T) {
			live, err := session.New(library, config)
			if err != nil {
				t.Fatal(err)
			}
			start := row.Context.IRQ.FrameOrigin + uint64(row.Context.IRQ.Video.Line)*3420
			if _, err := live.AdvancePCM(start, nil); err != nil {
				t.Fatal(err)
			}
			initial := live.Snapshot()
			direct, err := session.Restore(library, config, initial)
			if err != nil {
				t.Fatal(err)
			}
			graphics, flags, world := row.Graphics, row.Flags, row.World
			wantGraphics, wantFlags, wantWorld := graphics, flags, world
			want, err := direct.BeginBattleWorldLogic(row.Clock, row.Context, &wantGraphics, &wantFlags, &wantWorld, row.Input, scoreclock.Limits{MaxOperations: 65536})
			if err != nil {
				t.Fatal("real logical prelude setup", err)
			}
			_, err = live.BeginBattleWorldLogic(row.Clock, row.Context, &graphics, &flags, &world, row.Input, scoreclock.Limits{MaxOperations: want.Operations - 1})
			if !errors.Is(err, scoreclock.ErrBudget) || !reflect.DeepEqual(live.Snapshot(), initial) || !reflect.DeepEqual(graphics, row.Graphics) || flags != row.Flags || !reflect.DeepEqual(world, row.World) {
				t.Fatal("late prelude budget leaked owners", err)
			}
			got, err := live.BeginBattleWorldLogic(row.Clock, row.Context, &graphics, &flags, &world, row.Input, scoreclock.Limits{MaxOperations: 65536})
			if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(live.Snapshot(), direct.Snapshot()) || !reflect.DeepEqual(graphics, wantGraphics) || flags != wantFlags || !reflect.DeepEqual(world, wantWorld) || len(got.Frames) != row.End.Frames {
				t.Fatal("real joint prelude differs", err)
			}
			raw, err := json.Marshal(struct {
				Audio    session.State
				Context  scoreclock.BattleMainContext
				Graphics scoreclock.BattleGraphics
				World    engine.ChuWorldState
			}{live.Snapshot(), got.Context, graphics, world})
			if err != nil {
				t.Fatal(err)
			}
			var restored struct {
				Audio    session.State
				Context  scoreclock.BattleMainContext
				Graphics scoreclock.BattleGraphics
				World    engine.ChuWorldState
			}
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			other, err := session.Restore(library, config, restored.Audio)
			if err != nil || !reflect.DeepEqual(other.Snapshot(), live.Snapshot()) || !reflect.DeepEqual(restored.Context, got.Context) || !reflect.DeepEqual(restored.Graphics, graphics) || !reflect.DeepEqual(restored.World, world) {
				t.Fatal("logical joint state restore differs", err)
			}
		})
	}
}
