package engine

import (
	"errors"
	"math"
	"math/bits"
)

// ArenaWorkOperation is one indivisible work quantum of the actual default
// arena/camera rule path. Fixed source arithmetic is compiled into that path;
// no source program, addresses or recorded operation list is loaded at runtime.
// InternalRefresh marks the camera division's internal refresh skip, which
// occurs after its dynamic work. Other operations have no such skip.
type ArenaWorkOperation struct {
	Phase           string
	CPUCycles       uint16
	InternalRefresh bool
}
type ArenaWorkHook func(ArenaWorkOperation) error

// Advance charges only this work against the caller's sole main clock. IRQ
// acceptance and interrupt service remain the caller's responsibility between
// operations; this function neither owns a second clock nor chooses IRQ times.
func (op ArenaWorkOperation) Advance(cycle, refresh uint64) (uint64, uint64, error) {
	if op.Phase == "" || op.CPUCycles < 4 || op.CPUCycles > 136 || op.CPUCycles&1 != 0 || op.InternalRefresh && op.CPUCycles < 106 {
		return 0, 0, errors.New("invalid arena work quantum")
	}
	if cycle >= refresh {
		if cycle > math.MaxUint64-896 {
			return 0, 0, errors.New("arena work refresh overflow")
		}
		refresh = cycle + 896
		cycle += 14
	}
	delta := uint64(op.CPUCycles) * 7
	if cycle > math.MaxUint64-delta {
		return 0, 0, errors.New("arena work clock overflow")
	}
	cycle += delta
	if op.InternalRefresh && cycle >= refresh {
		if refresh > math.MaxUint64-896 {
			return 0, 0, errors.New("arena internal refresh overflow")
		}
		refresh += 896
	}
	return cycle, refresh, nil
}

type arenaWorkEmitter struct {
	hook ArenaWorkHook
	err  error
}

func (w *arenaWorkEmitter) spend(phase string, cycles ...uint16) {
	if w.hook == nil || w.err != nil {
		return
	}
	for _, cost := range cycles {
		if w.err = w.hook(ArenaWorkOperation{Phase: phase, CPUCycles: cost}); w.err != nil {
			return
		}
	}
}
func (w *arenaWorkEmitter) branch(phase string, taken, word bool) {
	cost := uint16(10)
	if !taken {
		cost = 8
		if word {
			cost = 12
		}
	}
	w.spend(phase, cost)
}
func (w *arenaWorkEmitter) loop(phase string, more bool) {
	cost := uint16(14)
	if more {
		cost = 10
	}
	w.spend(phase, cost)
}
func (w *arenaWorkEmitter) division(sum, count uint16) {
	if w.hook == nil || w.err != nil {
		return
	}
	// Both sums wrap to 16 bits before division; counts are 1..16. In this
	// domain no internal carry round occurs: successful subtract rounds are
	// exactly the set bits of floor(sum/(2*count)). No operand shadow is kept.
	cost := 136 - 2*bits.OnesCount16(sum/(2*count))
	w.err = w.hook(ArenaWorkOperation{Phase: "camera-divide", CPUCycles: uint16(cost), InternalRefresh: true})
}
func (w *arenaWorkEmitter) rows(phase string, groups int, cost uint16) {
	if w.hook == nil || w.err != nil {
		return
	}
	for i := 0; i < groups; i++ {
		w.spend(phase, cost, cost, cost, cost)
		w.loop(phase, i+1 < groups)
	}
}

// WorldWorkOperation currently covers the default arena only. Absence of a
// callback does not imply zero cost for other world work or callbacks.
type WorldWorkOperation struct {
	ObjectID uint16
	Work     ArenaWorkOperation
}
type WorldWorkHook func(WorldWorkOperation, WorldSoundOwners) error
type ChuWorldHooks struct {
	Sound WorldSoundHook
	Work  WorldWorkHook
}
