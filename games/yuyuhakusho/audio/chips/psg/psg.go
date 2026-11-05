// Package psg models the integrated MD PSG as typed divider, latch and noise
// state. It emits individual amplitude changes; it contains no resampler,
// firmware, CPU interpreter, native context, or prerecorded event schedule.
package psg

import (
	"errors"
	"math"
	"sort"
)

type Profile uint8

const IntegratedHQ150Stereo Profile = 1

const (
	ClockPeriod      uint64 = 240
	FrameCycles      uint64 = 896040
	MaxAdvanceCycles uint64 = 2 * FrameCycles
	MaxInputAhead    uint64 = 1008
	maxDividerPeriod uint64 = 1023 * ClockPeriod
)

// These are floor(2800 * 10^(-attenuation/10)), with the sixteenth setting
// disconnected. The fixed reference profile amplifies each output by 150%.
var amplitude = [16]uint16{2800, 2224, 1766, 1403, 1114, 885, 703, 558, 443, 352, 280, 222, 176, 140, 111, 0}

type Event struct {
	Cycle uint64
	Data  uint8
}

// Edge preserves each individual nonzero native amplitude change. Equal-cycle
// entries must not be coalesced: fractional FIR interpolation rounds each
// delta separately. Delta is already amplified for the fixed stereo profile.
type Edge struct {
	Cycle uint64
	Delta [2]int32
}

// State owns the write-order cursor, next internal synchronization clock,
// independent divider phases, noise feedback and deferred amplitude pipeline.
// Level is the unamplified canonical amplitude, not a second attenuation copy.
// Output amplitudes and frequency increments are derived when needed.
type State struct {
	Cursor       uint64
	NextTick     uint64
	Latch        uint8
	ToneDivider  [3]uint16
	Level        [4]uint16
	NoiseControl uint8
	NoiseShift   uint16
	NextEdge     [4]uint64
	High         [4]bool
	PendingDelta [4][2]int32
}

type Stream struct{ state State }

func ResetState() State { return State{Latch: 3, NoiseShift: 1 << 15} }

func valid(state State) error {
	if state.Latch > 7 || state.NoiseControl > 7 || state.NoiseShift == 0 {
		return errors.New("invalid PSG latch or noise state")
	}
	if state.NextTick < state.Cursor || state.NextTick-state.Cursor >= ClockPeriod {
		return errors.New("invalid PSG synchronization clock")
	}
	for _, divider := range state.ToneDivider {
		if divider > 1023 {
			return errors.New("invalid PSG tone divider")
		}
	}
	for channel, level := range state.Level {
		known := false
		for _, value := range amplitude {
			known = known || value == level
		}
		if !known {
			return errors.New("invalid PSG amplitude")
		}
		edge := state.NextEdge[channel]
		if edge < state.NextTick || edge-state.NextTick > maxDividerPeriod || edge%ClockPeriod != state.NextTick%ClockPeriod {
			return errors.New("invalid PSG divider phase")
		}
		delta := state.PendingDelta[channel]
		if delta[0] != delta[1] || delta[0] < -4200 || delta[0] > 4200 {
			return errors.New("invalid PSG deferred amplitude")
		}
	}
	return nil
}

func New(profile Profile, initial State) (*Stream, error) {
	if profile != IntegratedHQ150Stereo {
		return nil, errors.New("unsupported PSG profile")
	}
	if err := valid(initial); err != nil {
		return nil, err
	}
	return &Stream{state: initial}, nil
}

func (stream *Stream) Snapshot() State { return stream.state }

func level(state *State, channel int) int32 { return int32(state.Level[channel]) * 150 / 100 }

func period(state *State, channel int) uint64 {
	if channel == 3 {
		setting := state.NoiseControl & 3
		if setting != 3 {
			return (16 << setting) * ClockPeriod
		}
		channel = 2
	}
	divider := uint64(state.ToneDivider[channel])
	if divider == 0 {
		divider = 1
	}
	return divider * ClockPeriod
}

func appendEdge(edges []Edge, cycle uint64, delta [2]int32) []Edge {
	if delta == [2]int32{} {
		return edges
	}
	return append(edges, Edge{Cycle: cycle, Delta: delta})
}

// synchronize advances the copy supplied by its caller. No failed operation
// can commit partially generated edges or a partially advanced oscillator.
func synchronize(state *State, to uint64) ([]Edge, error) {
	if to < state.Cursor {
		return nil, errors.New("PSG input time moved backwards")
	}
	if to-state.Cursor > MaxAdvanceCycles {
		return nil, errors.New("PSG advance exceeds bounded interval")
	}
	if to <= state.NextTick {
		state.Cursor = to
		return nil, nil
	}
	steps := (to-state.NextTick-1)/ClockPeriod + 1
	advance := steps * ClockPeriod
	if state.NextTick > math.MaxUint64-advance {
		return nil, errors.New("PSG synchronization clock overflows")
	}
	nextTick := state.NextTick + advance
	var edges []Edge
	for channel := 0; channel < 4; channel++ {
		edges = appendEdge(edges, state.NextTick, state.PendingDelta[channel])
		state.PendingDelta[channel] = [2]int32{}
		for state.NextEdge[channel] < to {
			clock := state.NextEdge[channel]
			state.High[channel] = !state.High[channel]
			var sign int32
			if channel < 3 {
				sign = -1
				if state.High[channel] {
					sign = 1
				}
			} else if state.High[3] {
				previous := state.NoiseShift & 1
				feedback := previous
				if state.NoiseControl&4 != 0 {
					feedback ^= (state.NoiseShift >> 3) & 1
				}
				state.NoiseShift = (state.NoiseShift >> 1) | (feedback << 15)
				sign = int32(state.NoiseShift&1) - int32(previous)
			}
			change := sign * level(state, channel)
			edges = appendEdge(edges, clock, [2]int32{change, change})
			interval := period(state, channel)
			if clock > math.MaxUint64-interval {
				return nil, errors.New("PSG divider clock overflows")
			}
			state.NextEdge[channel] += interval
		}
	}
	// The native channel traversal is not chronological. Sorting preserves its
	// ordering between distinct writes at the same timestamp while presenting
	// an ordered stream to the common resampler.
	sort.SliceStable(edges, func(i, j int) bool { return edges[i].Cycle < edges[j].Cycle })
	state.Cursor, state.NextTick = to, nextTick
	return edges, nil
}

func (stream *Stream) Advance(to uint64) ([]Edge, error) {
	next := stream.state
	edges, err := synchronize(&next, to)
	if err != nil {
		return nil, err
	}
	stream.state = next
	return edges, nil
}

// Write advances clocks strictly before the raw event time, then changes the
// selected register. The eventual amplitude change is owned by PendingDelta
// until a later internal clock is actually processed.
func (stream *Stream) Write(event Event) ([]Edge, error) {
	next := stream.state
	edges, err := synchronize(&next, event.Cycle)
	if err != nil {
		return nil, err
	}
	if event.Data&0x80 != 0 {
		next.Latch = event.Data >> 4 & 7
	}
	index := int(next.Latch)
	channel := index / 2
	switch {
	case index < 6 && index%2 == 0:
		old := next.ToneDivider[channel]
		if event.Data&0x80 != 0 {
			next.ToneDivider[channel] = old&0x3f0 | uint16(event.Data&15)
		} else {
			next.ToneDivider[channel] = old&15 | uint16(event.Data&63)<<4
		}
	case index == 6:
		if next.NoiseShift&1 != 0 {
			change := -level(&next, 3)
			for side := range next.PendingDelta[3] {
				next.PendingDelta[3][side] += change
			}
		}
		next.NoiseShift = 1 << 15
		next.NoiseControl = event.Data & 7
		if next.NoiseControl&3 == 3 {
			next.NextEdge[3] = next.NextEdge[2]
		}
	default:
		old := level(&next, channel)
		next.Level[channel] = amplitude[event.Data&15]
		active := next.High[channel]
		if channel == 3 {
			active = next.NoiseShift&1 != 0
		}
		if active {
			change := level(&next, channel) - old
			for side := range next.PendingDelta[channel] {
				next.PendingDelta[channel][side] += change
			}
		}
	}
	stream.state = next
	return edges, nil
}

// FrameBoundary represents the reference's coordinate rebase in an absolute
// timeline. It owns no second frame counter and does not flush a resampler.
// A raw write up to MaxInputAhead beyond end has already advanced this chip;
// the native frame rebase does not undo or repeat that work.
func (stream *Stream) FrameBoundary(end uint64) ([]Edge, error) {
	if end%FrameCycles != 0 {
		return nil, errors.New("invalid PSG frame boundary")
	}
	if end < stream.state.Cursor {
		if stream.state.Cursor-end > MaxInputAhead {
			return nil, errors.New("PSG frame input exceeds lookahead bound")
		}
		return nil, nil
	}
	return stream.Advance(end)
}
