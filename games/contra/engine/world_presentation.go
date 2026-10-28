package contra

import "errors"

// WorldPresentation owns pending and applied visual state across world ticks.
// Simulation writes these owners; the next display boundary submits them before
// new player/enemy work. The same boundary remains active while play is paused.
type WorldPresentation struct {
	Background             [2048]uint8
	Display                DisplayState
	AppliedOAM             [256]uint8
	AppliedScroll          CollisionScroll
	Sprites                SpriteCompositionState
	Patterns               PatternPages
	Palette                PaletteOutput
	Queue                  GraphicsQueue
	Terminated             bool
	DiscardedGraphicsBytes uint8
}

// Seal represents CB1E's final zero command. It is not a graphics packet, but
// its byte counts toward the next boundary's palette admission threshold.
func (p *WorldPresentation) Seal() error {
	if p == nil || p.Terminated || int(p.Queue.Used)+int(p.DiscardedGraphicsBytes) >= GraphicsBufferCapacity {
		return errors.New("invalid world graphics termination")
	}
	p.Terminated = true
	return nil
}

// PresentedFrame is an immutable-by-ownership submission. Background writes are
// deltas consumed in order by the scene; sprites/patterns/palette are snapshots.
// A reconnect must pair it with the scene's accumulated background state.
type PresentedFrame struct {
	Background [2048]uint8
	Display    DisplayState
	Scroll     CollisionScroll
	OAM        [256]uint8
	Patterns   [8]uint8
	Palette    [32]uint8
	Writes     []BackgroundWrite
}

// Commit submits previous simulation work atomically. Keep the byte cursor until
// after Palette.Commit: the original gate defers colors at 71 graphics bytes.
func (p *WorldPresentation) Commit() (PresentedFrame, error) {
	if p == nil {
		return PresentedFrame{}, errors.New("missing world presentation")
	}
	raw, err := p.Queue.Encode()
	if err != nil {
		return PresentedFrame{}, err
	}
	used := len(raw)
	used += int(p.DiscardedGraphicsBytes)
	next := *p
	next.Display.SplitActive, next.Display.RasterActive = false, false
	if next.Terminated {
		used++
	}
	if used > GraphicsBufferCapacity {
		return PresentedFrame{}, errors.New("graphics terminator overlaps palette")
	}
	if _, err := next.Palette.Commit(uint8(used)); err != nil {
		return PresentedFrame{}, err
	}
	writes, err := next.Queue.DecodedWrites()
	if err != nil {
		return PresentedFrame{}, err
	}
	if err := next.ApplyImmediate(writes); err != nil {
		return PresentedFrame{}, err
	}
	if next.Display.Busy != 0 {
		next.Display.Busy--
	}
	next.Display.AppliedMask = next.Display.Mask
	if next.Display.Busy != 0 {
		next.Display.AppliedMask = 0
	}
	next.Patterns.Commit()
	next.AppliedOAM = next.Sprites.OAM
	out := PresentedFrame{OAM: next.Sprites.OAM, Patterns: next.Patterns.Applied,
		Palette: next.Palette.Applied, Writes: writes, Background: next.Background, Display: next.Display, Scroll: next.AppliedScroll}
	next.Queue.Reset()
	next.Terminated = false
	next.DiscardedGraphicsBytes = 0
	*p = next
	return out, nil
}

// CD0E terminates the pending stream at its first byte without rewinding $21.
// Keep only its relevant admission cost, not ignored stale CPU-buffer bytes.
func (p *WorldPresentation) DiscardGraphics(mode GraphicsQueueMode) error {
	if p == nil || p.Terminated || (mode != GraphicsText && mode != GraphicsRectangular) {
		return errors.New("invalid graphics discard")
	}
	used := int(p.DiscardedGraphicsBytes) + int(p.Queue.Used)
	if used > GraphicsBufferCapacity {
		return errors.New("discarded graphics exceed storage")
	}
	p.DiscardedGraphicsBytes = uint8(used)
	p.Queue.Reset()
	p.Queue.Mode = mode
	return nil
}

func (p *WorldPresentation) CommitWithScroll(scroll CollisionScroll) (PresentedFrame, error) {
	if p == nil {
		return PresentedFrame{}, errors.New("missing world presentation")
	}
	next := *p
	next.AppliedScroll = scroll
	out, err := next.Commit()
	if err != nil {
		return PresentedFrame{}, err
	}
	*p = next
	return out, nil
}

func (p WorldPresentation) Snapshot() PresentedFrame {
	return PresentedFrame{OAM: p.AppliedOAM, Patterns: p.Patterns.Applied, Palette: p.Palette.Applied, Background: p.Background, Display: p.Display, Scroll: p.AppliedScroll}
}
