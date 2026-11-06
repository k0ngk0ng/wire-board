package scoreclock_test

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"reflect"
	"testing"
)

type battlePrefixNative struct {
	ID      string
	Clock   scoreclock.Clock
	Context scoreclock.BattleSampleContext
	Input   scoreclock.BattleSampleInput
	CPU     []struct {
		Clock   scoreclock.Clock
		Context scoreclock.BattleSampleContext
	}
	IO        []scoreclock.BattleSampleIO
	Auxiliary []struct{ Target, Delay uint64 }
	End       struct {
		Clock   scoreclock.Clock
		Context scoreclock.BattleSampleContext
	}
}

func battlePrefixNatives(t *testing.T) []battlePrefixNative {
	t.Helper()
	var fixture struct {
		Version int
		Cases   []battlePrefixNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/battle-sample-irq/portable-battle-prefix.json.gz"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 23 {
		t.Fatal("whole original prefix fixture missing")
	}
	return fixture.Cases
}

type observedBattleAux struct {
	row      battlePrefixNative
	index    int
	frontier uint64
}

func (a *observedBattleAux) PCMFrontier() uint64 { return a.frontier }
func (a *observedBattleAux) RunUntil(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	if len(dma) != 0 || a.index >= len(a.row.Auxiliary) || a.row.Auxiliary[a.index].Target != at {
		return pcmcommands.RunResult{}, errors.New("unexpected original auxiliary request")
	}
	value := a.row.Auxiliary[a.index]
	a.index++
	a.frontier = at
	return pcmcommands.RunResult{Frontier: at, M68KDelay: value.Delay}, nil
}
func TestBattleSamplePrefixOriginalBatchesAndIO(t *testing.T) {
	for _, row := range battlePrefixNatives(t) {
		t.Run(row.ID, func(t *testing.T) {
			aux := &observedBattleAux{row: row, frontier: row.Context.FrameOrigin + uint64(row.Context.Video.Line)*3420}
			cpus, ios := 0, 0
			result, err := scoreclock.PrepareBattleSampleIRQ(row.Clock, row.Context, row.Input, aux, func(event scoreclock.BattleSampleObservation) error {
				switch event.Stage {
				case "cpu":
					if cpus >= len(row.CPU) {
						t.Fatal("extra CPU quantum", event)
					}
					want := row.CPU[cpus]
					if event.Clock != want.Clock || !reflect.DeepEqual(event.Context, want.Context) {
						t.Fatal("native CPU/batch boundary differs", cpus, event, want)
					}
					cpus++
				case "io":
					if ios >= len(row.IO) || event.IO == nil || !reflect.DeepEqual(*event.IO, row.IO[ios]) {
						t.Fatal("native IO differs", ios, event.IO, row.IO[minimumIndex(ios, len(row.IO))])
					}
					ios++
				default:
					t.Fatal("unknown sample observation", event.Stage)
				}
				return nil
			}, scoreclock.Limits{MaxOperations: 65536})
			if err != nil || cpus != len(row.CPU) || ios != len(row.IO) || aux.index != len(row.Auxiliary) || result.Clock != row.End.Clock || !reflect.DeepEqual(result.Context, row.End.Context) || int(result.Operations) != len(row.CPU) {
				t.Fatal("complete original prefix differs", err, cpus, ios, aux.index, result, row.End)
			}
		})
	}
}
func minimumIndex(index, length int) int {
	if index >= length {
		return length - 1
	}
	return index
}
func TestBattleSamplePrefixUnsupportedInputsBudgetAndObserverFailure(t *testing.T) {
	row := battlePrefixNatives(t)[0]
	sentinel := errors.New("prefix observer failure")
	for _, kind := range []string{"budget", "observer", "mask", "phase", "frontier", "controller", "window"} {
		t.Run(kind, func(t *testing.T) {
			clock, context, input := row.Clock, row.Context, row.Input
			aux := &observedBattleAux{row: row, frontier: context.FrameOrigin + uint64(context.Video.Line)*3420}
			limits := scoreclock.Limits{MaxOperations: 65536}
			switch kind {
			case "budget":
				limits.MaxOperations = 1
			case "mask":
				context.Mask = 0
			case "phase":
				context.Phase = 2
			case "frontier":
				aux.frontier = 0
			case "controller":
				input.Controllers.Ports[0].Count = 2
			case "window":
				clock.Cycle = context.FrameOrigin + 896040
			}
			calls := 0
			_, err := scoreclock.PrepareBattleSampleIRQ(clock, context, input, aux, func(scoreclock.BattleSampleObservation) error {
				calls++
				if kind == "observer" {
					return sentinel
				}
				return nil
			}, limits)
			if err == nil {
				t.Fatal("invalid prefix succeeded", kind)
			}
			if kind == "budget" && !errors.Is(err, scoreclock.ErrBudget) {
				t.Fatal(err)
			}
			if kind == "observer" && (!errors.Is(err, sentinel) || calls != 1) {
				t.Fatal("observer error not retained", calls, err)
			}
			if kind != "budget" && kind != "observer" && (calls != 0 || aux.index != 0) {
				t.Fatal("invalid context produced external work", kind, calls, aux.index)
			}
		})
	}
}
