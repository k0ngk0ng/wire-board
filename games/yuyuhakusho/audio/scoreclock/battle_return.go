package scoreclock

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/videoclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type BattleSampleReturn struct {
	Clock                  Clock
	Context                BattleSampleContext
	Operations             uint32
	Returned, PendingWorld bool
}

var ErrBattleWorldWork = errors.New("pending battle World IRQ work is not priced")
var ErrBattleVerticalReturn = errors.New("pending vertical IRQ return is not priced")

func newBattleReturn(s *service, context BattleSampleContext, callerMask uint8, aux SampleAuxiliary, observer BattleSampleObserver) (*battleSample, error) {
	if callerMask > 7 || aux == nil || context.Mask != 7 || context.Phase != 1 || context.Pin != 0 && context.Pin != 4 || context.Video.Phase != videoclock.Scanlines {
		return nil, ErrBattleSampleContext
	}
	video, err := videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, context.Video)
	if err != nil {
		return nil, err
	}
	start := context.FrameOrigin + uint64(video.LineCycle())
	if start < context.FrameOrigin || context.FrameOrigin > ^uint64(0)-896040 || s.clock.Cycle < start || s.clock.Cycle >= start+3420 || aux.PCMFrontier() < start {
		return nil, ErrBattleSampleContext
	}
	p := &battleSample{s: s, context: context, video: video, aux: aux, observer: observer}
	s.before = p.before
	return p, nil
}
func (p *battleSample) acceptReturnIRQ() (bool, error) {
	accepted, err := engine.AcceptWorldIRQ(p.s.clock.Cycle, p.context.Pin, p.context.Mask)
	if err != nil {
		return false, err
	}
	if !accepted.Accepted {
		return false, nil
	}
	if accepted.Mask == 6 {
		return false, ErrBattleVerticalReturn
	}
	p.context.Mask = accepted.Mask
	effect, err := p.video.Acknowledge()
	if err != nil {
		return false, err
	}
	if err := p.effect(effect); err != nil {
		return false, err
	}
	p.s.clock.Cycle = accepted.Cycle
	return true, nil
}

// RTE restores the saved mask and checks an IRQ inside the operation. Its
// fixed twenty cycles follow that exception entry; this is not ordinary
// instruction-boundary acceptance or an extra refresh opportunity.
func (p *battleSample) returnIRQ(callerMask uint8) (bool, error) {
	accepted := false
	err := p.step(engine.MainWorkOperation{Phase: "return-from-IRQ", CPUCycles: 20}, func() error {
		p.context.Mask = callerMask
		var err error
		accepted, err = p.acceptReturnIRQ()
		return err
	})
	return accepted, err
}
func (p *battleSample) lateReload() error {
	if err := p.step(engine.MainWorkOperation{Phase: "late-reload-mask", CPUCycles: 20}, func() error { p.context.Mask = 7; return nil }); err != nil {
		return err
	}
	if err := p.spend("late-reload-phase", 16, 10); err != nil {
		return err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "late-reload-counter", CPUCycles: 20}, func() error {
		effect, err := p.video.WriteRegister(videoclock.CounterReload, 223)
		if err != nil {
			return err
		}
		if err := p.effect(effect); err != nil {
			return err
		}
		return p.io(BattleSampleIO{Kind: "reload", Write: true, Value: 0x8adf, Width: 2})
	}); err != nil {
		return err
	}
	return p.step(engine.MainWorkOperation{Phase: "late-reload-advance", CPUCycles: 16}, func() error { p.context.Phase = 2; return nil })
}
func (p *battleSample) finish(callerMask uint8) (BattleSampleReturn, error) {
	if callerMask > 7 || p.context.Mask != 7 || p.context.Phase != 1 {
		return BattleSampleReturn{}, ErrBattleSampleContext
	}
	start := p.s.operations
	if err := p.step(engine.MainWorkOperation{Phase: "restore-sampling-temporaries", CPUCycles: 112, InternalRefresh: true, TailCycles: 12}, nil); err != nil {
		return BattleSampleReturn{}, err
	}
	for {
		accepted, err := p.returnIRQ(callerMask)
		if err != nil {
			return BattleSampleReturn{}, err
		}
		if !accepted {
			// Returning can itself overrun the current batch. Deliver its auxiliary
			// work/line edge before the caller's next operation and perform the normal
			// new-batch check, which is distinct from the check within RTE above.
			if err := p.sync(); err != nil {
				return BattleSampleReturn{}, err
			}
			accepted, err = p.acceptReturnIRQ()
			if err != nil {
				return BattleSampleReturn{}, err
			}
		}
		if !accepted {
			return BattleSampleReturn{Clock: p.s.clock, Context: p.snapshot(), Operations: p.s.operations - start, Returned: true}, nil
		}
		// Exception delay may pass a batch target before the handler's first op.
		if err := p.sync(); err != nil {
			return BattleSampleReturn{}, err
		}
		if p.context.Phase == 2 {
			return BattleSampleReturn{Clock: p.s.clock, Context: p.snapshot(), Operations: p.s.operations - start, PendingWorld: true}, nil
		}
		if p.context.Phase != 1 {
			return BattleSampleReturn{}, ErrBattleSampleContext
		}
		if err := p.lateReload(); err != nil {
			return BattleSampleReturn{}, err
		}
	}
}

// FinishBattleSampleIRQ completes the pending temporary restore and RTE,
// including a following late-reload IRQ. A new World IRQ is reported pending
// before its first operation, never treated as a resumed World. Owners supplied
// here are detached; the session-level whole call handles atomic commit.
func FinishBattleSampleIRQ(clock Clock, context BattleSampleContext, callerMask uint8, aux SampleAuxiliary, observer BattleSampleObserver, limits Limits) (BattleSampleReturn, error) {
	s, err := tickService(clock, limits)
	if err != nil {
		return BattleSampleReturn{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	p, err := newBattleReturn(s, context, callerMask, aux, observer)
	if err != nil {
		return BattleSampleReturn{}, err
	}
	return p.finish(callerMask)
}
