// Package videoclock owns the fixed reference VDP's line counter and interrupt
// signals. The caller owns CPU execution, instruction costs, IRQ acceptance and
// the order of external line and register operations. No original code runs here.
package videoclock

import "errors"

// Profile is deliberately closed to the observed noninterlaced NTSC profile.
type Profile uint8

const NTSC224H40 Profile = 1

type Config struct{ Profile Profile }

// Phase records the externally driven frame stage. BetweenFrames follows visible
// line 223; PreVertical and PostVertical divide the first blank line at its VINT
// edge. Scanlines contains the remaining blank and visible lines.
type Phase uint8

const (
	BetweenFrames Phase = iota
	PreVertical
	PostVertical
	Scanlines
)

// State contains only counter and interrupt state. Line is the frame ordinal
// 1..261 during Scanlines and zero in the other phases. CounterReload does not
// immediately replace the running Counter.
type State struct {
	Phase             Phase
	Line              uint16
	Counter           uint8
	Reload            uint8
	HorizontalEnabled bool
	VerticalEnabled   bool
	DisplayEnabled    bool
	HorizontalPending bool
	VerticalPending   bool
}

// Effect is an operation's CPU pin request, not a summary of highest pending
// interrupt. SetIRQ replaces the external pin; UpdateIRQ ORs Level into it. The
// two operations are mutually exclusive. In particular HINT ORs level 4 while a
// VINT can remain pending. Deferred requires the external CPU owner to perform the reference's
// delayed IRQ service; this package does not execute an instruction for it.
// Cycle belongs to generated line/vertical signals and is frame-relative.
type Effect struct {
	SetIRQ     bool
	UpdateIRQ  bool
	Level      uint8
	Deferred   bool
	Horizontal bool
	Vertical   bool
	Cycle      uint32
}

type Register uint8

const (
	HorizontalControl Register = 0
	VerticalControl   Register = 1
	CounterReload     Register = 10
	WidthControl      Register = 12
)

const (
	lineCycles  uint32 = 3420
	frameCycles uint32 = 262 * lineCycles
	verticalAt  uint32 = 788
)

type Clock struct{ state State }

// New validates a checkpoint without creating any hidden frame or CPU state.
func New(config Config, state State) (*Clock, error) {
	if config.Profile != NTSC224H40 {
		return nil, errors.New("unsupported VDP counter profile")
	}
	switch state.Phase {
	case BetweenFrames, PreVertical, PostVertical:
		if state.Line != 0 {
			return nil, errors.New("noncanonical line outside scanline phase")
		}
	case Scanlines:
		if state.Line == 0 || state.Line >= 262 {
			return nil, errors.New("VDP line ordinal outside scanline phase")
		}
	default:
		return nil, errors.New("unknown VDP counter phase")
	}
	return &Clock{state: state}, nil
}

func (c *Clock) Snapshot() State { return c.state }

// BeginFrame enters the first blank line. This line can assert HINT, but does
// not reload or decrement the counter. VerticalEdge must be called separately
// after the external CPU's first execution batch, including any overrun.
func (c *Clock) BeginFrame() (Effect, error) {
	if c.state.Phase != BetweenFrames {
		return Effect{}, errors.New("VDP frame is already active")
	}
	c.state.Phase = PreVertical
	if c.state.Counter == 0 {
		return c.horizontal(0), nil
	}
	return Effect{}, nil
}

func (c *Clock) VerticalEdge() (Effect, error) {
	if c.state.Phase != PreVertical {
		return Effect{}, errors.New("VINT edge outside first blank-line stage")
	}
	c.state.Phase = PostVertical
	c.state.VerticalPending = true
	return Effect{SetIRQ: c.state.VerticalEnabled, Level: 6, Vertical: true, Cycle: verticalAt}, nil
}

// NextLine follows original call order rather than inferring stage changes from
// an instruction's raw cycle. Blank lines preserve Counter; the last blank line
// reloads it before the active lines decrement it or raise/reload HINT.
func (c *Clock) NextLine() (Effect, error) {
	if c.state.Phase != PostVertical && c.state.Phase != Scanlines {
		return Effect{}, errors.New("scanline advance outside an active post-VINT stage")
	}
	next := c.state.Line + 1
	if next == 262 {
		c.state.Phase = BetweenFrames
		c.state.Line = 0
		return Effect{}, nil
	}
	c.state.Phase = Scanlines
	c.state.Line = next
	if next == 37 {
		c.state.Counter = c.state.Reload
	} else if next >= 38 {
		if c.state.Counter == 0 {
			c.state.Counter = c.state.Reload
			return c.horizontal(uint32(next) * lineCycles), nil
		}
		c.state.Counter--
	}
	return Effect{}, nil
}

func (c *Clock) horizontal(cycle uint32) Effect {
	c.state.HorizontalPending = true
	return Effect{UpdateIRQ: c.state.HorizontalEnabled, Level: 4, Horizontal: true, Cycle: cycle}
}

// WriteRegister accepts the four semantic registers used by this counter
// profile. Unrelated control bits are not retained; unsupported counter-latch,
// height, width or interlace changes fail before changing any state.
func (c *Clock) WriteRegister(register Register, value uint8) (Effect, error) {
	switch register {
	case HorizontalControl:
		if value&2 != 0 {
			return Effect{}, errors.New("latched HV counters are unsupported")
		}
		enabled := value&16 != 0
		changed := enabled != c.state.HorizontalEnabled
		c.state.HorizontalEnabled = enabled
		if changed && c.state.HorizontalPending {
			if c.state.VerticalEnabled && c.state.VerticalPending {
				return Effect{SetIRQ: true, Level: 6}, nil
			}
			if enabled {
				return Effect{SetIRQ: true, Level: 4, Deferred: true}, nil
			}
			return Effect{SetIRQ: true}, nil
		}
	case VerticalControl:
		if value&4 == 0 || value&8 != 0 {
			return Effect{}, errors.New("only mode 5 with 224 visible lines is supported")
		}
		enabled := value&32 != 0
		changed := enabled != c.state.VerticalEnabled
		c.state.VerticalEnabled = enabled
		c.state.DisplayEnabled = value&64 != 0
		if changed && c.state.VerticalPending {
			if enabled {
				return Effect{SetIRQ: true, Level: 6, Deferred: true}, nil
			}
			if c.state.HorizontalEnabled && c.state.HorizontalPending {
				return Effect{SetIRQ: true, Level: 4}, nil
			}
			return Effect{SetIRQ: true}, nil
		}
	case CounterReload:
		c.state.Reload = value
	case WidthControl:
		if value&1 == 0 || value&6 != 0 {
			return Effect{}, errors.New("only noninterlaced H40 is supported")
		}
	default:
		return Effect{}, errors.New("unsupported VDP counter register")
	}
	return Effect{}, nil
}

// Acknowledge follows the reference VDP's enabled-VINT priority irrespective of
// which level the external CPU accepted. All other acknowledgements clear HINT
// and lower the requested pin; a disabled VINT remains pending.
func (c *Clock) Acknowledge() (Effect, error) {
	if c.state.VerticalEnabled && c.state.VerticalPending {
		c.state.VerticalPending = false
		if c.state.HorizontalEnabled && c.state.HorizontalPending {
			return Effect{SetIRQ: true, Level: 4}, nil
		}
	} else {
		c.state.HorizontalPending = false
	}
	return Effect{SetIRQ: true}, nil
}

func (c *Clock) VCounter() uint16 {
	if c.state.Phase == BetweenFrames {
		return 223
	}
	if c.state.Line < 38 {
		return 224 + c.state.Line
	}
	return c.state.Line - 38
}

func (c *Clock) LineCycle() uint32 {
	if c.state.Phase == BetweenFrames {
		return frameCycles
	}
	return uint32(c.state.Line) * lineCycles
}

// StatusBits returns only the stored VBlank/VINT mask 0x88. Pixel, FIFO, DMA,
// collision and HBlank status are outside this package's contract.
func (c *Clock) StatusBits() uint8 {
	var bits uint8
	if c.state.Phase != BetweenFrames && c.state.Line < 37 {
		bits |= 8
	}
	if c.state.VerticalPending {
		bits |= 128
	}
	return bits
}

// ReadStatus is a M68K status read's 0x88 projection. It neither acknowledges an
// interrupt nor clears pending. The early VINT bit uses raw plus instruction
// cycles before the explicit edge; after that edge the asserted Z80 IRQ prevents
// a cleared VINT from being reported again on the first blank line.
func (c *Clock) ReadStatus(rawCycle, instructionCycles uint32) (uint8, error) {
	if c.state.Phase == BetweenFrames || rawCycle < c.LineCycle() || instructionCycles > ^uint32(0)-rawCycle {
		return 0, errors.New("invalid active VDP status-read cycle")
	}
	bits := c.StatusBits()
	if !c.state.DisplayEnabled {
		bits |= 8
	}
	if c.state.Phase == PreVertical && rawCycle+instructionCycles >= verticalAt {
		bits |= 128
	}
	return bits, nil
}

// ReadVertical returns the high byte of an unlatched M68K HVC read. It uses the
// raw CPU cycle, without instruction-cycle lookahead, and advances at most one
// line on overrun before applying the NTSC-224 counter discontinuity.
func (c *Clock) ReadVertical(rawCycle uint32) (uint8, error) {
	if c.state.Phase == BetweenFrames || rawCycle < c.LineCycle() {
		return 0, errors.New("invalid active VDP vertical-read cycle")
	}
	vc := int(c.VCounter())
	if rawCycle-c.LineCycle() >= lineCycles {
		vc = (vc + 1) % 262
	}
	if vc > 234 {
		vc -= 262
	}
	return uint8(vc), nil
}
