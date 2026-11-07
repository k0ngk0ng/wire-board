package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

type worldFrameWorkNative struct {
	Name, Kind                   string
	Before, After                WorldClockState
	Seed, NextSeed, RandomResult uint32
	Clock                        struct{ Cycle, Refresh uint64 }
	Steps                        []struct{ Cycle, Refresh uint64 }
}

func frameWorkNatives(t *testing.T) []worldFrameWorkNative {
	t.Helper()
	packed, err := os.ReadFile("../verification/world-frame-work/native-frame-work.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Version int
		Cases   []worldFrameWorkNative
	}
	if err := json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 4568 {
		t.Fatal("whole original clock work input missing")
	}
	return fixture.Cases
}
func clockTask(name string) WorldClockTask {
	return map[string]WorldClockTask{"video": WorldVideoClock, "logic": WorldLogicClock, "toggle": WorldTogglePause, "pause": WorldApplyPause, "shake": WorldFeedback}[name]
}
func TestWorldFrameWorkOriginalCompleteHelperBoundaries(t *testing.T) {
	steps := 0
	for _, row := range frameWorkNatives(t) {
		cycle, refresh, index := row.Clock.Cycle, row.Clock.Refresh, 0
		hook := func(op MainWorkOperation) error {
			var err error
			cycle, refresh, err = op.Advance(cycle, refresh)
			if err != nil || index >= len(row.Steps) || cycle != row.Steps[index].Cycle || refresh != row.Steps[index].Refresh {
				t.Fatal("actual original frame work differs", row.Name, index, op, cycle, refresh, err, row.Steps)
			}
			index++
			return nil
		}
		if row.Kind == "random" {
			next, result, err := NextRandomWork(row.Seed, hook)
			if err != nil || next != row.NextSeed || result != row.RandomResult {
				t.Fatal("original RNG work result differs", row.Name, next, result, err)
			}
		} else {
			got, err := AdvanceWorldClockWork(row.Before, clockTask(row.Kind), func(op MainWorkOperation, _ *WorldClockState) error { return hook(op) })
			if err != nil || got != row.After {
				t.Fatal("original live clock work result differs", row.Name, got, row.After, err)
			}
		}
		if index != len(row.Steps) {
			t.Fatal("missing original frame operations", row.Name, index, len(row.Steps))
		}
		steps += index
	}
	if steps != 71555 {
		t.Fatal("incomplete original helper work", steps)
	}
}
func TestWorldFrameWorkFailureAtEveryNativeOperation(t *testing.T) {
	sentinel := errors.New("discard current world clock work")
	for _, row := range frameWorkNatives(t) {
		for failure := range row.Steps {
			calls := 0
			if row.Kind == "random" {
				next, result, err := NextRandomWork(row.Seed, func(MainWorkOperation) error {
					calls++
					if calls == failure+1 {
						return sentinel
					}
					return nil
				})
				if !errors.Is(err, sentinel) || calls != failure+1 || next != row.Seed || result != 0 {
					t.Fatal("RNG operation failure leaked output", row.Name, failure, calls, next, result, err)
				}
			} else {
				got, err := AdvanceWorldClockWork(row.Before, clockTask(row.Kind), func(_ MainWorkOperation, live *WorldClockState) error {
					calls++
					if calls == failure+1 {
						live.AudioPause = 255
						return sentinel
					}
					return nil
				})
				if !errors.Is(err, sentinel) || calls != failure+1 || got != row.Before {
					t.Fatal("clock operation failure leaked state", row.Name, failure, calls, err)
				}
			}
		}
	}
}
func TestWorldFrameWorkPauseHoldsLiveConsumerValue(t *testing.T) {
	before := WorldClockState{AudioPause: 1}
	got, err := AdvanceWorldClockWork(before, WorldApplyPause, func(_ MainWorkOperation, live *WorldClockState) error { live.AudioPause = 255; return nil })
	if err != nil || got.AudioPause != 255 || before.AudioPause != 1 {
		t.Fatal("pause hold overwrote IRQ acknowledgement", got, err)
	}
	before.PauseFlags = 0x40
	got, err = AdvanceWorldClockWork(before, WorldApplyPause, func(op MainWorkOperation, live *WorldClockState) error {
		if op.Phase == "pause-state-set" {
			live.AudioPause = 255
		}
		return nil
	})
	if err != nil || got.AudioPause != 1 || got.PauseFlags != 0xc0 {
		t.Fatal("pause transition missed actual producer write", got, err)
	}
}

func TestWorldFrameWorkPreludeUsesLiveOwnersAndExactOrder(t *testing.T) {
	initial := WorldFrameState{Clock: WorldClockState{ControllerCount: 4, PauseFlags: 1, PauseMask: 15, AudioPause: 1}, RandomSeed: 0x5eedbeef, ResetEnabled: 1}
	samples := [8]ControllerFrame{}
	for i := range samples {
		samples[i], _ = SampleController(0, Buttons(i&3), MD6, false)
	}
	want, outcome, err := BeginWorldFrame(initial, samples)
	if err != nil {
		t.Fatal(err)
	}
	normalized := 0
	var phases []string
	got, actual, err := BeginWorldFrameWork(initial, WorldFrameWorkHooks{
		Normalize: func(owners WorldFrameOwners, work MainWorkHook) error {
			normalized++
			if owners.Clock == nil || owners.Controllers == nil || owners.RandomSeed == nil || owners.SlowGate == nil || owners.ResetEnabled == nil {
				t.Fatal("missing real frame owners")
			}
			if err := work(MainWorkOperation{Phase: "declared-normalization", CPUCycles: 16}); err != nil {
				return err
			}
			*owners.Controllers = samples
			return nil
		},
		Work: func(op MainWorkOperation, owners WorldFrameOwners) error {
			phases = append(phases, op.Phase)
			owners.Clock.AudioPause = 255
			return nil
		},
	})
	// No pause transition occurs here. The same acknowledgement must survive
	// the whole frame, including helpers after the input callback.
	want.Clock.AudioPause = 255
	if err != nil || normalized != 1 || !reflect.DeepEqual(got, want) || actual != outcome || len(phases) == 0 {
		t.Fatal("live observed prelude differs", got, want, actual, outcome, normalized, err)
	}
	for _, kind := range []string{"normalize", "late-work", "slow-skip", "reset", "missing-normalizer"} {
		t.Run(kind, func(t *testing.T) {
			state := initial
			calls := 0
			sentinel := errors.New("late prelude error")
			hooks := WorldFrameWorkHooks{Normalize: func(owners WorldFrameOwners, _ MainWorkHook) error {
				calls++
				*owners.Controllers = samples
				if kind == "normalize" {
					return sentinel
				}
				return nil
			}, Work: func(op MainWorkOperation, _ WorldFrameOwners) error {
				if kind == "late-work" && op.Phase == "feedback-pan-return" {
					return sentinel
				}
				return nil
			}}
			if kind == "slow-skip" {
				state.SlowGate = 1
				state.Clock.SlowFlag = 1
			}
			if kind == "reset" {
				hooks.Normalize = func(owners WorldFrameOwners, _ MainWorkHook) error {
					calls++
					(*owners.Controllers)[0] = ControllerFrame{Current: 0xf0, Pressed: 0x80}
					return nil
				}
			}
			if kind == "missing-normalizer" {
				hooks.Normalize = nil
			}
			got, out, err := BeginWorldFrameWork(state, hooks)
			switch kind {
			case "slow-skip":
				if err != nil || calls != 0 || out.InputConsumed || got.RandomSeed != state.RandomSeed || got.Clock.Frame != state.Clock.Frame+1 {
					t.Fatal("slow prelude consumed input", got, out, calls, err)
				}
			case "reset":
				if err != nil || !out.ResetRequested || got.RandomSeed != state.RandomSeed {
					t.Fatal("reset prelude advanced RNG", got, out, err)
				}
			default:
				if err == nil || !reflect.DeepEqual(got, state) || out != (WorldFrameOutcome{}) {
					t.Fatal("prelude failure leaked state", kind, got, out, err)
				}
			}
		})
	}
}
