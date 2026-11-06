package scoreclock

import (
	"errors"
	"math"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/videoclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type BattleWorldTransfer struct {
	Words, SourceWord uint32
	Address           uint16
	Step              uint8
	Target            videoclock.DMATarget
}
type BattleVDPLayout struct {
	Planes                                                   [2]uint16
	Window, Sprites, HScroll                                 uint16
	PlaneSize, WindowHorizontal, WindowVertical, CounterBase uint8
}

// Graphics owns pending transfers and the caller's display/color latches.
// CachedDisplay is the desired value, which can change before its port write;
// the actual hardware display bit stays with videoclock.State.
type BattleGraphics struct {
	Pending                      []BattleWorldTransfer
	UploadPending, CachedDisplay bool
	SceneIRQReady                bool `json:",omitempty"`
	SavedColors                  [2]uint16
	SpriteMarker                 uint16
	Layout                       BattleVDPLayout
}

// DMAWindows is zero or one current VDP bus state, never a transfer history.
// A late host write affects the background's earlier frontier in this fixed
// reference profile. The caller retains this owner between main operations.
type BattleMainContext struct {
	IRQ        BattleSampleContext
	DMAWindows []pcmcommands.DMAInterval
}
type BattleWorldClockDevice interface {
	BattleWorldDevice
	RunDMAUntil(uint64, []pcmcommands.DMAInterval) (pcmcommands.RunResult, error)
	DMAWaiting() bool
	AdvanceDMA(uint64, uint32) error
	FrameBoundary() (mixed.Frame, error)
}
type BattleWorldDispatch struct {
	Clock                       Clock
	Context                     BattleMainContext
	Operations                  uint32
	AwaitingLogic, BusyReturned bool
	Frames                      []mixed.Frame
}
type battleDMAWork struct {
	target    videoclock.DMATarget
	remaining uint32
}
type battleWorldClock struct {
	p                  *battleSample
	device             BattleWorldClockDevice
	graphics           *BattleGraphics
	flags              *BattleWorldFlags
	world              *engine.ChuWorldState
	windows            []pcmcommands.DMAInterval
	active             *battleDMAWork
	frames             []mixed.Frame
	pendingVertical    bool
	verticalReturnMask uint8
}

var ErrBattleWorldProfile = errors.New("unsupported timed battle World profile")

func newWorldClock(s *service, context BattleMainContext, graphics *BattleGraphics, flags *BattleWorldFlags, world *engine.ChuWorldState, device BattleWorldClockDevice, observer BattleWorldObserver) (*battleWorldClock, error) {
	return newWorldClockEntry(s, context, graphics, flags, world, device, observer, 7)
}
func newWorldClockEntry(s *service, context BattleMainContext, graphics *BattleGraphics, flags *BattleWorldFlags, world *engine.ChuWorldState, device BattleWorldClockDevice, observer BattleWorldObserver, entryMask uint8) (*battleWorldClock, error) {
	c := context.IRQ
	if graphics == nil || flags == nil || world == nil || device == nil || c.Mask != entryMask || c.Phase != 2 || c.Pin != 0 && c.Pin != 4 && c.Pin != 6 || c.Video.Phase != videoclock.Scanlines || c.FrameOrigin > math.MaxUint64-4*896040 || len(graphics.Pending) > 256 || len(context.DMAWindows) > 1 || world.SceneMode != 52 || world.SceneRequest != 52 || world.InterruptMode != 4 {
		return nil, ErrBattleWorldProfile
	}
	for _, t := range graphics.Pending {
		if t.Words == 0 || t.Words > 65536 || t.SourceWord >= 0x800000 || t.Target < videoclock.DMAVRAM || t.Target > videoclock.DMAVSRAM {
			return nil, ErrBattleWorldProfile
		}
	}
	for _, v := range context.DMAWindows {
		if v.Begin != 0 || v.End > 896040 || v.Mode > 1 || v.Remaining != 0 {
			return nil, ErrBattleWorldProfile
		}
	}
	video, err := videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, c.Video)
	if err != nil {
		return nil, err
	}
	start := c.FrameOrigin + uint64(video.LineCycle())
	if s.clock.Cycle < start || s.clock.Cycle >= start+3420 {
		return nil, ErrBattleWorldProfile
	}
	if device.PCMFrontier() < start {
		held, err := device.BusGranted(s.clock.Cycle)
		if err != nil {
			return nil, err
		}
		if !held {
			return nil, ErrBattleWorldProfile
		}
	}
	p := &battleSample{s: s, context: c, video: video, aux: device}
	w := &battleWorldClock{p: p, device: device, graphics: graphics, flags: flags, world: world, windows: append([]pcmcommands.DMAInterval(nil), context.DMAWindows...)}
	if observer != nil {
		p.observer = func(e BattleSampleObservation) error {
			return observer(BattleWorldObservation{Stage: e.Stage, Clock: e.Clock, Context: e.Context, Flags: *flags, Work: e.Work, IO: e.IO})
		}
	}
	s.before = w.before
	p.mainSync = w.sync
	return w, nil
}
func (w *battleWorldClock) snapshot() BattleMainContext {
	return BattleMainContext{IRQ: w.p.snapshot(), DMAWindows: append([]pcmcommands.DMAInterval(nil), w.windows...)}
}

func (w *battleWorldClock) dma(target videoclock.DMATarget, remaining uint32, at uint32) error {
	video := w.p.video
	// VBlank is effectively blank even while the cached register is enabled.
	// Project that effective condition without changing the actual video owner.
	if video.StatusBits()&8 != 0 && video.Snapshot().DisplayEnabled {
		state := video.Snapshot()
		state.DisplayEnabled = false
		var err error
		video, err = videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, state)
		if err != nil {
			return err
		}
	}
	effect, err := video.AdvanceForcedBlankDMA(target, remaining, at, w.device.DMAWaiting())
	if err != nil {
		return err
	}
	end := w.p.context.FrameOrigin + uint64(effect.End)
	if end < w.p.context.FrameOrigin {
		return ErrBattleWorldProfile
	}
	if err := w.device.AdvanceDMA(end, effect.Remaining); err != nil {
		return err
	}
	w.windows = []pcmcommands.DMAInterval{{End: effect.End, Mode: effect.Mode, Remaining: effect.Remaining}}
	w.p.s.clock.Cycle = end
	if effect.Remaining != 0 {
		w.active = &battleDMAWork{target, effect.Remaining}
	} else {
		w.active = nil
	}
	return nil
}

func (w *battleWorldClock) sync() error {
	p := w.p
	for {
		state := p.video.Snapshot()
		target := p.context.FrameOrigin + uint64(p.video.LineCycle()) + 3420
		if state.Phase == videoclock.PreVertical {
			target = p.context.FrameOrigin + 788
		}
		if p.s.clock.Cycle < target {
			return nil
		}
		result, err := w.device.RunDMAUntil(target, w.windows)
		if err != nil {
			return err
		}
		if err := p.s.add(result.M68KDelay); err != nil {
			return err
		}
		if state.Phase == videoclock.PreVertical {
			effect, err := p.video.VerticalEdge()
			if err != nil {
				return err
			}
			if err := p.effect(effect); err != nil {
				return err
			}
			continue
		}
		effect, err := p.video.NextLine()
		if err != nil {
			return err
		}
		if err := p.effect(effect); err != nil {
			return err
		}
		if p.video.Snapshot().Phase == videoclock.BetweenFrames {
			frame, err := w.device.FrameBoundary()
			if err != nil {
				return err
			}
			w.frames = append(w.frames, frame)
			if p.context.FrameOrigin > math.MaxUint64-896040 {
				return ErrBattleWorldProfile
			}
			p.context.FrameOrigin += 896040
			w.windows = nil
			effect, err = p.video.BeginFrame()
			if err != nil {
				return err
			}
			if err := p.effect(effect); err != nil {
				return err
			}
		}
		if w.active != nil {
			active := *w.active
			if err := w.dma(active.target, active.remaining, p.video.LineCycle()); err != nil {
				return err
			}
		}
	}
}
func (w *battleWorldClock) accept() (bool, error) {
	p := w.p
	// The original CPU is rebased each video frame. Its modulo-seven/ten
	// acceptance phase must use that coordinate, while the sole clock is global.
	irq, err := engine.AcceptWorldIRQ(p.s.clock.Cycle-p.context.FrameOrigin, p.context.Pin, p.context.Mask)
	if err != nil {
		return false, err
	}
	if !irq.Accepted {
		return false, nil
	}
	if irq.Mask != 6 {
		return false, ErrBattleWorldWork
	}
	w.verticalReturnMask = p.context.Mask
	p.context.Mask = irq.Mask
	effect, err := p.video.Acknowledge()
	if err != nil {
		return false, err
	}
	if err := p.effect(effect); err != nil {
		return false, err
	}
	p.s.clock.Cycle = p.context.FrameOrigin + irq.Cycle
	w.pendingVertical = true
	return true, nil
}
func (w *battleWorldClock) before() error {
	if err := w.sync(); err != nil {
		return err
	}
	work := w.p.work
	accepted, err := w.accept()
	if err != nil {
		return err
	}
	if accepted {
		w.p.work = nil
		err = w.handleVertical()
		w.p.work = work
		if err != nil {
			return err
		}
		if err := w.sync(); err != nil {
			return err
		}
	}
	if work != nil && w.p.observer != nil {
		return w.p.observer(BattleSampleObservation{Stage: "cpu", Clock: w.p.s.clock, Context: w.p.snapshot(), Work: *work})
	}
	return nil
}
func (w *battleWorldClock) restoreMask(phase string, cost uint16, mask uint8) error {
	if err := w.p.step(engine.MainWorkOperation{Phase: phase, CPUCycles: cost}, func() error { w.p.context.Mask = mask; _, err := w.accept(); return err }); err != nil {
		return err
	}
	return w.handleVertical()
}
func (w *battleWorldClock) displayValue(dma bool) uint8 {
	value := uint8(4)
	if w.p.video.Snapshot().VerticalEnabled {
		value |= 32
	}
	if w.graphics.CachedDisplay {
		value |= 64
	}
	if dma {
		value |= 16
	}
	return value
}
func (w *battleWorldClock) control(phase string, cost uint16, value uint32, width uint8) error {
	return w.p.step(engine.MainWorkOperation{Phase: phase, CPUCycles: cost}, func() error {
		if err := w.p.io(BattleSampleIO{Kind: "vdp-control", Write: true, Value: value, Width: width}); err != nil {
			return err
		}
		if width == 2 && value&0xc000 == 0x8000 {
			register := videoclock.Register((value >> 8) & 31)
			if register == videoclock.HorizontalControl || register == videoclock.VerticalControl || register == videoclock.CounterReload || register == videoclock.WidthControl {
				effect, err := w.p.video.WriteRegister(register, uint8(value))
				if err != nil {
					return err
				}
				return w.p.effect(effect)
			}
		}
		return nil
	})
}
func (w *battleWorldClock) data(phase string, cost uint16, value uint16) error {
	return w.p.step(engine.MainWorkOperation{Phase: phase, CPUCycles: cost}, func() error {
		return w.p.io(BattleSampleIO{Kind: "palette-data", Write: true, Value: uint32(value), Width: 2})
	})
}
func transferCommand(t BattleWorldTransfer) uint32 {
	upper := uint32(t.Address&0x3fff) | 0x4000
	lower := uint32(t.Address>>14) | 0x80
	if t.Target == videoclock.DMACRAM {
		upper |= 0x8000
	}
	if t.Target == videoclock.DMAVSRAM {
		lower |= 0x10
	}
	return upper<<16 | lower
}
func (w *battleWorldClock) transfer(t BattleWorldTransfer) error {
	p := w.p
	mask := p.context.Mask
	if err := p.spend("save-transfer-mask", 14); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "mask-transfer", CPUCycles: 20}, func() error { p.context.Mask = 7; return nil }); err != nil {
		return err
	}
	if err := p.spend("transfer-display-setup", 12, 10); err != nil {
		return err
	}
	if err := w.control("enable-transfer", 8, 0x8100|uint32(w.displayValue(true)), 2); err != nil {
		return err
	}
	length := t.Words & 65535
	if err := w.control("transfer-length", 20, (0x9300|length&255)<<16|0x9400|length>>8, 4); err != nil {
		return err
	}
	if err := w.control("transfer-source-low", 20, (0x9500|t.SourceWord&255)<<16|0x9600|(t.SourceWord>>8)&255, 4); err != nil {
		return err
	}
	if err := w.control("transfer-source-high-step", 20, (0x9700|t.SourceWord>>16)<<16|0x8f00|uint32(t.Step), 4); err != nil {
		return err
	}
	command := transferCommand(t)
	if err := w.control("transfer-destination-high", 12, command>>16, 2); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "transfer-destination-low", CPUCycles: 12}, func() error {
		if err := p.io(BattleSampleIO{Kind: "vdp-control", Write: true, Value: command & 65535, Width: 2}); err != nil {
			return err
		}
		return w.dma(t.Target, t.Words, uint32(p.s.clock.Cycle-p.context.FrameOrigin))
	}); err != nil {
		return err
	}
	if err := w.control("restore-transfer-display", 16, 0x8100|uint32(w.displayValue(false)), 2); err != nil {
		return err
	}
	return w.restoreMask("restore-transfer-mask", 16, mask)
}
func (w *battleWorldClock) flushTransfers() error {
	p := w.p
	if err := p.spend("transfer-queue-test", 8, 12, 6); err != nil {
		return err
	}
	empty := len(w.graphics.Pending) == 0
	if err := p.branch("transfer-queue-empty", empty, false); err != nil {
		return err
	}
	if !empty {
		items := w.graphics.Pending
		if err := p.step(engine.MainWorkOperation{Phase: "consume-transfer-queue", CPUCycles: 12}, func() error { w.graphics.Pending = nil; return nil }); err != nil {
			return err
		}
		if err := p.spend("reset-transfer-destination", 16, 12); err != nil {
			return err
		}
		for index, t := range items {
			if err := w.transfer(t); err != nil {
				return err
			}
			if err := p.spend("transfer-queue-remaining", 6); err != nil {
				return err
			}
			if err := p.branch("transfer-queue-loop", index+1 < len(items), false); err != nil {
				return err
			}
		}
		if err := p.step(engine.MainWorkOperation{Phase: "clear-transfer-upload", CPUCycles: 20}, func() error { w.graphics.UploadPending = false; return nil }); err != nil {
			return err
		}
	} else {
		w.graphics.Pending = nil
	}
	return p.spend("return-transfer-flush", 16)
}
func (w *battleWorldClock) releaseGate() error {
	p := w.p
	if err := p.worldBus(w.device, true); err != nil {
		return err
	}
	if err := p.worldGate(w.device, false); err != nil {
		return err
	}
	var paused bool
	if err := p.step(engine.MainWorkOperation{Phase: "world-pause-read", CPUCycles: 16}, func() error { paused = w.world.Clock.AudioPause == 255; return nil }); err != nil {
		return err
	}
	if err := p.branch("world-paused-bus", paused, true); err != nil {
		return err
	}
	if !paused {
		return p.worldBus(w.device, false)
	}
	return nil
}
func (w *battleWorldClock) run() (BattleWorldDispatch, error) {
	p := w.p
	if err := p.spend("world-active-test", 12); err != nil {
		return BattleWorldDispatch{}, err
	}
	busy := w.flags.WorldActive
	if err := p.branch("world-active-branch", !busy, false); err != nil {
		return BattleWorldDispatch{}, err
	}
	if busy {
		if err := p.step(engine.MainWorkOperation{Phase: "world-missed", CPUCycles: 20}, func() error { w.flags.MissedWorld = true; return nil }); err != nil {
			return BattleWorldDispatch{}, err
		}
		if err := w.releaseGate(); err != nil {
			return BattleWorldDispatch{}, err
		}
	} else {
		if err := p.step(engine.MainWorkOperation{Phase: "world-active-start", CPUCycles: 20}, func() error { w.flags.WorldActive = true; return nil }); err != nil {
			return BattleWorldDispatch{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "world-missed-clear", CPUCycles: 20}, func() error { w.flags.MissedWorld = false; return nil }); err != nil {
			return BattleWorldDispatch{}, err
		}
		if err := w.restoreMask("unmask-world-transfer", 20, 0); err != nil {
			return BattleWorldDispatch{}, err
		}
		if err := p.spend("call-transfer-flush", 20); err != nil {
			return BattleWorldDispatch{}, err
		}
		if err := w.flushTransfers(); err != nil {
			return BattleWorldDispatch{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "mask-world-gate-release", CPUCycles: 20}, func() error { p.context.Mask = 7; return nil }); err != nil {
			return BattleWorldDispatch{}, err
		}
		if err := w.releaseGate(); err != nil {
			return BattleWorldDispatch{}, err
		}
	}
	if err := p.step(engine.MainWorkOperation{Phase: "world-transfer-finished", CPUCycles: 20}, func() error { w.flags.TransferActive = false; return nil }); err != nil {
		return BattleWorldDispatch{}, err
	}
	if err := w.restoreMask("unmask-world-dispatch", 20, 0); err != nil {
		return BattleWorldDispatch{}, err
	}
	if busy {
		if err := p.spend("return-busy-world", 16); err != nil {
			return BattleWorldDispatch{}, err
		}
	} else {
		if err := p.spend("call-logical-world", 20); err != nil {
			return BattleWorldDispatch{}, err
		}
	}
	if err := w.sync(); err != nil {
		return BattleWorldDispatch{}, err
	}
	return BattleWorldDispatch{Clock: p.s.clock, Context: w.snapshot(), Operations: p.s.operations, AwaitingLogic: !busy, BusyReturned: busy, Frames: w.frames}, nil
}

// AdvanceBattleWorldWrapper runs the actual transfer wrapper, including frame
// flushes and interrupted steady battle VBlank setup. It stops before logical
// World execution, or before the outer IRQ epilogue after a busy return. All
// supplied owners must be detached; session handles their joint commit.
func AdvanceBattleWorldWrapper(clock Clock, context BattleMainContext, graphics *BattleGraphics, flags *BattleWorldFlags, world *engine.ChuWorldState, device BattleWorldClockDevice, observer BattleWorldObserver, limits Limits) (BattleWorldDispatch, error) {
	s, err := tickService(clock, limits)
	if err != nil {
		return BattleWorldDispatch{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	w, err := newWorldClock(s, context, graphics, flags, world, device, observer)
	if err != nil {
		return BattleWorldDispatch{}, err
	}
	return w.run()
}
