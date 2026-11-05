// Package phaseclock expresses effects of the fixed reference timing profile.
// It does not choose event times or execute the original CPU or firmware.
package phaseclock

import (
	"errors"
	"math"
)

// DMA is the current VDP bus interval in reference-frame-relative master cycles.
// Modes 0/1 use the ROM bus; fill/copy modes 2/3 do not hold it for this access.
// Remaining controls the wait flag, independently from the current End boundary.
type DMA struct {
	End       uint32
	Mode      uint8
	Remaining uint32
}

// Effect belongs to one supplied bank transaction. The caller retains its own
// clock and wait-flag ownership; SetWaitFlag is applied with OR, never assignment.
type Effect struct {
	At          uint32
	Z80After    uint32
	M68KDelay   uint32
	SetWaitFlag bool
}

// ResolveBankAccess computes the pinned reference core's bus wait effect.
// frameCycle must be relative to the current reference frame, after the memory
// operation's base cost and before this bus effect. Absolute master time has a
// different modulo-seven phase because 896040 is not divisible by seven.
//
// This intentionally reproduces the reference's approximate bus profile; it is
// not a declaration of measured hardware timing or an independent event clock.
func ResolveBankAccess(frameCycle uint32, dma DMA) (Effect, error) {
	if dma.Mode > 3 {
		return Effect{}, errors.New("unknown reference DMA mode")
	}
	at := frameCycle
	setWait := false
	if dma.Mode < 2 && at < dma.End {
		at = dma.End
		setWait = dma.Remaining != 0
	}
	if at > math.MaxUint32-45 {
		return Effect{}, errors.New("reference bank resume cycle overflows")
	}
	return Effect{At: at, Z80After: at + 45, M68KDelay: ((at%7 + 72) / 7) * 7, SetWaitFlag: setWait}, nil
}
