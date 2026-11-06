// Package mixed couples the independent PCM protocol and FM/PSG synthesizer.
// It preserves producer order and makes each exposed operation transactional;
// it does not choose world interrupts, score costs or a playback timeline.
package mixed

import (
	"errors"
	"fmt"
	"math"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
)

type Config struct {
	PCM   pcmcommands.Config
	Chips chips.Config
}

// State contains each owner's canonical state once. LastPCMWasDAC records only
// the cause of the current chip Cursor; it adds no clock or output history.
type State struct {
	Version       uint8
	ChipConfig    chips.Config
	PCM           pcmcommands.State
	Chips         chips.InteractiveState
	LastPCMWasDAC bool
}

type Frame struct {
	PCM      []int16
	Protocol pcmcommands.Frame
}

type Runtime struct {
	config        chips.Config
	origin        uint64
	pcm           *pcmcommands.Device
	chip          *chips.Interactive
	lastPCMWasDAC bool
}

func validPair(pcm pcmcommands.State, chip chips.InteractiveState) error {
	if pcm.Frame > math.MaxUint64/chips.FrameCycles || pcm.Frame*chips.FrameCycles != chip.Time {
		return errors.New("PCM and chip reference frames differ")
	}
	if len(pcm.PendingLedger) != 0 {
		return errors.New("mixed checkpoint contains unconsumed PCM output")
	}
	return nil
}

func New(config Config, initial chips.State) (*Runtime, error) {
	if !config.PCM.IncludeYMAddresses {
		return nil, errors.New("mixed synthesis requires physical PCM address/data ports")
	}
	pcm, err := pcmcommands.New(config.PCM)
	if err != nil {
		return nil, err
	}
	chip, err := chips.NewInteractive(config.Chips, initial)
	if err != nil {
		return nil, err
	}
	if err = validPair(pcm.Snapshot(), chip.Snapshot()); err != nil {
		return nil, err
	}
	return &Runtime{config: config.Chips, origin: config.PCM.Origin, pcm: pcm, chip: chip}, nil
}

func Restore(config Config, state State) (*Runtime, error) {
	if state.Version != 1 || state.ChipConfig != config.Chips || !config.PCM.IncludeYMAddresses {
		return nil, errors.New("mixed checkpoint profile differs")
	}
	if err := validPair(state.PCM, state.Chips); err != nil {
		return nil, err
	}
	pcm, err := pcmcommands.Restore(config.PCM, state.PCM)
	if err != nil {
		return nil, err
	}
	chip, err := chips.RestoreInteractive(config.Chips, state.Chips)
	if err != nil {
		return nil, err
	}
	return &Runtime{config: config.Chips, origin: config.PCM.Origin, pcm: pcm, chip: chip, lastPCMWasDAC: state.LastPCMWasDAC}, nil
}

func (r *Runtime) Snapshot() State {
	return State{Version: 1, ChipConfig: r.config, PCM: r.pcm.Snapshot(), Chips: r.chip.Snapshot(), LastPCMWasDAC: r.lastPCMWasDAC}
}

func (r *Runtime) fork() (*Runtime, error) {
	chip, err := chips.RestoreInteractive(r.config, r.chip.Snapshot())
	if err != nil {
		return nil, err
	}
	return &Runtime{config: r.config, origin: r.origin, pcm: r.pcm.Fork(), chip: chip, lastPCMWasDAC: r.lastPCMWasDAC}, nil
}

func (r *Runtime) atomic(action func(*Runtime) error) error {
	next, err := r.fork()
	if err != nil {
		return err
	}
	if err = action(next); err != nil {
		return err
	}
	*r = *next
	return nil
}

// Bus/publication calls use the PCM owner's coordinate, including its origin.
// The synthesizer uses a relative cursor and its own immutable master phase.
func (r *Runtime) relative(at uint64) (uint64, error) {
	if at < r.origin {
		return 0, errors.New("mixed access precedes PCM origin")
	}
	return at - r.origin, nil
}

func (r *Runtime) consume(events []pcmcommands.Event) error {
	for i, event := range events {
		if event.Kind == pcmcommands.Bank || event.Kind == pcmcommands.Metadata || event.Kind == pcmcommands.Dummy {
			continue
		}
		at, err := r.relative(event.Cycle)
		if err != nil {
			return err
		}
		switch event.Kind {
		case pcmcommands.YMAddress:
			if event.Port != 0 && event.Port != 2 {
				return errors.New("PCM address lacks a physical address port")
			}
			err = r.chip.WritePort(at, event.Port, event.Value)
		case pcmcommands.DAC, pcmcommands.Pan, pcmcommands.Enable, pcmcommands.YMWrite:
			if event.Port != 1 && event.Port != 3 {
				return errors.New("PCM data lacks a physical data port")
			}
			err = r.chip.WritePort(at, event.Port, event.Value)
		case pcmcommands.Reset:
			cursor := r.chip.Snapshot().Cursor
			if at < cursor {
				if !r.lastPCMWasDAC {
					return errors.New("backwards reset lacks a completed PCM DAC cause")
				}
				err = r.chip.ResetYMAfterDAC(at, cursor)
			} else {
				err = r.chip.ResetYM(at)
			}
		default:
			return fmt.Errorf("unsupported PCM physical event %q", event.Kind)
		}
		if err != nil {
			return fmt.Errorf("PCM event%d %s at%d: %w", i, event.Kind, event.Cycle, err)
		}
		r.lastPCMWasDAC = event.Kind == pcmcommands.DAC
	}
	return nil
}

func (r *Runtime) Status() pcmcommands.Status { return r.pcm.Status() }

func (r *Runtime) RunUntil(at uint64, dma ...[]pcmcommands.DMAInterval) (pcmcommands.RunResult, error) {
	var result pcmcommands.RunResult
	err := r.atomic(func(next *Runtime) error {
		var err error
		result, err = next.pcm.RunUntil(at, dma...)
		if err != nil {
			return err
		}
		return next.consume(next.pcm.DrainLedger())
	})
	if err != nil {
		return pcmcommands.RunResult{}, err
	}
	return result, nil
}

func (r *Runtime) RequestBus(at uint64, asserted bool) (uint64, error) {
	return r.RequestBusWithDMA(at, asserted, nil)
}

func (r *Runtime) RequestBusWithDMA(at uint64, asserted bool, dma []pcmcommands.DMAInterval) (uint64, error) {
	var after uint64
	err := r.atomic(func(next *Runtime) error {
		var err error
		after, err = next.pcm.RequestBusWithDMA(at, asserted, dma)
		if err != nil {
			return err
		}
		return next.consume(next.pcm.DrainLedger())
	})
	if err != nil {
		return 0, err
	}
	return after, nil
}

func (r *Runtime) BusGranted(at uint64) (bool, error) { return r.pcm.BusGranted(at) }
func (r *Runtime) Mailbox(at uint64) (uint8, error)   { return r.pcm.Mailbox(at) }

func (r *Runtime) PublishAccepted(input pcmcommands.Input) error {
	return r.atomic(func(next *Runtime) error { return next.pcm.PublishAccepted(input) })
}

func (r *Runtime) WritePort(at uint64, port, value uint8) error {
	return r.atomic(func(next *Runtime) error {
		cycle, err := next.relative(at)
		if err != nil {
			return err
		}
		if err = next.chip.WritePort(cycle, port, value); err != nil {
			return err
		}
		next.lastPCMWasDAC = false
		return nil
	})
}

func (r *Runtime) WritePSG(at uint64, value uint8) error {
	return r.atomic(func(next *Runtime) error {
		cycle, err := next.relative(at)
		if err != nil {
			return err
		}
		if err = next.chip.WritePSG(cycle, value); err != nil {
			return err
		}
		next.lastPCMWasDAC = false
		return nil
	})
}

func (r *Runtime) ReadStatus(at uint64, port uint8) (uint8, error) {
	var value uint8
	err := r.atomic(func(next *Runtime) error {
		cycle, err := next.relative(at)
		if err != nil {
			return err
		}
		value, err = next.chip.ReadStatus(cycle, port)
		if err != nil {
			return err
		}
		next.lastPCMWasDAC = false
		return nil
	})
	if err != nil {
		return 0, err
	}
	return value, nil
}

func (r *Runtime) finish(protocol pcmcommands.Frame) (Frame, error) {
	if err := r.consume(protocol.Ledger); err != nil {
		return Frame{}, err
	}
	before := r.chip.Snapshot().Cursor
	frame, err := r.chip.FrameBoundary()
	if err != nil {
		return Frame{}, err
	}
	if r.chip.Snapshot().Cursor != before {
		r.lastPCMWasDAC = false
	}
	return Frame{PCM: frame.PCM, Protocol: protocol}, nil
}

// FrameBoundary flushes both owners only after the caller has run or held the
// PCM Device to its boundary. It does not invent an extra RunUntil budget.
func (r *Runtime) FrameBoundary() (Frame, error) {
	var result Frame
	err := r.atomic(func(next *Runtime) error {
		protocol, err := next.pcm.FlushFrame()
		if err != nil {
			return err
		}
		result, err = next.finish(protocol)
		return err
	})
	if err != nil {
		return Frame{}, err
	}
	return result, nil
}

// AdvanceFrame accepts typed publication/hold windows from the real caller.
// These are external decisions, not a driver or world timing substitute.
func (r *Runtime) AdvanceFrame(plan pcmcommands.FramePlan) (Frame, error) {
	var result Frame
	err := r.atomic(func(next *Runtime) error {
		protocol, err := next.pcm.AdvanceFrame(plan)
		if err != nil {
			return err
		}
		result, err = next.finish(protocol)
		return err
	})
	if err != nil {
		return Frame{}, err
	}
	return result, nil
}
