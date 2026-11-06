package scoreclock_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type battleWorldNative struct {
	ID            string
	Clock         scoreclock.Clock
	Context       scoreclock.BattleSampleContext
	Flags         scoreclock.BattleWorldFlags
	DisplayReady  bool
	Pause         uint8
	Blocked, Held bool
	Frontier      uint64
	CPU           []struct {
		Clock   scoreclock.Clock
		Context scoreclock.BattleSampleContext
		Flags   scoreclock.BattleWorldFlags
	}
	IO        []scoreclock.BattleSampleIO
	Auxiliary []struct {
		Target, Delay uint64
		Held          bool
	}
	Bus []struct {
		At            uint64
		Asserted      bool
		Delay         uint64
		GrantedBefore bool
		FrontierAfter uint64
	}
	End struct {
		Clock         scoreclock.Clock
		Context       scoreclock.BattleSampleContext
		Flags         scoreclock.BattleWorldFlags
		Blocked, Held bool
	}
}

func battleWorldNatives(t *testing.T) []battleWorldNative {
	t.Helper()
	var fixture struct {
		Version int
		Cases   []battleWorldNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/world-irq-gate/portable-world-entry.json.gz"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 23 {
		t.Fatal("original World entry fixture missing")
	}
	return fixture.Cases
}

// The native line and bus feedback are declared inputs. This oracle checks the
// main owner, not an independent reconstruction of the background player.
type observedWorldDevice struct {
	row                battleWorldNative
	auxIndex, busIndex int
	frontier           uint64
	held, blocked      bool
}

func newObservedWorld(row battleWorldNative) *observedWorldDevice {
	return &observedWorldDevice{row: row, frontier: row.Frontier, held: row.Held, blocked: row.Blocked}
}
func (a *observedWorldDevice) PCMFrontier() uint64             { return a.frontier }
func (a *observedWorldDevice) BusGranted(uint64) (bool, error) { return a.held, nil }
func (a *observedWorldDevice) RequestBus(at uint64, asserted bool) (uint64, error) {
	if a.busIndex >= len(a.row.Bus) {
		return 0, errors.New("extra World bus request")
	}
	x := a.row.Bus[a.busIndex]
	if at != x.At || asserted != x.Asserted || a.held != x.GrantedBefore {
		return 0, errors.New("original World bus request differs")
	}
	a.busIndex++
	a.held, a.frontier = asserted, x.FrontierAfter
	return at + x.Delay, nil
}
func (a *observedWorldDevice) SetBankBlocked(_ uint64, blocked bool) error {
	if !a.held {
		return errors.New("World gate lacks held bus")
	}
	a.blocked = blocked
	return nil
}
func (a *observedWorldDevice) RunUntil(at uint64, dma []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	if len(dma) != 0 || a.auxIndex >= len(a.row.Auxiliary) {
		return pcmcommands.RunResult{}, errors.New("extra World auxiliary request")
	}
	x := a.row.Auxiliary[a.auxIndex]
	if at != x.Target || a.held != x.Held {
		return pcmcommands.RunResult{}, errors.New("original World auxiliary request differs")
	}
	a.auxIndex++
	if !a.held {
		a.frontier = at
	}
	return pcmcommands.RunResult{Frontier: a.frontier, M68KDelay: x.Delay}, nil
}

func TestBattleWorldEntryOriginalBatchesFlagsAndIO(t *testing.T) {
	steps, accesses := 0, 0
	for _, row := range battleWorldNatives(t) {
		t.Run(row.ID, func(t *testing.T) {
			device := newObservedWorld(row)
			flags, pause := row.Flags, row.Pause
			cpus, ios := 0, 0
			result, err := scoreclock.PrepareBattleWorldIRQ(row.Clock, row.Context, scoreclock.BattleWorldInput{DisplayReady: row.DisplayReady}, &flags, &pause, device, func(event scoreclock.BattleWorldObservation) error {
				switch event.Stage {
				case "cpu":
					if cpus >= len(row.CPU) {
						t.Fatal("extra World CPU work", event)
					}
					want := row.CPU[cpus]
					if event.Clock != want.Clock || !reflect.DeepEqual(event.Context, want.Context) || event.Flags != want.Flags {
						t.Fatal("native World CPU/batch differs", cpus, event, want)
					}
					cpus++
				case "io":
					if ios >= len(row.IO) || event.IO == nil || *event.IO != row.IO[ios] {
						t.Fatal("native World IO differs", ios, event.IO, row.IO[minimumIndex(ios, len(row.IO))])
					}
					ios++
				default:
					t.Fatal("unexpected World observation", event.Stage)
				}
				return nil
			}, scoreclock.Limits{MaxOperations: 65536})
			if err != nil || cpus != len(row.CPU) || ios != len(row.IO) || device.auxIndex != len(row.Auxiliary) || device.busIndex != len(row.Bus) || result.Clock != row.End.Clock || !reflect.DeepEqual(result.Context, row.End.Context) || flags != row.End.Flags || device.blocked != row.End.Blocked || device.held != row.End.Held || pause != row.Pause || !result.AwaitingWorld || int(result.Operations) != len(row.CPU) {
				t.Fatal("whole original World entry differs", err, cpus, ios, device, result, flags, row.End)
			}
			steps += cpus
			accesses += ios
		})
	}
	if steps != 4013 || accesses != 994 {
		t.Fatal("incomplete original World entry coverage", steps, accesses)
	}
}

func TestBattleWorldEntryInvalidContextBudgetAndObserver(t *testing.T) {
	row := battleWorldNatives(t)[0]
	sentinel := errors.New("stop World entry")
	for _, kind := range []string{"budget", "observer", "mask", "phase", "frontier", "pin", "window", "flags", "pause", "device"} {
		t.Run(kind, func(t *testing.T) {
			clock, context := row.Clock, row.Context
			device := newObservedWorld(row)
			flags, pause := row.Flags, row.Pause
			fp, pp := &flags, &pause
			var d scoreclock.BattleWorldDevice = device
			limits := scoreclock.Limits{MaxOperations: 65536}
			switch kind {
			case "budget":
				limits.MaxOperations = 1
			case "mask":
				context.Mask = 0
			case "phase":
				context.Phase = 0
			case "frontier":
				device.frontier = 0
			case "pin":
				context.Pin = 1
			case "window":
				clock.Cycle = context.FrameOrigin + 896040
			case "flags":
				fp = nil
			case "pause":
				pp = nil
			case "device":
				d = nil
			}
			calls := 0
			_, err := scoreclock.PrepareBattleWorldIRQ(clock, context, scoreclock.BattleWorldInput{DisplayReady: true}, fp, pp, d, func(scoreclock.BattleWorldObservation) error {
				calls++
				if kind == "observer" {
					return sentinel
				}
				return nil
			}, limits)
			if err == nil {
				t.Fatal("invalid World entry succeeded", kind)
			}
			if kind == "budget" && !errors.Is(err, scoreclock.ErrBudget) {
				t.Fatal(err)
			}
			if kind == "observer" && (!errors.Is(err, sentinel) || calls != 1) {
				t.Fatal("World observer error lost", calls, err)
			}
			if kind != "budget" && kind != "observer" && (calls != 0 || device.auxIndex != 0 || device.busIndex != 0 || flags != row.Flags) {
				t.Fatal("invalid entry produced effects", kind)
			}
		})
	}
}
