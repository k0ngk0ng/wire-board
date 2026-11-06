package scoreclock_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type actualWorldDevice struct{ *mixed.Runtime }

func (a actualWorldDevice) PCMFrontier() uint64 { return a.Snapshot().PCM.Time }
func (a actualWorldDevice) RunUntil(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	return a.Runtime.RunUntil(at, dma)
}

// These are declared main-entry/VDP inputs with the real production PCM and
// synth owners. Both display branches and pause/hold combinations are covered;
// the native fixture above retains only contexts actually observed there.
func TestBattleWorldEntryActualMixedTransaction(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	config := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	pcmConfig, err := library.PCMConfig(0, 64)
	if err != nil {
		t.Fatal(err)
	}
	mixedConfig := mixed.Config{PCM: pcmConfig, Chips: config.Chips}
	row := battleWorldNatives(t)[0]
	for _, ready := range []bool{false, true} {
		for _, paused := range []uint8{0, 255} {
			for _, held := range []bool{false, true} {
				t.Run(fmt.Sprintf("ready-%t/pause-%d/held-%t", ready, paused, held), func(t *testing.T) {
					base, err := session.New(library, config)
					if err != nil {
						t.Fatal(err)
					}
					initial := base.Snapshot()
					device, err := mixed.Restore(mixedConfig, initial.Mixed)
					if err != nil {
						t.Fatal(err)
					}
					start := row.Context.FrameOrigin + uint64(row.Context.Video.Line)*3420
					if held {
						start -= 3420
					}
					if _, err := device.RunUntil(start); err != nil {
						t.Fatal(err)
					}
					if held {
						if _, err := device.RequestBus(start, true); err != nil {
							t.Fatal(err)
						}
					}
					initial.Mixed = device.Snapshot()
					live, err := session.Restore(library, config, initial)
					if err != nil {
						t.Fatal(err)
					}
					input := scoreclock.BattleWorldInput{DisplayReady: ready}
					flags := scoreclock.BattleWorldFlags{WorldActive: true, MissedWorld: true}
					wantFlags, pause := flags, paused
					gateOperation := uint32(0)
					ops := uint32(0)
					want, err := scoreclock.PrepareBattleWorldIRQ(row.Clock, row.Context, input, &wantFlags, &pause, actualWorldDevice{device}, func(e scoreclock.BattleWorldObservation) error {
						if e.Stage == "cpu" {
							ops++
						}
						if e.IO != nil && e.IO.Kind == "transfer-gate" {
							gateOperation = ops
						}
						return nil
					}, scoreclock.Limits{MaxOperations: 65536})
					if err != nil || gateOperation == 0 {
						t.Fatal("actual mixed entry setup", err)
					}
					before := initial
					_, err = live.BeginBattleWorldIRQ(row.Clock, row.Context, input, &flags, &pause, scoreclock.Limits{MaxOperations: gateOperation})
					if !errors.Is(err, scoreclock.ErrBudget) || flags.TransferActive || pause != paused || !reflect.DeepEqual(live.Snapshot(), before) {
						t.Fatal("post-gate budget leaked owners", err)
					}
					got, err := live.BeginBattleWorldIRQ(row.Clock, row.Context, input, &flags, &pause, scoreclock.Limits{MaxOperations: 65536})
					expectedState := initial
					expectedState.Mixed = device.Snapshot()
					state := live.Snapshot()
					if err != nil || !reflect.DeepEqual(got, want) || !got.AwaitingWorld || flags != wantFlags || !flags.TransferActive || !flags.WorldActive || !flags.MissedWorld || pause != paused || !reflect.DeepEqual(state, expectedState) || !state.Mixed.PCM.Global.BankBlocked || (state.Mixed.PCM.Held != nil) != (paused == 255) {
						t.Fatal("actual World entry owner mismatch", err, got, want)
					}
					raw, err := json.Marshal(state)
					if err != nil {
						t.Fatal(err)
					}
					var checkpoint session.State
					if err := json.Unmarshal(raw, &checkpoint); err != nil {
						t.Fatal(err)
					}
					restored, err := session.Restore(library, config, checkpoint)
					if err != nil || !reflect.DeepEqual(restored.Snapshot(), state) {
						t.Fatal("World gate checkpoint differs", err)
					}
					// A later failure in the real outer World transaction discards the
					// branch's gate, bus and synth changes together.
					live, err = session.Restore(library, config, initial)
					if err != nil {
						t.Fatal(err)
					}
					world := engine.ChuWorldState{Objects: engine.WorldObjects{0x8100: {ObjectID: 0x8100, Header: engine.MatchObjectHeader{Behavior: 0x1edac, Size: 64}}}, PendingSound: 4}
					sentinel := errors.New("late World entry rollback")
					calls := 0
					resultWorld, events, _, err := live.WorldFrame(world, [8]engine.Buttons{}, engine.ChuWorldLibrary{}, func(branch *session.Runtime, op engine.WorldSoundOperation, owners engine.WorldSoundOwners, apply func() error) error {
						if err := apply(); err != nil {
							return err
						}
						if op.Request.Kind != engine.RequestReturn {
							return nil
						}
						calls++
						stagedFlags := row.Flags
						if _, err := branch.BeginBattleWorldIRQ(row.Clock, row.Context, input, &stagedFlags, owners.AudioPause, scoreclock.Limits{MaxOperations: 65536}); err != nil {
							return err
						}
						return sentinel
					})
					if !errors.Is(err, sentinel) || calls != 1 || events != nil || !reflect.DeepEqual(resultWorld, world) || !reflect.DeepEqual(live.Snapshot(), initial) {
						t.Fatal("late World entry leaked audio", calls, err)
					}
				})
			}
		}
	}
}
