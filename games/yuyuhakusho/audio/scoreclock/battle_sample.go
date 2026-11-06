package scoreclock

import (
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmcommands"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/videoclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"math"
)

// BattleSampleContext borrows the caller's video and IRQ owners for one
// unflushed reference frame. Phase is semantic: sample=0, late reload=1,
// world=2. Clock is supplied separately and has a single owner in the service.
type BattleSampleContext struct {
	FrameOrigin      uint64
	Video            videoclock.State
	Pin, Mask, Phase uint8
}
type BattleSampleInput struct {
	DisplayReady bool
	RestoreColor uint16
	Controllers  engine.ControllerScanInput
}
type BattleSampleIO struct {
	At             uint64
	Kind           string
	Port           uint8
	Control, Write bool
	Value          uint32
	Width          uint8
}
type BattleSampleObservation struct {
	Stage   string
	Clock   Clock
	Context BattleSampleContext
	Work    engine.MainWorkOperation
	IO      *BattleSampleIO
}
type BattleSampleObserver func(BattleSampleObservation) error
type SampleAuxiliary interface {
	PCMFrontier() uint64
	RunUntil(uint64, []pcmcommands.DMAInterval) (pcmcommands.RunResult, error)
}
type BattleSamplePrefix struct {
	Clock      Clock
	Context    BattleSampleContext
	Samples    engine.ControllerScanResult
	Operations uint32
}
type BattleSampleResult struct {
	Prefix         BattleSamplePrefix
	Sound          SoundIRQResult
	Clock          Clock
	Context        BattleSampleContext
	Operations     uint32
	AwaitingReturn bool
}

var ErrBattleSampleContext = errors.New("unsupported battle sample IRQ context")
var ErrBattleSampleFrame = errors.New("battle sampling leaves the current unflushed video frame")

type battleSample struct {
	s             *service
	context       BattleSampleContext
	video         *videoclock.Clock
	aux           SampleAuxiliary
	observer      BattleSampleObserver
	work          *engine.MainWorkOperation
	cachedDisplay *bool        // Borrowed only when the enclosing main owner supplies it.
	mainSync      func() error // Same enclosing main coordinator, never checkpointed.
}

func newBattleSample(s *service, context BattleSampleContext, input BattleSampleInput, aux SampleAuxiliary, observer BattleSampleObserver) (*battleSample, error) {
	if aux == nil || context.Mask != 4 || context.Phase != 0 || context.Pin != 0 && context.Pin != 4 && context.Pin != 6 || context.Video.Phase != videoclock.Scanlines || context.FrameOrigin > math.MaxUint64-896040 {
		return nil, ErrBattleSampleContext
	}
	if err := engine.ValidateControllerScanInput(input.Controllers); err != nil {
		return nil, err
	}
	video, err := videoclock.New(videoclock.Config{Profile: videoclock.NTSC224H40}, context.Video)
	if err != nil {
		return nil, err
	}
	start := context.FrameOrigin + uint64(video.LineCycle())
	if s.clock.Cycle < start || s.clock.Cycle >= start+3420 || aux.PCMFrontier() < start {
		return nil, ErrBattleSampleContext
	}
	p := &battleSample{s: s, context: context, video: video, aux: aux, observer: observer}
	s.before = p.before
	return p, nil
}
func (p *battleSample) snapshot() BattleSampleContext {
	context := p.context
	context.Video = p.video.Snapshot()
	return context
}
func (p *battleSample) effect(effect videoclock.Effect) error {
	if effect.Deferred {
		return ErrBattleSampleContext
	}
	if effect.SetIRQ {
		p.context.Pin = effect.Level
	}
	if effect.UpdateIRQ {
		p.context.Pin |= effect.Level
	}
	return nil
}

// The original board runs the auxiliary player after the main execution batch,
// against the batch TARGET, then charges its real main contention and advances
// the VDP line. A current instruction can overrun; no line changes inside it.
func (p *battleSample) sync() error {
	if p.mainSync != nil {
		return p.mainSync()
	}
	for {
		target := p.context.FrameOrigin + uint64(p.video.LineCycle()) + 3420
		if p.s.clock.Cycle < target {
			return nil
		}
		if target >= p.context.FrameOrigin+896040 {
			return ErrBattleSampleFrame
		}
		result, err := p.aux.RunUntil(target, nil)
		if err != nil {
			return err
		}
		if err := p.s.add(result.M68KDelay); err != nil {
			return err
		}
		effect, err := p.video.NextLine()
		if err != nil {
			return err
		}
		if err := p.effect(effect); err != nil {
			return err
		}
	}
}
func (p *battleSample) before() error {
	if err := p.sync(); err != nil {
		return err
	}
	if p.work != nil && p.observer != nil {
		return p.observer(BattleSampleObservation{Stage: "cpu", Clock: p.s.clock, Context: p.snapshot(), Work: *p.work})
	}
	return nil
}
func (p *battleSample) io(io BattleSampleIO) error {
	io.At = p.s.clock.Cycle
	if p.observer != nil {
		return p.observer(BattleSampleObservation{Stage: "io", Clock: p.s.clock, Context: p.snapshot(), IO: &io})
	}
	return nil
}
func (p *battleSample) step(op engine.MainWorkOperation, effect func() error) error {
	p.work = &op
	defer func() { p.work = nil }()
	body := uint64(op.CPUCycles)
	if op.InternalRefresh {
		body = uint64(op.TailCycles)
	}
	return p.s.cost(body, func() error {
		if effect != nil {
			if err := effect(); err != nil {
				return err
			}
		}
		if op.InternalRefresh {
			if err := p.s.add(uint64(op.CPUCycles) * 7); err != nil {
				return err
			}
			if p.s.clock.Cycle >= p.s.clock.Refresh {
				if p.s.clock.Refresh > math.MaxUint64-896 {
					return ErrBattleSampleFrame
				}
				p.s.clock.Refresh += 896
			}
		}
		return nil
	})
}
func (p *battleSample) spend(phase string, costs ...uint16) error {
	for _, cost := range costs {
		if err := p.step(engine.MainWorkOperation{Phase: phase, CPUCycles: cost}, nil); err != nil {
			return err
		}
	}
	return nil
}
func (p *battleSample) branch(phase string, taken, word bool) error {
	cost := uint16(10)
	if !taken {
		cost = 8
		if word {
			cost = 12
		}
	}
	return p.spend(phase, cost)
}
func (p *battleSample) vertical(target uint8) error {
	if err := p.spend("vertical-wait-setup", 8, 12); err != nil {
		return err
	}
	for {
		var value uint8
		read := func() error {
			var err error
			value, err = p.video.ReadVertical(uint32(p.s.clock.Cycle - p.context.FrameOrigin))
			if err != nil {
				return err
			}
			return p.io(BattleSampleIO{Kind: "vertical", Value: uint32(value), Width: 1})
		}
		if err := p.step(engine.MainWorkOperation{Phase: "vertical-compare", CPUCycles: 8}, read); err != nil {
			return err
		}
		if err := p.branch("vertical-first-branch", target > value, false); err != nil {
			return err
		}
		if target > value {
			continue
		}
		if err := p.step(engine.MainWorkOperation{Phase: "vertical-confirm", CPUCycles: 8}, read); err != nil {
			return err
		}
		if err := p.branch("vertical-second-branch", target > value, false); err != nil {
			return err
		}
		if target <= value {
			return nil
		}
	}
}
func (p *battleSample) delay(count int) error {
	if err := p.spend("sample-settle-count", 8); err != nil {
		return err
	}
	for i := 0; i <= count; i++ {
		cost := uint16(10)
		if i == count {
			cost = 14
		}
		if err := p.spend("sample-settle-loop", cost); err != nil {
			return err
		}
	}
	return nil
}
func (p *battleSample) prepare(input BattleSampleInput) (BattleSamplePrefix, error) {
	if err := p.step(engine.MainWorkOperation{Phase: "mask-sampling-IRQ", CPUCycles: 20}, func() error { p.context.Mask = 7; return nil }); err != nil {
		return BattleSamplePrefix{}, err
	}
	if err := p.spend("sample-phase-test", 16, 12, 12); err != nil {
		return BattleSamplePrefix{}, err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "sample-next-reload", CPUCycles: 20}, func() error {
		effect, err := p.video.WriteRegister(videoclock.CounterReload, 184)
		if err != nil {
			return err
		}
		if err := p.effect(effect); err != nil {
			return err
		}
		return p.io(BattleSampleIO{Kind: "reload", Write: true, Value: 0x8ab8, Width: 2})
	}); err != nil {
		return BattleSamplePrefix{}, err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "sample-phase-advance", CPUCycles: 16}, func() error { p.context.Phase = 1; return nil }); err != nil {
		return BattleSamplePrefix{}, err
	}
	if err := p.step(engine.MainWorkOperation{Phase: "save-sampling-temporaries", CPUCycles: 112, InternalRefresh: true, TailCycles: 8}, nil); err != nil {
		return BattleSamplePrefix{}, err
	}
	if err := p.spend("sample-display-test", 12); err != nil {
		return BattleSamplePrefix{}, err
	}
	if err := p.branch("sample-display-branch", !input.DisplayReady, false); err != nil {
		return BattleSamplePrefix{}, err
	}
	if input.DisplayReady {
		if err := p.vertical(13); err != nil {
			return BattleSamplePrefix{}, err
		}
		if err := p.delay(42); err != nil {
			return BattleSamplePrefix{}, err
		}
		if err := p.spend("sample-display-control", 12, 20, 12); err != nil {
			return BattleSamplePrefix{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "sample-enable-display", CPUCycles: 8}, func() error {
			value := uint8(0x44)
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
			return BattleSamplePrefix{}, err
		}
		if err := p.vertical(15); err != nil {
			return BattleSamplePrefix{}, err
		}
		if err := p.delay(40); err != nil {
			return BattleSamplePrefix{}, err
		}
		if err := p.spend("sample-palette-setup", 12, 12, 16, 12); err != nil {
			return BattleSamplePrefix{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "sample-palette-address", CPUCycles: 12}, func() error {
			return p.io(BattleSampleIO{Kind: "palette-address", Write: true, Value: 0xc0000000, Width: 4})
		}); err != nil {
			return BattleSamplePrefix{}, err
		}
		if err := p.step(engine.MainWorkOperation{Phase: "sample-palette-restore", CPUCycles: 8}, func() error {
			return p.io(BattleSampleIO{Kind: "palette-data", Write: true, Value: uint32(input.RestoreColor), Width: 2})
		}); err != nil {
			return BattleSamplePrefix{}, err
		}
	}
	if err := p.spend("sample-controller-call", 20); err != nil {
		return BattleSamplePrefix{}, err
	}
	samples, err := engine.ScanBattleControllers(input.Controllers, func(op engine.ControllerScanOperation) error {
		return p.step(op.Work, func() error {
			if op.IO == nil {
				return nil
			}
			io := op.IO
			return p.io(BattleSampleIO{Kind: "controller", Port: io.Port, Control: io.Control, Write: io.Write, Value: uint32(io.Value), Width: 1})
		})
	})
	if err != nil {
		return BattleSamplePrefix{}, err
	}
	if err := p.spend("sample-sound-call", 20); err != nil {
		return BattleSamplePrefix{}, err
	}
	if err := p.sync(); err != nil {
		return BattleSamplePrefix{}, err
	}
	return BattleSamplePrefix{Clock: p.s.clock, Context: p.snapshot(), Samples: samples, Operations: p.s.operations}, nil
}

// PrepareBattleSampleIRQ ends at the actual sound-scheduler call. Supplied aux
// owns real background feedback; all owners must be detached transactions.
// The supported window has no VDP DMA and does not cross a frame flush.
func PrepareBattleSampleIRQ(clock Clock, context BattleSampleContext, input BattleSampleInput, aux SampleAuxiliary, observer BattleSampleObserver, limits Limits) (BattleSamplePrefix, error) {
	s, err := tickService(clock, limits)
	if err != nil {
		return BattleSamplePrefix{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	p, err := newBattleSample(s, context, input, aux, observer)
	if err != nil {
		return BattleSamplePrefix{}, err
	}
	return p.prepare(input)
}

// AdvanceBattleSampleIRQ executes the prefix and the real whole sound driver
// on one clock, refresh owner and CPU budget. The saved-temporary/RTE epilogue
// is still a separate pending stage; callers must not resume the World yet.
func AdvanceBattleSampleIRQ(clock Clock, context BattleSampleContext, physical BattleSampleInput, aux SampleAuxiliary, observer BattleSampleObserver, state *engine.SoundSchedulerState, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus Bus, chip Chip, limits Limits) (BattleSampleResult, error) {
	return runBattleSampleIRQ(clock, context, physical, aux, observer, state, root, owners, input, library, bus, chip, limits, false, 0)
}

// RunBattleSampleIRQ also completes the IRQ return on the same service budget.
// An unpriced new World IRQ fails instead of resuming the interrupted World.
func RunBattleSampleIRQ(clock Clock, context BattleSampleContext, physical BattleSampleInput, callerMask uint8, aux SampleAuxiliary, observer BattleSampleObserver, state *engine.SoundSchedulerState, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus Bus, chip Chip, limits Limits) (BattleSampleResult, error) {
	if callerMask > 7 {
		return BattleSampleResult{}, ErrBattleSampleContext
	}
	return runBattleSampleIRQ(clock, context, physical, aux, observer, state, root, owners, input, library, bus, chip, limits, true, callerMask)
}
func runBattleSampleIRQ(clock Clock, context BattleSampleContext, physical BattleSampleInput, aux SampleAuxiliary, observer BattleSampleObserver, state *engine.SoundSchedulerState, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus Bus, chip Chip, limits Limits, complete bool, callerMask uint8) (BattleSampleResult, error) {
	if state == nil {
		return BattleSampleResult{}, ErrRunningProfile
	}
	if _, err := validateRequestOwners(root, owners, input, library, bus, chip, true); err != nil {
		return BattleSampleResult{}, err
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return BattleSampleResult{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	p, err := newBattleSample(s, context, physical, aux, observer)
	if err != nil {
		return BattleSampleResult{}, err
	}
	prefix, err := p.prepare(physical)
	if err != nil {
		return BattleSampleResult{}, err
	}
	sound, err := advanceSoundIRQOnService(s, state, root, owners, input, library, false)
	if err != nil {
		return BattleSampleResult{}, err
	}
	if err := p.sync(); err != nil {
		return BattleSampleResult{}, err
	}
	if complete {
		returned, err := p.finish(callerMask)
		if err != nil {
			return BattleSampleResult{}, err
		}
		if returned.PendingWorld {
			return BattleSampleResult{}, ErrBattleWorldWork
		}
	}
	return BattleSampleResult{Prefix: prefix, Sound: sound, Clock: s.clock, Context: p.snapshot(), Operations: s.operations, AwaitingReturn: !complete}, nil
}
