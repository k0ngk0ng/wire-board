package scoreclock_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type actualWorldClockDevice struct{ *mixed.Runtime }

func (d actualWorldClockDevice) PCMFrontier() uint64 { return d.Snapshot().PCM.Time }
func (d actualWorldClockDevice) RunUntil(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	return d.Runtime.RunUntil(at, dma)
}

// This uses the production PCM and full synthesizer. The supplied main/VDP
// entry is explicit; native CPU expectations above use their own external aux
// feedback. It does not replace a whole match's independently initialized clock.
func TestWorldClockActualAudioFramesAndAtomicWorldOwners(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	config := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	pcmConfig, err := library.PCMConfig(0, 64)
	if err != nil {
		t.Fatal(err)
	}
	mixedConfig := mixed.Config{PCM: pcmConfig, Chips: config.Chips}
	for _, row := range worldClockNatives(t) {
		t.Run(row.ID, func(t *testing.T) {
			base, err := session.New(library, config)
			if err != nil {
				t.Fatal(err)
			}
			initial := base.Snapshot()
			direct, err := mixed.Restore(mixedConfig, initial.Mixed)
			if err != nil {
				t.Fatal(err)
			}
			start := row.Context.IRQ.FrameOrigin + uint64(row.Context.IRQ.Video.Line)*3420
			if _, err := direct.RunUntil(start); err != nil {
				t.Fatal(err)
			}
			if _, err := direct.RequestBus(start, true); err != nil {
				t.Fatal(err)
			}
			if err := direct.SetBankBlocked(start+1, row.Blocked); err != nil {
				t.Fatal(err)
			}
			if !row.Held {
				if _, err := direct.RequestBus(start+2, false); err != nil {
					t.Fatal(err)
				}
			}
			initial.Mixed = direct.Snapshot()
			live, err := session.Restore(library, config, initial)
			if err != nil {
				t.Fatal(err)
			}
			graphics, flags, world := row.Graphics, row.Flags, row.World
			world.Clock.AudioPause = row.Pause
			wantGraphics, wantFlags, wantWorld := graphics, flags, world
			want, err := scoreclock.AdvanceBattleWorldWrapper(row.Clock, row.Context, &wantGraphics, &wantFlags, &wantWorld, actualWorldClockDevice{direct}, nil, scoreclock.Limits{MaxOperations: 65536})
			if err != nil {
				t.Fatal("real World clock setup", err)
			}
			beforeGraphics, beforeFlags, beforeWorld := graphics, flags, world
			_, err = live.PrepareBattleWorldLogic(row.Clock, row.Context, &graphics, &flags, &world, scoreclock.Limits{MaxOperations: want.Operations - 1})
			if !errors.Is(err, scoreclock.ErrBudget) || !reflect.DeepEqual(live.Snapshot(), initial) || !reflect.DeepEqual(graphics, beforeGraphics) || flags != beforeFlags || !reflect.DeepEqual(world, beforeWorld) {
				t.Fatal("late World/DMA/frame budget leaked owners", err)
			}
			got, err := live.PrepareBattleWorldLogic(row.Clock, row.Context, &graphics, &flags, &world, scoreclock.Limits{MaxOperations: 65536})
			expected := initial
			expected.Mixed = direct.Snapshot()
			if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(graphics, wantGraphics) || flags != wantFlags || !reflect.DeepEqual(world, wantWorld) || !reflect.DeepEqual(live.Snapshot(), expected) || len(got.Frames) != row.End.Frames {
				t.Fatal("joint World/DMA/vertical/audio differs", err, got, want)
			}
			if len(got.Frames) > 0 && len(got.Frames[0].PCM) == 0 {
				t.Fatal("cross-frame audio was not actually finalized")
			}
			raw, err := json.Marshal(struct {
				Audio    session.State
				Main     scoreclock.BattleMainContext
				Graphics scoreclock.BattleGraphics
				Flags    scoreclock.BattleWorldFlags
				World    engine.ChuWorldState
			}{live.Snapshot(), got.Context, graphics, flags, world})
			if err != nil {
				t.Fatal(err)
			}
			var restored struct {
				Audio    session.State
				Main     scoreclock.BattleMainContext
				Graphics scoreclock.BattleGraphics
				Flags    scoreclock.BattleWorldFlags
				World    engine.ChuWorldState
			}
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			other, err := session.Restore(library, config, restored.Audio)
			if err != nil || !reflect.DeepEqual(other.Snapshot(), live.Snapshot()) || !reflect.DeepEqual(restored.Main, got.Context) || !reflect.DeepEqual(restored.Graphics, graphics) || restored.Flags != flags || !reflect.DeepEqual(restored.World, world) {
				t.Fatal("joint main/audio/World result cannot restore", err)
			}
		})
	}
}

func TestWorldClockOuterWorldFailureDiscardsCrossFrameAudio(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	config := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	row := worldClockNatives(t)[0]
	live, err := session.New(library, config)
	if err != nil {
		t.Fatal(err)
	}
	start := row.Context.IRQ.FrameOrigin + uint64(row.Context.IRQ.Video.Line)*3420
	if _, err := live.AdvancePCM(start, nil); err != nil {
		t.Fatal(err)
	}
	initial := live.Snapshot()
	world := row.World
	world.Objects = engine.WorldObjects{0x8100: {ObjectID: 0x8100, Header: engine.MatchObjectHeader{Behavior: 0x1edac, Size: 64}}}
	world.PendingSound = 4
	sentinel := errors.New("discard full World clock branch after frame flush")
	calls := 0
	got, events, _, err := live.WorldFrame(world, [8]engine.Buttons{}, engine.ChuWorldLibrary{}, func(branch *session.Runtime, op engine.WorldSoundOperation, owners engine.WorldSoundOwners, apply func() error) error {
		if err := apply(); err != nil {
			return err
		}
		if op.Request.Kind != engine.RequestReturn {
			return nil
		}
		calls++
		stagedWorld, graphics, flags := row.World, row.Graphics, row.Flags
		stagedWorld.Clock.AudioPause = *owners.AudioPause
		result, err := branch.PrepareBattleWorldLogic(row.Clock, row.Context, &graphics, &flags, &stagedWorld, scoreclock.Limits{MaxOperations: 65536})
		if err != nil {
			return err
		}
		if len(result.Frames) != 1 || !result.AwaitingLogic {
			t.Fatal("did not reach real cross-frame dispatch")
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) || calls != 1 || events != nil || !reflect.DeepEqual(got, world) || !reflect.DeepEqual(live.Snapshot(), initial) {
		t.Fatal("outer World failure leaked main/audio branch", calls, err)
	}
}
