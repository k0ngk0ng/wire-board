package engine

import "errors"

// ObjectDrawState is the unique current drawing owner. Counts and cursors use
// logical sprite indices, never hardware table addresses. ReservedCount is the
// number of retained leading sprites, including the first entry. WriteIndex
// advances during later submissions; FrameStartIndex stays at this frame's
// first available entry. Flags retains the drawing/overflow byte's semantics.
// The fixed layer heads and retained links are derived temporary structure.
type ObjectDrawState struct {
	ReservedCount   uint16
	Count           uint16
	WriteIndex      uint16
	FrameStartIndex uint16
	Flags           uint8
}

// ObjectDrawWorkHook borrows the staged current owner before the operation's
// semantic mutation. The hook advances the shared main clock through this work
// operation once; fields written by this operation publish after its successful
// completion. The retained-count read is latched before its hook. Hook changes
// to other fields remain live. No hook or execution record enters the state.
type ObjectDrawWorkHook func(MainWorkOperation, *ObjectDrawState) error

type objectPrepareWork struct {
	state *ObjectDrawState
	hook  ObjectDrawWorkHook
	err   error
}

func (w *objectPrepareWork) step(phase string, cost uint16, apply func()) {
	if w.err != nil {
		return
	}
	if w.hook != nil {
		w.err = w.hook(MainWorkOperation{Phase: phase, CPUCycles: cost}, w.state)
	}
	if w.err == nil && apply != nil {
		apply()
	}
}

func (w *objectPrepareWork) loop(phase string, more bool) {
	cost := uint16(14)
	if more {
		cost = 10
	}
	w.step(phase, cost, nil)
}

// PrepareObjectDrawWork rebuilds a frame's drawing chains and leading retained
// links with each actual preparation work boundary. The supported retained
// count is 1..80, matching the established H40 display capacity; only 1 and 2
// are observed in the fixed natural scenario. The derived fixed chains need
// no persistent copy. A caller that advances IRQ/audio must branch those owners
// with this state and discard that branch on any error. This function returns
// the entry state on error, including errors after partially completed work.
func PrepareObjectDrawWork(state ObjectDrawState, hook ObjectDrawWorkHook) (ObjectDrawState, error) {
	before := state
	if err := PrepareObjectDrawOnOwner(&state, hook); err != nil {
		return before, err
	}
	return state, nil
}

// PrepareObjectDrawOnOwner performs the same complete work directly on the
// caller's detached transaction owner. Every hook receives this exact pointer;
// completed field writes are visible immediately through the caller's owner.
// The hook precedes each field mutation, and the retained-count read is latched
// before its hook. No state snapshot is merged back at return. Errors preserve
// the completed work and hook changes; the caller must discard the whole joint
// graphics/clock/audio transaction instead of reusing the partially advanced
// owner. A nil owner or unsupported entry count performs no work.
func PrepareObjectDrawOnOwner(state *ObjectDrawState, hook ObjectDrawWorkHook) error {
	if state == nil {
		return errors.New("missing object drawing owner")
	}
	if state.ReservedCount < 1 || state.ReservedCount > 80 {
		return errors.New("retained sprite count exceeds supported drawing capacity")
	}
	w := objectPrepareWork{state: state, hook: hook}
	w.step("object-drawing-disable", 16, func() { state.Flags = 0 })
	w.step("object-layer-head-base", 8, nil)
	w.step("object-layer-head-source", 12, nil)
	w.step("object-layer-head-count", 8, nil)
	for layer := 0; layer < 64 && w.err == nil; layer++ {
		w.step("object-layer-head-reset", 12, nil)
		w.step("object-layer-head-next", 4, nil)
		w.loop("object-layer-head-loop", layer < 63)
	}
	w.step("object-retained-base", 8, nil)
	w.step("object-retained-count-clear", 4, nil)
	reserved := state.ReservedCount
	if w.err == nil && (reserved < 1 || reserved > 80) {
		return errors.New("live retained sprite count exceeds supported drawing capacity")
	}
	w.step("object-retained-count-read", 12, nil)
	count := uint16(0)
	for index := uint16(0); index < reserved && w.err == nil; index++ {
		w.step("object-retained-count-advance", 4, func() { count++ })
		w.step("object-retained-link-write", 12, nil)
		w.step("object-retained-next", 8, nil)
		w.loop("object-retained-loop", index+1 < reserved)
	}
	w.step("object-count-publish", 12, func() { state.Count = count })
	w.step("object-write-index-publish", 12, func() { state.WriteIndex = count })
	w.step("object-frame-start-publish", 12, func() { state.FrameStartIndex = count })
	w.step("object-drawing-enable", 16, func() { state.Flags = 128 })
	w.step("object-prepare-return", 16, nil)
	return w.err
}
