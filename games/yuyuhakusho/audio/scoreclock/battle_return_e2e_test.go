package scoreclock_test

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"reflect"
	"testing"
)

type battleReturnNative struct {
	ID         string
	Clock      scoreclock.Clock
	Context    scoreclock.BattleSampleContext
	CallerMask uint8
	CPU        []struct {
		Clock   scoreclock.Clock
		Context scoreclock.BattleSampleContext
	}
	IO        []scoreclock.BattleSampleIO
	Auxiliary []struct{ Target, Delay uint64 }
	End       struct {
		Clock   scoreclock.Clock
		Context scoreclock.BattleSampleContext
	}
	PendingWorld bool
}

func battleReturnNatives(t *testing.T) []battleReturnNative {
	t.Helper()
	var data struct {
		Version int
		Cases   []battleReturnNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/battle-irq-return/portable-battle-return.json.gz"), &data); err != nil {
		t.Fatal(err)
	}
	if data.Version != 1 || len(data.Cases) != 1895 {
		t.Fatal("original return matrix missing")
	}
	return data.Cases
}
func TestBattleReturnOriginalCompleteSuffix(t *testing.T) {
	pending := 0
	for _, row := range battleReturnNatives(t) {
		// Reuse the actual-batch feedback adapter; declared private cases return zero.
		input := battlePrefixNative{Context: row.Context}
		input.Auxiliary = row.Auxiliary
		aux := &observedBattleAux{row: input, frontier: row.Context.FrameOrigin + uint64(row.Context.Video.Line)*3420}
		cpu, io := 0, 0
		result, err := scoreclock.FinishBattleSampleIRQ(row.Clock, row.Context, row.CallerMask, aux, func(event scoreclock.BattleSampleObservation) error {
			if event.Stage == "cpu" {
				if cpu >= len(row.CPU) || event.Clock != row.CPU[cpu].Clock || !reflect.DeepEqual(event.Context, row.CPU[cpu].Context) {
					t.Fatal("native return CPU differs", row.ID, cpu, event)
				}
				cpu++
			} else if event.Stage == "io" {
				if io >= len(row.IO) || event.IO == nil || !reflect.DeepEqual(*event.IO, row.IO[io]) {
					t.Fatal("native return IO differs", row.ID, io, event)
				}
				io++
			} else {
				t.Fatal("unexpected return observation", event.Stage)
			}
			return nil
		}, scoreclock.Limits{MaxOperations: 65536})
		if err != nil || cpu != len(row.CPU) || io != len(row.IO) || aux.index != len(row.Auxiliary) || result.Clock != row.End.Clock || !reflect.DeepEqual(result.Context, row.End.Context) || result.PendingWorld != row.PendingWorld || result.Returned == row.PendingWorld || int(result.Operations) != len(row.CPU) {
			t.Fatal("whole native return differs", row.ID, err, cpu, io, result, row.End)
		}
		if result.PendingWorld {
			pending++
		}
	}
	if pending != 8 {
		t.Fatal("unpriced world IRQ cases omitted", pending)
	}
}
func TestBattleReturnFailureAndUnsupportedStage(t *testing.T) {
	row := battleReturnNatives(t)[0]
	sentinel := errors.New("return observer error")
	for _, kind := range []string{"budget", "observer", "mask", "phase", "caller-mask", "frontier"} {
		context := row.Context
		mask := row.CallerMask
		limits := scoreclock.Limits{MaxOperations: 65536}
		input := battlePrefixNative{Context: context}
		input.Auxiliary = row.Auxiliary
		aux := &observedBattleAux{row: input, frontier: context.FrameOrigin + uint64(context.Video.Line)*3420}
		switch kind {
		case "budget":
			limits.MaxOperations = 1
		case "mask":
			context.Mask = 0
		case "phase":
			context.Phase = 0
		case "caller-mask":
			mask = 8
		case "frontier":
			aux.frontier = 0
		}
		calls := 0
		_, err := scoreclock.FinishBattleSampleIRQ(row.Clock, context, mask, aux, func(scoreclock.BattleSampleObservation) error {
			calls++
			if kind == "observer" {
				return sentinel
			}
			return nil
		}, limits)
		if err == nil {
			t.Fatal("invalid return succeeded", kind)
		}
		if kind == "budget" && !errors.Is(err, scoreclock.ErrBudget) {
			t.Fatal(err)
		}
		if kind == "observer" && (!errors.Is(err, sentinel) || calls != 1) {
			t.Fatal("return observer continued", calls, err)
		}
		if kind != "budget" && kind != "observer" && (calls != 0 || aux.index != 0) {
			t.Fatal("unsupported return emitted work", kind, calls, aux.index)
		}
	}
}
