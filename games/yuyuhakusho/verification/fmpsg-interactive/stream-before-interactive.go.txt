package chips

import (
	"errors"
	"math"
	"math/bits"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/dsp"
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

// FrameBoundary commits one fixed native reference frame atomically. PSG
// edges are inserted individually, including ahead edges into the FIR tail.
// FM lookahead samples advance chip state but are discarded from this frame's
// output, matching the separate native FM-buffer flush transaction.
func (stream *Stream) FrameBoundary() (Frame, error) {
	if err := validBase(stream.config, stream.state); err != nil {
		return Frame{}, err
	}
	if err := validQueue(stream.config, stream.state, nil); err != nil {
		return Frame{}, err
	}
	next := copyState(stream.state)
	start := next.Time
	end, ok := plus(start, FrameCycles)
	if !ok {
		return Frame{}, errors.New("mixed frame end overflow")
	}
	absolute, ok := plus(stream.config.MasterOrigin, start)
	if !ok {
		return Frame{}, errors.New("mixed absolute start overflow")
	}
	if _, ok = plus(stream.config.MasterOrigin, end); !ok {
		return Frame{}, errors.New("mixed absolute end overflow")
	}
	offset := phaseOffset(absolute)
	sampleCount := int((FrameCycles*rateFactor + offset) >> 52)
	var buffer [2][]int32
	for side := range buffer {
		buffer[side] = make([]int32, sampleCount+18)
		copy(buffer[side], next.Tail[side][:])
	}
	kernel := dsp.StepKernel()
	addDelta := func(cycle uint64, delta [2]int32) error {
		if cycle < start || (cycle > end && cycle-end > MaxInputAhead) {
			return errors.New("chip edge outside bounded current FIR frame")
		}
		fixed := uint32(((cycle-start)*rateFactor + offset) >> 32)
		phase := int((fixed >> 15) & 31)
		fraction := int64(fixed & 32767)
		position := int(fixed >> 20)
		if position+16 > len(buffer[0]) {
			return errors.New("chip edge exceeds fixed FIR tail")
		}
		for side := range buffer {
			part := int32(int64(delta[side]) * fraction >> 15)
			main := delta[side] - part
			for tap := 0; tap < 8; tap++ {
				buffer[side][position+tap] += int32(int64(kernel[phase][tap])*int64(main) + int64(kernel[phase+1][tap])*int64(part))
				buffer[side][position+8+tap] += int32(int64(kernel[32-phase][7-tap])*int64(main) + int64(kernel[31-phase][7-tap])*int64(part))
			}
		}
		return nil
	}
	chip, err := NewYM(next.YM)
	if err != nil {
		return Frame{}, err
	}
	psgChip, err := psg.New(psg.IntegratedHQ150Stereo, next.PSG)
	if err != nil {
		return Frame{}, err
	}
	first := (ChipPeriod - absolute%ChipPeriod) % ChipPeriod
	tick, ok := plus(start, first)
	if !ok {
		return Frame{}, errors.New("mixed FM grid overflow")
	}
	advanceFM := func(to uint64) error {
		if to <= tick {
			return nil
		}
		count := (to-tick-1)/ChipPeriod + 1
		if count > MaxYMBlockSamples {
			return errors.New("mixed FM block exceeds sample cap")
		}
		generated, err := chip.Generate(int(count))
		if err != nil {
			return err
		}
		for i := uint64(0); i < count; i++ {
			if tick < end {
				current := [2]int32{generated[i*2], generated[i*2+1]}
				delta := [2]int32{current[0] - next.LastFMOutput[0], current[1] - next.LastFMOutput[1]}
				if err = addDelta(tick, delta); err != nil {
					return err
				}
				next.LastFMOutput = current
			}
			var valid bool
			tick, valid = plus(tick, ChipPeriod)
			if !valid {
				return errors.New("mixed FM tick overflow")
			}
		}
		return nil
	}
	commitPSGEdges := func(edges []psg.Edge) error {
		for _, edge := range edges {
			if err := addDelta(edge.Cycle, edge.Delta); err != nil {
				return err
			}
		}
		return nil
	}
	var reads []Readback
	cursor := start + uint64(next.InputAhead)
	for _, event := range next.Pending {
		switch event.Kind {
		case YMPortWrite:
			if event.Port&1 != 0 {
				if err = advanceFM(event.Cycle); err != nil {
					return Frame{}, err
				}
				local := event.Cycle - start
				deadline := (local+BusyClockPeriod-1)/BusyClockPeriod*BusyClockPeriod + 32*BusyClockPeriod
				next.BusyUntil, ok = plus(start, deadline)
				if !ok {
					return Frame{}, errors.New("mixed YM busy overflow")
				}
			}
			if err = chip.WritePort(event.Port, event.Value); err != nil {
				return Frame{}, err
			}
		case PSGByteWrite:
			edges, err := psgChip.Write(psg.Event{Cycle: event.Cycle, Data: event.Value})
			if err != nil {
				return Frame{}, err
			}
			if err = commitPSGEdges(edges); err != nil {
				return Frame{}, err
			}
		case YMStatusRead:
			value := uint8(0)
			if event.Port == 0 {
				if err = advanceFM(event.Cycle); err != nil {
					return Frame{}, err
				}
				value = chip.Snapshot().Status
				if event.Cycle < next.BusyUntil {
					value |= 128
				}
			}
			reads = append(reads, Readback{Cycle: event.Cycle, Port: event.Port, Value: value})
		}
		cursor = event.Cycle
	}
	edges, err := psgChip.FrameBoundary(end)
	if err != nil {
		return Frame{}, err
	}
	if err = commitPSGEdges(edges); err != nil {
		return Frame{}, err
	}
	if err = advanceFM(end); err != nil {
		return Frame{}, err
	}
	pcm := make([]int16, sampleCount*2)
	for i := 0; i < sampleCount; i++ {
		for side := range buffer {
			value := next.Integrator[side] >> 15
			next.Integrator[side] += buffer[side][i]
			if value > 32767 {
				value = 32767
			} else if value < -32768 {
				value = -32768
			}
			pcm[i*2+side] = int16(value)
			next.Integrator[side] -= value << 6
		}
	}
	for side := range next.Tail {
		copy(next.Tail[side][:], buffer[side][sampleCount:sampleCount+18])
	}
	next.Time = end
	next.InputAhead = 0
	if cursor > end {
		next.InputAhead = uint32(cursor - end)
	}
	if next.BusyUntil <= end {
		next.BusyUntil = 0
	}
	next.YM = chip.Snapshot()
	next.PSG = psgChip.Snapshot()
	next.Pending = nil
	if err = validBase(stream.config, next); err != nil {
		return Frame{}, err
	}
	stream.state = next
	return Frame{PCM: pcm, Reads: reads}, nil
}
