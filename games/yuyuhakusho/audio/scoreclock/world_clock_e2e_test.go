package scoreclock_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type worldClockNative struct {
	ID            string
	Clock         scoreclock.Clock
	Context       scoreclock.BattleMainContext
	Graphics      scoreclock.BattleGraphics
	World         engine.ChuWorldState
	Flags         scoreclock.BattleWorldFlags
	Pause         uint8
	Held, Blocked bool
	Frontier      uint64
	CPU           []struct {
		Clock   scoreclock.Clock
		Context scoreclock.BattleSampleContext
		Flags   scoreclock.BattleWorldFlags
	}
	IO        []scoreclock.BattleSampleIO
	Auxiliary []struct{ Target, Delay uint64 }
	Bus       []struct {
		At            uint64
		Asserted      bool
		Delay         uint64
		HeldBefore    bool
		FrontierAfter uint64
	}
	End struct {
		Clock                                                     scoreclock.Clock
		Context                                                   scoreclock.BattleSampleContext
		Flags                                                     scoreclock.BattleWorldFlags
		Blocked, Held, AwaitingLogic, BusyReturned, QueueConsumed bool
		Frames                                                    int
	}
}

func worldClockNatives(t *testing.T) []worldClockNative {
	t.Helper()
	var fixture struct {
		Version int
		Cases   []worldClockNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/world-clock/portable-world-clock.json.gz"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 19 {
		t.Fatal("whole original World wrapper oracle missing")
	}
	return fixture.Cases
}

// Native background and request contention are declared external inputs. Main
// DMA, video/frame order and all wrapper/vertical IRQ work are production work.
type observedWorldClockDevice struct {
	row                        worldClockNative
	frontier                   uint64
	held, blocked              bool
	auxIndex, busIndex, frames int
}

func observedWorldClock(row worldClockNative) *observedWorldClockDevice {
	return &observedWorldClockDevice{row: row, frontier: row.Frontier, held: row.Held, blocked: row.Blocked}
}
func (d *observedWorldClockDevice) PCMFrontier() uint64             { return d.frontier }
func (d *observedWorldClockDevice) DMAWaiting() bool                { return false }
func (d *observedWorldClockDevice) AdvanceDMA(uint64, uint32) error { return nil }
func (d *observedWorldClockDevice) BusGranted(uint64) (bool, error) { return d.held, nil }
func (d *observedWorldClockDevice) RunUntil(uint64, []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	return pcmcommands.RunResult{}, errors.New("World coordinator must use its explicitly scheduled DMA interface")
}
func (d *observedWorldClockDevice) RunDMAUntil(at uint64, windows []pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	if d.auxIndex >= len(d.row.Auxiliary) || at != d.row.Auxiliary[d.auxIndex].Target {
		return pcmcommands.RunResult{}, errors.New("original World auxiliary batch target differs")
	}
	x := d.row.Auxiliary[d.auxIndex]
	d.auxIndex++
	if !d.held {
		d.frontier = at
	}
	return pcmcommands.RunResult{Frontier: d.frontier, M68KDelay: x.Delay}, nil
}
func (d *observedWorldClockDevice) RequestBus(at uint64, asserted bool) (uint64, error) {
	if d.busIndex >= len(d.row.Bus) {
		return 0, errors.New("extra original World bus request")
	}
	x := d.row.Bus[d.busIndex]
	if at != x.At || asserted != x.Asserted || d.held != x.HeldBefore {
		return 0, errors.New("original World bus RMW differs")
	}
	d.busIndex++
	d.held, d.frontier = asserted, x.FrontierAfter
	return at + x.Delay, nil
}
func (d *observedWorldClockDevice) SetBankBlocked(_ uint64, blocked bool) error {
	if !d.held {
		return errors.New("World gate lacks actual bus")
	}
	d.blocked = blocked
	return nil
}
func (d *observedWorldClockDevice) FrameBoundary() (mixed.Frame, error) {
	d.frames++
	return mixed.Frame{Protocol: pcmcommands.Frame{Index: uint64(d.frames - 1)}}, nil
}

func TestWorldClockOriginalWholeWrapperAndVerticalIRQ(t *testing.T) {
	steps, ios, frames, busy := 0, 0, 0, 0
	for _, row := range worldClockNatives(t) {
		t.Run(row.ID, func(t *testing.T) {
			device := observedWorldClock(row)
			graphics, flags, world := row.Graphics, row.Flags, row.World
			world.Clock.AudioPause = row.Pause
			cpus, accesses := 0, 0
			got, err := scoreclock.AdvanceBattleWorldWrapper(row.Clock, row.Context, &graphics, &flags, &world, device, func(e scoreclock.BattleWorldObservation) error {
				switch e.Stage {
				case "cpu":
					if cpus >= len(row.CPU) {
						t.Fatal("extra World work", e)
					}
					want := row.CPU[cpus]
					if e.Clock != want.Clock || !reflect.DeepEqual(e.Context, want.Context) || e.Flags != want.Flags {
						t.Fatal("whole native World work differs", cpus, e, want)
					}
					cpus++
				case "io":
					if accesses >= len(row.IO) || e.IO == nil || *e.IO != row.IO[accesses] {
						t.Fatal("whole native World IO differs", accesses, e.IO, row.IO[minimumIndex(accesses, len(row.IO))])
					}
					accesses++
				default:
					t.Fatal("unexpected whole World observation", e.Stage)
				}
				return nil
			}, scoreclock.Limits{MaxOperations: 65536})
			if err != nil || got.Clock != row.End.Clock || !reflect.DeepEqual(got.Context.IRQ, row.End.Context) || flags != row.End.Flags || device.held != row.End.Held || device.blocked != row.End.Blocked || got.AwaitingLogic != row.End.AwaitingLogic || got.BusyReturned != row.End.BusyReturned || cpus != len(row.CPU) || accesses != len(row.IO) || device.auxIndex != len(row.Auxiliary) || device.busIndex != len(row.Bus) || len(got.Frames) != row.End.Frames || device.frames != row.End.Frames || int(got.Operations) != cpus {
				t.Fatal("whole native World dispatch differs", err, got, row.End, device, cpus, accesses)
			}
			if row.End.QueueConsumed && len(graphics.Pending) != 0 || !row.End.QueueConsumed && !reflect.DeepEqual(graphics.Pending, row.Graphics.Pending) {
				t.Fatal("World transfer consumption differs")
			}
			if world.Clock.AudioPause != row.Pause {
				t.Fatal("World wrapper changed pause")
			}
			steps += cpus
			ios += accesses
			frames += device.frames
			if got.BusyReturned {
				busy++
			}
		})
	}
	if steps != 2268 || ios != 994 || frames != 6 || busy != 4 {
		t.Fatal("incomplete whole World coverage", steps, ios, frames, busy)
	}
}

func TestWorldClockInvalidInputAndExternalFailures(t *testing.T) {
	row := worldClockNatives(t)[0]
	sentinel := errors.New("World clock observation failed")
	for _, kind := range []string{"budget", "observer", "mask", "phase", "window", "frontier", "scene", "source", "target", "length", "world", "graphics", "flags", "device"} {
		t.Run(kind, func(t *testing.T) {
			clock, context := row.Clock, row.Context
			graphics, flags, world := row.Graphics, row.Flags, row.World
			graphics.Pending = append([]scoreclock.BattleWorldTransfer(nil), graphics.Pending...)
			world.Clock.AudioPause = row.Pause
			gp, fp, wp := &graphics, &flags, &world
			device := observedWorldClock(row)
			var dev scoreclock.BattleWorldClockDevice = device
			limits := scoreclock.Limits{MaxOperations: 65536}
			switch kind {
			case "budget":
				limits.MaxOperations = 1
			case "mask":
				context.IRQ.Mask = 0
			case "phase":
				context.IRQ.Phase = 0
			case "window":
				clock.Cycle = context.IRQ.FrameOrigin + 896040
			case "frontier":
				device.frontier = 0
			case "scene":
				world.SceneMode = 0
			case "source":
				graphics.Pending[0].SourceWord = 0x800000
			case "target":
				graphics.Pending[0].Target = 0
			case "length":
				graphics.Pending[0].Words = 0
			case "world":
				wp = nil
			case "graphics":
				gp = nil
			case "flags":
				fp = nil
			case "device":
				dev = nil
			}
			calls := 0
			_, err := scoreclock.AdvanceBattleWorldWrapper(clock, context, gp, fp, wp, dev, func(scoreclock.BattleWorldObservation) error {
				calls++
				if kind == "observer" {
					return sentinel
				}
				return nil
			}, limits)
			if err == nil {
				t.Fatal("unsupported World clock input succeeded", kind)
			}
			if kind == "budget" && !errors.Is(err, scoreclock.ErrBudget) {
				t.Fatal(err)
			}
			if kind == "observer" && (!errors.Is(err, sentinel) || calls != 1) {
				t.Fatal("observer error lost", calls, err)
			}
			if kind != "budget" && kind != "observer" && (calls != 0 || device.auxIndex != 0 || device.busIndex != 0 || device.frames != 0) {
				t.Fatal("invalid World clock input did external work", kind)
			}
		})
	}
}
