package videoclock

import "errors"

type DMATarget uint8

const (
	DMAVRAM DMATarget = iota + 1
	DMACRAM
	DMAVSRAM
)

// DMASegment is one actual VDP batch's remaining transfer and bus effect.
// End is frame-relative. The main owner assigns its cursor to End; an already
// waiting auxiliary owner does the same when MoveAux is true. These are not
// additive delays, and must not be clamped to an instruction's old tail.
type DMASegment struct {
	End, Remaining      uint32
	Mode                uint8
	MoveAux, AuxWaiting bool
}

// AdvanceForcedBlankDMA prices one RAM/ROM-bus DMA segment for the battle's
// noninterlaced H40, display-disabled profile. The caller supplies the real
// transfer length and auxiliary wait latch. This method does not run CPUs,
// copy VDP memory, advance scanlines, consume a queue or flush a frame.
//
// The first blank region can complete more than one line at once. Elsewhere
// the budget ends at the current line. A write's refresh may put its access up
// to thirteen master cycles beyond that line. Preserve the reference's uint32
// product at that edge and its strict comparison, including a zero-byte,
// full-budget segment. Ordinary signed arithmetic gives different durations.
func (c *Clock) AdvanceForcedBlankDMA(target DMATarget, remaining, at uint32, auxWaiting bool) (DMASegment, error) {
	if c.state.Phase == BetweenFrames || c.state.DisplayEnabled || remaining == 0 || remaining > 65536 || target < DMAVRAM || target > DMAVSRAM {
		return DMASegment{}, errors.New("unsupported forced-blank DMA profile")
	}
	start := c.LineCycle()
	if at < start || at > start+lineCycles+13 {
		return DMASegment{}, errors.New("DMA access leaves the current VDP batch")
	}
	rate, mode := int64(102), uint8(1)
	if target != DMAVRAM {
		rate, mode = 198, 0
	}
	end := int64(start + lineCycles)
	if c.StatusBits()&8 != 0 {
		end = 37 * int64(lineCycles)
	}
	cycles := end - int64(at)
	count := int64(uint32(cycles) * uint32(rate) / lineCycles)
	if int64(remaining) < count {
		count = int64(remaining)
		cycles = count * int64(lineCycles) / rate
	}
	left := remaining
	if count > 0 {
		left -= uint32(count)
	}
	return DMASegment{End: uint32(int64(at) + cycles), Remaining: left, Mode: mode, MoveAux: auxWaiting, AuxWaiting: auxWaiting && left != 0}, nil
}
