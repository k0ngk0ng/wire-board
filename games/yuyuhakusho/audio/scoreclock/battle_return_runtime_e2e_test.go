package scoreclock_test

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"reflect"
	"testing"
)

func TestBattleReturnWholeActualAudioAndWorld(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	_, priorities := soundIRQNatives(t)
	row := battlePrefixNatives(t)[0]
	cfg := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	for _, kind := range []string{"direct", "late-budget", "world-resume", "late-world"} {
		t.Run(kind, func(t *testing.T) {
			live, err := session.New(library, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := live.AdvancePCM(uint64(row.Context.Video.Line)*3420, nil); err != nil {
				t.Fatal(err)
			}
			initial := live.Snapshot()
			direct, err := session.Restore(library, cfg, initial)
			if err != nil {
				t.Fatal(err)
			}
			q, pause := [4]uint8{4}, uint8(0)
			expectedQueue := q
			expectedPause := pause
			begin, err := direct.BeginBattleSampleIRQ(row.Clock, row.Context, row.Input, &expectedQueue, &expectedPause, priorities, scoreclock.Limits{MaxOperations: 65536})
			if err != nil {
				t.Fatal(err)
			}
			returned, err := scoreclock.FinishBattleSampleIRQ(begin.Clock, begin.Context, 0, runtimeBattleAux{direct}, nil, scoreclock.Limits{MaxOperations: 65536 - begin.Operations})
			if err != nil || !returned.Returned || returned.PendingWorld {
				t.Fatal("declared direct return failed", returned, err)
			}
			limits := scoreclock.Limits{MaxOperations: 65536}
			if kind == "late-budget" {
				limits.MaxOperations = begin.Operations + 1
			}
			if kind == "world-resume" || kind == "late-world" {
				world := engine.ChuWorldState{Objects: engine.WorldObjects{0x8100: {ObjectID: 0x8100, Header: engine.MatchObjectHeader{Behavior: 0x1edac, Size: 64}}}, PendingSound: 4}
				if kind == "late-world" {
					world.Objects[0x9fc0] = engine.WorldObject{ObjectID: 0x9fc0, Header: engine.MatchObjectHeader{Behavior: 0xdeadbeef, Size: 64}}
				}
				calls := 0
				got, events, _, err := live.WorldFrame(world, [8]engine.Buttons{}, engine.ChuWorldLibrary{}, func(branch *session.Runtime, op engine.WorldSoundOperation, owners engine.WorldSoundOwners, apply func() error) error {
					if err := apply(); err != nil {
						return err
					}
					if op.Request.Kind != engine.RequestReturn {
						return nil
					}
					calls++
					result, err := branch.RunBattleSampleIRQ(row.Clock, row.Context, row.Input, 0, owners.Queue, owners.AudioPause, priorities, limits)
					if err != nil {
						return err
					}
					if result.AwaitingReturn || result.Context.Mask != 0 {
						t.Fatal("World resumed before RTE")
					}
					return nil
				})
				if calls != 1 {
					t.Fatal("wrong IRQ count", calls)
				}
				if kind == "late-world" {
					if err == nil || events != nil || !reflect.DeepEqual(got, world) || !reflect.DeepEqual(live.Snapshot(), initial) {
						t.Fatal("late World failure leaked full IRQ", err)
					}
				} else {
					if err != nil || got.PendingSound != 0 || got.SoundQueue != expectedQueue || got.Clock.AudioPause != expectedPause || !reflect.DeepEqual(live.Snapshot(), direct.Snapshot()) {
						t.Fatal("resumed World differs", err)
					}
				}
				return
			}
			result, err := live.RunBattleSampleIRQ(row.Clock, row.Context, row.Input, 0, &q, &pause, priorities, limits)
			if kind == "late-budget" {
				if !errors.Is(err, scoreclock.ErrBudget) || q != [4]uint8{4} || pause != 0 || !reflect.DeepEqual(live.Snapshot(), initial) {
					t.Fatal("return budget leaked owners", err)
				}
				return
			}
			if err != nil || result.AwaitingReturn || result.Clock != returned.Clock || !reflect.DeepEqual(result.Context, returned.Context) || result.Operations != begin.Operations+returned.Operations || q != expectedQueue || pause != expectedPause || !reflect.DeepEqual(live.Snapshot(), direct.Snapshot()) {
				t.Fatal("complete IRQ differs", result, returned, err)
			}
		})
	}
}
