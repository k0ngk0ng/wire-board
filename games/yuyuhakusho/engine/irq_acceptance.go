package engine

import (
	"errors"
	"math"
)

// WorldIRQAcceptance is a transient normal execution-batch entry result. The
// caller retains the sole clock, pin, mask and VDP owners. Accepting an IRQ
// neither acknowledges the VDP nor executes the RAM IRQ prefix or handler.
type WorldIRQAcceptance struct {
	Accepted bool
	Cycle    uint64
	Mask     uint8
}

// AcceptWorldIRQ implements the fixed battle profile's ordinary 4/6-level pin
// acceptance at a new main execution batch. The caller must first finish the
// actual current work quantum and deliver any VDP signal. Deferred register
// writes and acceptance inside a mask-changing operation are separate paths.
func AcceptWorldIRQ(cycle uint64, level, mask uint8) (WorldIRQAcceptance, error) {
	if level != 0 && level != 4 && level != 6 || mask > 7 {
		return WorldIRQAcceptance{}, errors.New("unsupported battle IRQ pin or mask")
	}
	result := WorldIRQAcceptance{Cycle: cycle, Mask: mask}
	if level <= mask {
		return result, nil
	}
	// Ordinary entry aligns the CPU phase after 50 cycles, then waits to the
	// next ten-cycle boundary when needed. Keep the existing sub-CPU remainder
	// of the master clock; interrupt entry does not perform a bus refresh.
	phase := (cycle / 7) % 10
	cycles := uint64(50)
	if phase != 0 {
		cycles = 60 - phase
	}
	delay := cycles * 7
	if cycle > math.MaxUint64-delay {
		return WorldIRQAcceptance{}, errors.New("world IRQ entry clock overflow")
	}
	result.Accepted = true
	result.Cycle += delay
	result.Mask = level
	return result, nil
}
