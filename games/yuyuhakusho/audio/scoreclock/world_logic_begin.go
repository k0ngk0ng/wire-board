package scoreclock

import (
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type BattleWorldLogicBegin struct {
	Clock           Clock
	Context         BattleMainContext
	Operations      uint32
	Outcome         engine.WorldFrameOutcome
	AwaitingObjects bool
	Frames          []mixed.Frame
}

func (w *battleWorldClock) beginLogic(input engine.ControllerNormalizationInput) (BattleWorldLogicBegin, error) {
	world := w.world
	if world.Clock.ControllerCount > 8 || input.ChannelCount != uint8(world.Clock.ControllerCount) {
		return BattleWorldLogicBegin{}, ErrBattleWorldProfile
	}
	baseGetter, ports := input.LivePorts, input.Ports
	input.LivePorts = func() (engine.ControllerPorts, error) {
		// Normalization locks each read before its operation-complete hook.
		// Drain the same pending batch/IRQ before that read; its following Work
		// hook sees no new edge and prices the operation exactly once.
		if err := w.settle(); err != nil {
			return engine.ControllerPorts{}, err
		}
		if baseGetter != nil {
			return baseGetter()
		}
		return ports, nil
	}
	hooks := engine.WorldFrameWorkHooks{
		Work: func(op engine.MainWorkOperation, _ engine.WorldFrameOwners) error { return w.p.step(op, nil) },
		Normalize: func(owners engine.WorldFrameOwners, work engine.MainWorkHook) error {
			state, err := engine.NormalizeControllersWork(engine.ControllerNormalizationState{Controllers: *owners.Controllers, Devices: w.normalizedDevices}, input, engine.ControllerNormalizationHook(work))
			if err != nil {
				return err
			}
			*owners.Controllers = state.Controllers
			w.normalizedDevices = state.Devices
			return nil
		},
	}
	out, err := engine.AdvanceWorldFrameOnOwners(engine.WorldFrameOwners{Clock: &world.Clock, Controllers: &world.Controllers, RandomSeed: &world.RandomSeed, SlowGate: &world.SlowGate, ResetEnabled: &world.ResetEnabled}, hooks)
	if err != nil {
		return BattleWorldLogicBegin{}, err
	}
	if err := w.sync(); err != nil {
		return BattleWorldLogicBegin{}, err
	}
	return BattleWorldLogicBegin{Clock: w.p.s.clock, Context: w.snapshot(), Operations: w.p.s.operations, Outcome: out, AwaitingObjects: !out.ResetRequested, Frames: w.frames}, nil
}

// AdvanceBattleWorldLogicBegin executes actual3a2e through the3a5a object
// preparation boundary on the same video/PCM/IRQ coordinator. It borrows only
// detached canonical owners and leaves object callbacks and the tail pending.
func AdvanceBattleWorldLogicBegin(clock Clock, context BattleMainContext, graphics *BattleGraphics, flags *BattleWorldFlags, world *engine.ChuWorldState, input engine.ControllerNormalizationInput, device BattleWorldClockDevice, observer BattleWorldObserver, limits Limits) (BattleWorldLogicBegin, error) {
	if context.IRQ.Phase != 0 && context.IRQ.Phase != 2 {
		return BattleWorldLogicBegin{}, ErrBattleWorldProfile
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return BattleWorldLogicBegin{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	w, err := newWorldClockAt(s, context, graphics, flags, world, device, observer, 0, context.IRQ.Phase, true)
	if err != nil {
		return BattleWorldLogicBegin{}, err
	}
	return w.beginLogic(input)
}
