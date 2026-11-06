package scoreclock_test

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"reflect"
	"testing"
)

type runtimeBattleAux struct{ runtime *session.Runtime }

func (a runtimeBattleAux) PCMFrontier() uint64 { return a.runtime.Snapshot().Mixed.PCM.Time }
func (a runtimeBattleAux) RunUntil(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	return a.runtime.AdvancePCM(at, dma)
}

// Clock and VDP entry context are declared inputs; the production PCM owner
// supplies all background feedback. This tests the full actual owner transaction
// through the audio driver, with the outer IRQ return explicitly still pending.
func TestBattleSamplePrefixActualAudioTransaction(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	_, priorities := soundIRQNatives(t)
	row := battlePrefixNatives(t)[0]
	cfg := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	for _, kind := range []string{"success", "late-budget", "late-world"} {
		t.Run(kind, func(t *testing.T) {
			live, err := session.New(library, cfg)
			if err != nil {
				t.Fatal(err)
			}
			lineStart := row.Context.FrameOrigin + uint64(row.Context.Video.Line)*3420
			if _, err := live.AdvancePCM(lineStart, nil); err != nil {
				t.Fatal(err)
			}
			initial := live.Snapshot()
			direct, err := session.Restore(library, cfg, initial)
			if err != nil {
				t.Fatal(err)
			}
			prepared, err := scoreclock.PrepareBattleSampleIRQ(row.Clock, row.Context, row.Input, runtimeBattleAux{direct}, nil, scoreclock.Limits{MaxOperations: 65536})
			if err != nil {
				t.Fatal(err)
			}
			queue, pause := [4]uint8{4}, uint8(0)
			expectedQueue, expectedPause := queue, pause
			expected, err := direct.Interrupt(prepared.Clock, &expectedQueue, &expectedPause, priorities, scoreclock.Limits{MaxOperations: 65536 - prepared.Operations})
			if err != nil {
				t.Fatal(err)
			}
			// Settle only the already completed batch; no implicit frame flush occurs.
			finalTarget := row.Context.FrameOrigin + ((expected.Clock.Cycle-row.Context.FrameOrigin)/3420)*3420
			if _, err := direct.AdvancePCM(finalTarget, nil); err != nil {
				t.Fatal(err)
			}
			limits := scoreclock.Limits{MaxOperations: 65536}
			if kind == "late-budget" {
				limits.MaxOperations = prepared.Operations + 1
			}
			if kind == "late-world" {
				world := engine.ChuWorldState{Objects: engine.WorldObjects{0x8100: {ObjectID: 0x8100, Header: engine.MatchObjectHeader{Behavior: 0x1edac, Size: 64}}}, PendingSound: 4}
				sentinel := errors.New("discard pending outer return")
				got, events, _, err := live.WorldFrame(world, [8]engine.Buttons{}, engine.ChuWorldLibrary{}, func(branch *session.Runtime, op engine.WorldSoundOperation, owners engine.WorldSoundOwners, apply func() error) error {
					if err := apply(); err != nil {
						return err
					}
					if op.Request.Kind != engine.RequestReturn {
						return nil
					}
					result, err := branch.BeginBattleSampleIRQ(row.Clock, row.Context, row.Input, owners.Queue, owners.AudioPause, priorities, limits)
					if err != nil {
						return err
					}
					if !result.AwaitingReturn {
						t.Fatal("pretended complete outer IRQ")
					}
					return sentinel
				})
				if !errors.Is(err, sentinel) || events != nil || !reflect.DeepEqual(got, world) || !reflect.DeepEqual(live.Snapshot(), initial) {
					t.Fatal("late world failure leaked sample/audio", err)
				}
				return
			}
			result, err := live.BeginBattleSampleIRQ(row.Clock, row.Context, row.Input, &queue, &pause, priorities, limits)
			if kind == "late-budget" {
				if !errors.Is(err, scoreclock.ErrBudget) || queue != [4]uint8{4} || pause != 0 || !reflect.DeepEqual(live.Snapshot(), initial) {
					t.Fatal("late prefix budget leaked owners", err)
				}
				return
			}
			if err != nil || !result.AwaitingReturn || result.Context.Mask != 7 || result.Prefix.Clock != prepared.Clock || result.Clock != expected.Clock || queue != expectedQueue || pause != expectedPause || !reflect.DeepEqual(live.Snapshot(), direct.Snapshot()) {
				t.Fatal("joint prefix/driver differs", err, result, expected.Clock)
			}
			raw, _ := json.Marshal(live.Snapshot())
			var restored session.State
			if err := json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			other, err := session.Restore(library, cfg, restored)
			if err != nil || !reflect.DeepEqual(other.Snapshot(), live.Snapshot()) {
				t.Fatal("audio result cannot restore", err)
			}
		})
	}
}
