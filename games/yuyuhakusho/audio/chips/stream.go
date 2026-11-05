package chips

import (
	"errors"
	"math"
	"math/bits"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
)

type Profile uint8

const ReferenceMixedHQ44100 Profile = 1

const (
	FrameCycles      uint64 = 896040
	ChipPeriod       uint64 = 1008
	BusyClockPeriod  uint64 = 42
	MaxInputAhead    uint64 = 1008
	MaxPendingEvents        = 4096
	rateFactor       uint64 = ((1<<52)*44100 + 53693175 - 1) / 53693175
	phaseMask        uint64 = (1 << 52) - 1
)

type Config struct {
	Profile      Profile
	MasterOrigin uint64
}
type EventKind uint8

const (
	YMPortWrite EventKind = iota + 1
	PSGByteWrite
	YMStatusRead
)

// Event is a raw ordered port input in relative master cycles. Equal-cycle
// order is significant. A register intent expands to address then data at one
// cycle; PSG and FM remain separate chip inputs, never pre-mixed recordings.
type Event struct {
	Cycle       uint64
	Kind        EventKind
	Port, Value uint8
}
type Readback struct {
	Cycle       uint64
	Port, Value uint8
}
type Frame struct {
	PCM   []int16
	Reads []Readback
}

type State struct {
	Time         uint64
	YM           YMState
	PSG          psg.State
	LastFMOutput [2]int32
	Integrator   [2]int32
	Tail         [2][18]int32
	InputAhead   uint32
	BusyUntil    uint64
	Pending      []Event
}
type Stream struct {
	config Config
	state  State
}

func copyState(state State) State {
	state.Pending = append([]Event(nil), state.Pending...)
	return state
}
func plus(a, b uint64) (uint64, bool) {
	if a > math.MaxUint64-b {
		return 0, false
	}
	return a + b, true
}
func phaseOffset(absolute uint64) uint64 {
	_, low := bits.Mul64(absolute, rateFactor)
	low, _ = bits.Add64(low, rateFactor/2, 0)
	return low & phaseMask
}

func validBase(config Config, state State) error {
	if config.Profile != ReferenceMixedHQ44100 {
		return errors.New("unsupported mixed chip reference profile")
	}
	if state.Time%FrameCycles != 0 || state.InputAhead > uint32(MaxInputAhead) || len(state.Pending) > MaxPendingEvents {
		return errors.New("invalid mixed frame/ahead/pending state")
	}
	if _, ok := plus(config.MasterOrigin, state.Time); !ok {
		return errors.New("mixed origin+time overflow")
	}
	cursor, ok := plus(state.Time, uint64(state.InputAhead))
	if !ok {
		return errors.New("mixed raw cursor overflow")
	}
	if _, ok = plus(config.MasterOrigin, cursor); !ok {
		return errors.New("mixed origin+cursor overflow")
	}
	if state.BusyUntil != 0 {
		if state.BusyUntil <= state.Time || state.BusyUntil-state.Time > MaxInputAhead+33*BusyClockPeriod-1 {
			return errors.New("invalid mixed YM busy deadline")
		}
		if _, ok = plus(config.MasterOrigin, state.BusyUntil); !ok {
			return errors.New("mixed origin+busy overflow")
		}
	}
	if state.PSG.Cursor < state.Time || state.PSG.Cursor > cursor {
		return errors.New("mixed PSG/input cursor mismatch")
	}
	if _, err := psg.New(psg.IntegratedHQ150Stereo, state.PSG); err != nil {
		return err
	}
	if err := validYM(state.YM); err != nil {
		return err
	}
	for _, value := range state.LastFMOutput {
		if value < -60000 || value > 60000 {
			return errors.New("invalid prior mixed FM output")
		}
	}
	return nil
}
func validQueue(config Config, state State, input []Event) error {
	if len(state.Pending)+len(input) > MaxPendingEvents {
		return errors.New("too many mixed chip inputs")
	}
	end, ok := plus(state.Time, FrameCycles)
	if !ok {
		return errors.New("mixed frame end overflow")
	}
	cursor, ok := plus(state.Time, uint64(state.InputAhead))
	if !ok {
		return errors.New("mixed raw cursor overflow")
	}
	for _, batch := range [][]Event{state.Pending, input} {
		for _, e := range batch {
			if e.Cycle < cursor || (e.Cycle > end && e.Cycle-end > MaxInputAhead) {
				return errors.New("mixed input time is out of bounds/order")
			}
			if _, ok = plus(config.MasterOrigin, e.Cycle); !ok {
				return errors.New("mixed origin+event overflow")
			}
			switch e.Kind {
			case YMPortWrite:
				if e.Port > 3 {
					return errors.New("invalid YM write port")
				}
			case PSGByteWrite:
				if e.Port != 0 {
					return errors.New("invalid PSG write port")
				}
			case YMStatusRead:
				if e.Port > 3 || e.Value != 0 {
					return errors.New("invalid YM status request")
				}
			default:
				return errors.New("unknown mixed chip event")
			}
			cursor = e.Cycle
		}
	}
	return nil
}

func NewStream(config Config, state State) (*Stream, error) {
	if err := validBase(config, state); err != nil {
		return nil, err
	}
	if len(state.Pending) > 0 {
		if err := validQueue(config, state, nil); err != nil {
			return nil, err
		}
	}
	return &Stream{config: config, state: copyState(state)}, nil
}
func (stream *Stream) Snapshot() State { return copyState(stream.state) }
func (stream *Stream) Queue(events []Event) error {
	if err := validQueue(stream.config, stream.state, events); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	next := copyState(stream.state)
	next.Pending = append(next.Pending, events...)
	stream.state = next
	return nil
}

// FrameBoundary preserves the queued API while using the same frame transaction
// as the interactive API. The original state commits only after every input and
// the reference flush succeed; returned queued readbacks remain in input order.
func (stream *Stream) FrameBoundary() (Frame, error) {
	if err := validBase(stream.config, stream.state); err != nil {
		return Frame{}, err
	}
	if err := validQueue(stream.config, stream.state, nil); err != nil {
		return Frame{}, err
	}
	committed := stream.state
	committed.Pending = nil
	active, err := NewInteractive(stream.config, committed)
	if err != nil {
		return Frame{}, err
	}
	var reads []Readback
	for _, event := range stream.state.Pending {
		value, err := active.event(event)
		if err != nil {
			return Frame{}, err
		}
		if event.Kind == YMStatusRead {
			reads = append(reads, Readback{Cycle: event.Cycle, Port: event.Port, Value: value})
		}
	}
	frame, err := active.FrameBoundary()
	if err != nil {
		return Frame{}, err
	}
	next, err := active.CommittedState()
	if err != nil {
		return Frame{}, err
	}
	stream.state = next
	frame.Reads = reads
	return frame, nil
}
