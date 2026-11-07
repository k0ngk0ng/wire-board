package engine

import (
	"math/bits"
)

// ArenaWorkOperation retains the original API on the shared main-clock work.
type ArenaWorkOperation = MainWorkOperation
type ArenaWorkHook func(ArenaWorkOperation) error

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
	Frame *WorldFrameWorkHooks
	// Owners borrows the active transaction at each internal value/clone
	// boundary. It must not retain the pointer after the enclosing frame.
	Owners func(*ChuWorldState) error
}
