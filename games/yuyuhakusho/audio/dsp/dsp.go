// Package dsp renders the bounded DAC-only discrete reference profile. It
// executes no CPU, firmware, sample command, native context or event timetable.
package dsp

import (
	"errors"
	"math"
	"math/bits"
)

type Profile uint8

const ReferenceFrameDiscreteHQ44100 Profile = 1

const (
	FrameCycles      uint64 = 896040
	ChipPeriod       uint64 = 1008
	MaxInputAhead    uint64 = 1008
	MaxPendingEvents        = 4096
	// Untyped constant arithmetic is exact before conversion to uint64.
	rateFactor uint64 = ((1<<52)*44100 + 53693175 - 1) / 53693175
	phaseMask  uint64 = (1 << 52) - 1
)

type Config struct {
	Profile      Profile
	MasterOrigin uint64
}
type EventKind uint8

const (
	DACData EventKind = iota + 1
	DACEnable
	DACPan
)

// Cycle is relative to Config.MasterOrigin. Equal-cycle writes retain input
// order. Enable andPan use logical80 andC0 bits, not other YM register fields.
type Event struct {
	Cycle uint64
	Kind  EventKind
	Value uint8
}

// State owns committedDSP and genuinely unconsumed reference-frame inputs.
// Time is the last flush; InputAhead is the committed raw-input cursor beyond
// that flush. Resampler offset and next chip tick are derived, never duplicated.
type State struct {
	Time           uint64
	DACValue       uint8
	Enabled        bool
	Pan            uint8
	LastChipOutput [2]int32
	Integrator     [2]int32
	Tail           [2][18]int32
	InputAhead     uint32
	Pending        []Event
}
type Stream struct {
	config Config
	state  State
}

func clone(state State) State { state.Pending = append([]Event(nil), state.Pending...); return state }
func add(a, b uint64) (uint64, bool) {
	if a > math.MaxUint64-b {
		return 0, false
	}
	return a + b, true
}

// Multiplication may exceed64 bits, but only the residue modulo2^52 is needed.
// Since2^52 divides2^64, discarding the high product/add carry preserves the
// exact residue. Mul64/Add64 make the intentionally discarded parts explicit.
func phaseOffset(absolute uint64) uint64 {
	_, low := bits.Mul64(absolute, rateFactor)
	low, _ = bits.Add64(low, rateFactor/2, 0)
	return low & phaseMask
}
func nextChipDelta(absolute uint64) uint64 { return (ChipPeriod - absolute%ChipPeriod) % ChipPeriod }

func validState(config Config, state State) error {
	if config.Profile != ReferenceFrameDiscreteHQ44100 {
		return errors.New("unsupported DAC DSP reference profile")
	}
	if _, ok := add(config.MasterOrigin, state.Time); !ok {
		return errors.New("DSP origin plus time overflows")
	}
	if state.Time%FrameCycles != 0 || state.InputAhead > uint32(MaxInputAhead) || state.Pan&^uint8(0xc0) != 0 {
		return errors.New("invalid DAC DSP frame/pan/ahead state")
	}
	for _, level := range state.LastChipOutput {
		if level < -7648 || level > 8896 {
			return errors.New("invalid DAC-only previous chip output")
		}
	}
	if len(state.Pending) > MaxPendingEvents {
		return errors.New("too many pendingDAC events")
	}
	return validEvents(config, state, nil)
}
func validEvents(config Config, state State, events []Event) error {
	if len(state.Pending)+len(events) > MaxPendingEvents {
		return errors.New("too many pendingDAC events")
	}
	end, ok := add(state.Time, FrameCycles)
	if !ok {
		return errors.New("DSP frame end overflows")
	}
	cursor, ok := add(state.Time, uint64(state.InputAhead))
	if !ok {
		return errors.New("DSP raw input cursor overflows")
	}
	if _, ok := add(config.MasterOrigin, cursor); !ok {
		return errors.New("DSP raw input origin overflows")
	}
	for _, batch := range [][]Event{state.Pending, events} {
		for _, event := range batch {
			if event.Cycle < cursor {
				return errors.New("DAC events are out of reference-frame order")
			}
			if event.Cycle > end && event.Cycle-end > MaxInputAhead {
				return errors.New("DAC event exceeds bounded reference lookahead")
			}
			if _, ok := add(config.MasterOrigin, event.Cycle); !ok {
				return errors.New("DAC event origin plus cycle overflows")
			}
			switch event.Kind {
			case DACData:
			case DACEnable:
				if event.Value != 0 && event.Value != 0x80 {
					return errors.New("invalid logical DAC enable")
				}
			case DACPan:
				if event.Value&^uint8(0xc0) != 0 {
					return errors.New("invalid logical DAC pan")
				}
			default:
				return errors.New("unknown DAC event kind")
			}
			cursor = event.Cycle
		}
	}
	return nil
}

func New(config Config, initial State) (*Stream, error) {
	// An empty state at a very late origin remains representable; advancing its
	// next frame may fail atomically. Pending events need a representable frame.
	pending := initial.Pending
	initial.Pending = nil
	if config.Profile != ReferenceFrameDiscreteHQ44100 {
		return nil, errors.New("unsupported DAC DSP reference profile")
	}
	if _, ok := add(config.MasterOrigin, initial.Time); !ok {
		return nil, errors.New("DSP origin plus time overflows")
	}
	if initial.Time%FrameCycles != 0 || initial.InputAhead > uint32(MaxInputAhead) || initial.Pan&^uint8(0xc0) != 0 {
		return nil, errors.New("invalid DAC DSP frame/pan/ahead state")
	}
	cursor, ok := add(initial.Time, uint64(initial.InputAhead))
	if !ok {
		return nil, errors.New("DSP raw input cursor overflows")
	}
	if _, ok := add(config.MasterOrigin, cursor); !ok {
		return nil, errors.New("DSP raw input origin overflows")
	}
	for _, level := range initial.LastChipOutput {
		if level < -7648 || level > 8896 {
			return nil, errors.New("invalid DAC-only previous chip output")
		}
	}
	initial.Pending = pending
	if len(pending) > 0 {
		if err := validState(config, initial); err != nil {
			return nil, err
		}
	}
	return &Stream{config: config, state: clone(initial)}, nil
}
func (stream *Stream) Snapshot() State { return clone(stream.state) }

// Queue only copies unconsumed inputs. It never advancesDAC/register/filter
// state, and validation failure cannot alter a previous valid pending batch.
func (stream *Stream) Queue(events []Event) error {
	if err := validEvents(stream.config, stream.state, events); err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	next := clone(stream.state)
	next.Pending = append(next.Pending, events...)
	stream.state = next
	return nil
}
func chipLevel(state State) [2]int32 {
	raw := int32(0)
	if state.Enabled {
		raw = (int32(state.DACValue) - 128) * 64
	}
	var result [2]int32
	for channel, bit := range [2]uint8{128, 64} {
		on := state.Pan&bit != 0
		level := int32(768)
		if on {
			level += raw
		}
		if raw < 0 {
			if on {
				level -= 224
			} else {
				level -= 256
			}
		}
		result[channel] = level
	}
	return result
}

// FrameBoundary follows the explicitly observed reference-frame contract.
// Native raw writes may be ahead ofend. Their times stay unchanged: chip
// samples are generated to each write, but only ticks<end enter the flush;
// lookahead is discarded while the latest register values remain committed.
// DAC-onlymapping is memoryless. A future fullFM implementation must also
// retain its phase/envelope effects from discarded samples.
func (stream *Stream) FrameBoundary() ([]int16, error) {
	if err := validState(stream.config, stream.state); err != nil {
		return nil, err
	}
	next := clone(stream.state)
	start := next.Time
	end, ok := add(start, FrameCycles)
	if !ok {
		return nil, errors.New("DSP frame end overflows")
	}
	absolute, ok := add(stream.config.MasterOrigin, start)
	if !ok {
		return nil, errors.New("DSP origin plus time overflows")
	}
	if _, ok := add(stream.config.MasterOrigin, end); !ok {
		return nil, errors.New("DSP frame end origin overflows")
	}
	offset := phaseOffset(absolute)
	// One frame is fixed and bounded: FrameCycles*rateFactor+offset fitsuint64.
	_, product := bits.Mul64(FrameCycles, rateFactor)
	total, carry := bits.Add64(product, offset, 0)
	if carry != 0 {
		return nil, errors.New("DSP resampler frame product overflows")
	}
	samples := int(total >> 52)
	if samples < 1 || samples > 736 {
		return nil, errors.New("invalid bounded DSP frame sample count")
	}
	var buffer [2][]int32
	for channel := range buffer {
		buffer[channel] = make([]int32, samples+18)
		copy(buffer[channel], next.Tail[channel][:])
	}
	tick, ok := add(start, nextChipDelta(absolute))
	if !ok {
		return nil, errors.New("DSP next chip clock overflows")
	}
	addDelta := func(time uint64, delta [2]int32) {
		fixed := uint32(((time-start)*rateFactor + offset) >> 32)
		phase := int(fixed >> 15 & 31)
		fraction := int64(fixed & 32767)
		position := int(fixed >> 20)
		for channel := 0; channel < 2; channel++ {
			interpolated := int32(int64(delta[channel]) * fraction >> 15)
			main := delta[channel] - interpolated
			for tap := 0; tap < 8; tap++ {
				left := int64(stepKernel[phase][tap])*int64(main) + int64(stepKernel[phase+1][tap])*int64(interpolated)
				right := int64(stepKernel[32-phase][7-tap])*int64(main) + int64(stepKernel[31-phase][7-tap])*int64(interpolated)
				buffer[channel][position+tap] += int32(left)
				buffer[channel][position+8+tap] += int32(right)
			}
		}
	}
	advance := func(to uint64) {
		for tick < to {
			current := chipLevel(next)
			if tick < end {
				delta := [2]int32{current[0] - next.LastChipOutput[0], current[1] - next.LastChipOutput[1]}
				next.LastChipOutput = current
				if delta != [2]int32{} {
					addDelta(tick, delta)
				}
			}
			// This temporarynext tick is discarded at the reference-frame flush.
			if tick > math.MaxUint64-ChipPeriod {
				tick = math.MaxUint64
				break
			}
			tick += ChipPeriod
		}
	}
	cursor, _ := add(start, uint64(next.InputAhead))
	for _, event := range next.Pending {
		advance(event.Cycle)
		switch event.Kind {
		case DACData:
			next.DACValue = event.Value
		case DACEnable:
			next.Enabled = event.Value == 0x80
		case DACPan:
			next.Pan = event.Value
		}
		cursor = event.Cycle
	}
	advance(end)
	output := make([]int16, samples*2)
	for sample := 0; sample < samples; sample++ {
		for channel := 0; channel < 2; channel++ {
			value := next.Integrator[channel] >> 15
			next.Integrator[channel] += buffer[channel][sample]
			if value > 32767 {
				value = 32767
			} else if value < -32768 {
				value = -32768
			}
			output[sample*2+channel] = int16(value)
			next.Integrator[channel] -= value << 6
		}
	}
	for channel := range next.Tail {
		copy(next.Tail[channel][:], buffer[channel][samples:samples+18])
	}
	next.Time = end
	next.InputAhead = 0
	if cursor > end {
		next.InputAhead = uint32(cursor - end)
	}
	next.Pending = nil
	stream.state = next
	return output, nil
}
