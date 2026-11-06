package scoreclock

import (
	"errors"
	"math"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/videoclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// BattleWorldFlags borrows the main caller's three independent wrapper flags.
// It is not a second copy in the audio checkpoint.
type BattleWorldFlags struct {
	TransferActive, WorldActive, MissedWorld bool
}
type BattleWorldInput struct{ DisplayReady bool }
type BattleWorldDevice interface {
	SampleAuxiliary
	RequestBus(uint64, bool) (uint64, error)
	BusGranted(uint64) (bool, error)
	SetBankBlocked(uint64, bool) error
}
type BattleWorldObservation struct {
	Stage   string
	Clock   Clock
	Context BattleSampleContext
	Flags   BattleWorldFlags
	Work    engine.MainWorkOperation
	IO      *BattleSampleIO
}
type BattleWorldObserver func(BattleWorldObservation) error
type BattleWorldEntry struct {
	Clock         Clock
	Context       BattleSampleContext
	Operations    uint32
	AwaitingWorld bool
}

var ErrBattleWorldContext = errors.New("unsupported battle World IRQ context")

func newBattleWorld(s *service, context BattleSampleContext, flags *BattleWorldFlags, pause *uint8, device BattleWorldDevice, observer BattleWorldObserver) (*battleSample, error) {
	if flags == nil || pause == nil || device == nil || context.Mask != 4 || context.Phase != 2 || context.Pin != 0 && context.Pin != 4 && context.Pin != 6 || context.Video.Phase != videoclock.Scanlines || context.FrameOrigin > math.MaxUint64-896040 {
		return nil, ErrBattleWorldContext
	}
	video, err := videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, context.Video)
	if err != nil {
		return nil, err
	}
	start := context.FrameOrigin + uint64(video.LineCycle())
	if s.clock.Cycle < start || s.clock.Cycle >= start+3420 {
		return nil, ErrBattleWorldContext
	}
	// A paused, physically held player can legitimately retain a frontier from
	// an earlier line. The sampling constructor's running-player check stays
	// strict; only this actual held-owner path admits the lag.
	if device.PCMFrontier() < start {
		held, err := device.BusGranted(s.clock.Cycle)
		if err != nil {
			return nil, err
		}
		if !held {
			return nil, ErrBattleWorldContext
		}
	}
	p := &battleSample{s: s, context: context, video: video, aux: device}
	if observer != nil {
		p.observer = func(e BattleSampleObservation) error {
			return observer(BattleWorldObservation{Stage: e.Stage, Clock: e.Clock, Context: e.Context, Flags: *flags, Work: e.Work, IO: e.IO})
		}
	}
	s.before = p.before
	return p, nil
}

// World uses a read/modify/write bus loop. Its branch tests the old read,
// including the extra iteration after acquisition/release actually succeeds.
func (p *battleSample) worldBus(device BattleWorldDevice, asserted bool) error {
	for {
		var granted bool
		err := p.step(engine.MainWorkOperation{Phase: "world-bus-control", CPUCycles: 24}, func() error {
			var err error
			granted, err = device.BusGranted(p.s.clock.Cycle)
			if err != nil {
				return err
			}
			old := uint32(1)
			if granted {
				old = 0
			}
			if err := p.io(BattleSampleIO{Kind: "bus-control", Value: old, Width: 1}); err != nil {
				return err
			}
			value := uint32(0)
			if asserted {
				value = 1
			}
			if err := p.io(BattleSampleIO{Kind: "bus-control", Write: true, Value: value, Width: 1}); err != nil {
				return err
			}
			after, err := device.RequestBus(p.s.clock.Cycle, asserted)
			if err != nil {
				return err
			}
			if after < p.s.clock.Cycle {
				return errors.New("World bus returned a backwards main cursor")
			}
			p.s.clock.Cycle = after
			return nil
		})
		if err != nil {
			return err
		}
		repeat := granted != asserted
		if err := p.branch("world-bus-branch", repeat, false); err != nil {
			return err
		}
		if !repeat {
			return nil
		}
	}
}
func (p *battleSample) worldGate(device BattleWorldDevice, blocked bool) error {
	return p.step(engine.MainWorkOperation{Phase: "world-transfer-gate", CPUCycles: 20}, func() error {
		value := uint32(0)
		if blocked {
			value = 1
		}
		if err := p.io(BattleSampleIO{Kind: "transfer-gate", Write: true, Value: value, Width: 1}); err != nil {
			return err
		}
		if err := device.SetBankBlocked(p.s.clock.Cycle, blocked); err != nil {
			return err
		}
		return p.s.add(7)
	})
}
func (p *battleSample) prepareWorld(input BattleWorldInput, flags *BattleWorldFlags, pause *uint8, device BattleWorldDevice) (BattleWorldEntry, error) {
	if err := p.step(engine.MainWorkOperation{Phase: "mask-world-IRQ", CPUCycles: 20}, func() error { p.context.Mask = 7; return nil }); err != nil {
		return BattleWorldEntry{}, err
	}
	if err := p.spend("world-phase-test", 16, 12, 10); err != nil {
		return BattleWorldEntry{}, err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "world-transfer-start", CPUCycles: 20}, func() error { flags.TransferActive = true; return nil }); err != nil {
		return BattleWorldEntry{}, err
	}
	if err := p.worldBus(device, true); err != nil {
		return BattleWorldEntry{}, err
	}
	if err := p.worldGate(device, true); err != nil {
		return BattleWorldEntry{}, err
	}
	var paused bool
	if err := p.step(engine.MainWorkOperation{Phase: "world-pause-test", CPUCycles: 16}, func() error { paused = *pause == 255; return nil }); err != nil {
		return BattleWorldEntry{}, err
	}
	if err := p.branch("world-pause-branch", paused, true); err != nil {
		return BattleWorldEntry{}, err
	}
	if !paused {
		if err := p.worldBus(device, false); err != nil {
			return BattleWorldEntry{}, err
		}
	}
	if err := p.step(engine.MainWorkOperation{Phase: "save-world-temporaries", CPUCycles: 112, InternalRefresh: true, TailCycles: 8}, nil); err != nil {
		return BattleWorldEntry{}, err
	}
	if err := p.spend("world-display-test", 12); err != nil {
		return BattleWorldEntry{}, err
	}
	if err := p.branch("world-display-branch", !input.DisplayReady, false); err != nil {
		return BattleWorldEntry{}, err
	}
	if input.DisplayReady {
		if err := p.vertical(206); err != nil {
			return BattleWorldEntry{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "world-marker-address", CPUCycles: 28}, func() error {
			return p.io(BattleSampleIO{Kind: "palette-address", Write: true, Value: 0x74020003, Width: 4})
		}); err != nil {
			return BattleWorldEntry{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "world-marker-clear", CPUCycles: 20}, func() error { return p.io(BattleSampleIO{Kind: "palette-data", Write: true, Width: 2}) }); err != nil {
			return BattleWorldEntry{}, err
		}
		if err := p.spend("world-settle-count", 8); err != nil {
			return BattleWorldEntry{}, err
		}
		for i := 0; i <= 80; i++ {
			cost := uint16(10)
			if i == 80 {
				cost = 14
			}
			if err := p.spend("world-settle-loop", cost); err != nil {
				return BattleWorldEntry{}, err
			}
		}
		if err := p.step(engine.MainWorkOperation{Phase: "world-disable-cached-display", CPUCycles: 20}, func() error {
			if p.cachedDisplay != nil {
				*p.cachedDisplay = false
			}
			return nil
		}); err != nil {
			return BattleWorldEntry{}, err
		}
		if err := p.spend("world-display-control", 12, 4, 12, 12, 12); err != nil {
			return BattleWorldEntry{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "world-disable-display", CPUCycles: 8}, func() error {
			value := uint8(4)
			if p.video.Snapshot().VerticalEnabled {
				value |= 32
			}
			effect, err := p.video.WriteRegister(videoclock.VerticalControl, value)
			if err != nil {
				return err
			}
			if err := p.effect(effect); err != nil {
				return err
			}
			return p.io(BattleSampleIO{Kind: "display", Write: true, Value: 0x8100 | uint32(value), Width: 2})
		}); err != nil {
			return BattleWorldEntry{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "world-backdrop-address", CPUCycles: 12}, func() error {
			return p.io(BattleSampleIO{Kind: "palette-address", Write: true, Value: 0xc0000000, Width: 4})
		}); err != nil {
			return BattleWorldEntry{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "world-backdrop-clear", CPUCycles: 8}, func() error { return p.io(BattleSampleIO{Kind: "palette-data", Write: true, Width: 2}) }); err != nil {
			return BattleWorldEntry{}, err
		}
	}
	if err := p.spend("world-wrapper-call", 20); err != nil {
		return BattleWorldEntry{}, err
	}
	if err := p.sync(); err != nil {
		return BattleWorldEntry{}, err
	}
	return BattleWorldEntry{Clock: p.s.clock, Context: p.snapshot(), Operations: p.s.operations, AwaitingWorld: true}, nil
}

// PrepareBattleWorldIRQ stops at the actual World-wrapper entry. The caller
// supplies detached owners and must not resume interrupted work yet. This
// current window has no VDP DMA and cannot cross a reference frame boundary.
func PrepareBattleWorldIRQ(clock Clock, context BattleSampleContext, input BattleWorldInput, flags *BattleWorldFlags, pause *uint8, device BattleWorldDevice, observer BattleWorldObserver, limits Limits) (BattleWorldEntry, error) {
	s, err := tickService(clock, limits)
	if err != nil {
		return BattleWorldEntry{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	p, err := newBattleWorld(s, context, flags, pause, device, observer)
	if err != nil {
		return BattleWorldEntry{}, err
	}
	return p.prepareWorld(input, flags, pause, device)
}
