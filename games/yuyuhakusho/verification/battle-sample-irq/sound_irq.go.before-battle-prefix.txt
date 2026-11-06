package scoreclock

import (
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type SchedulerField struct {
	Kind  string
	At    uint64
	Value uint16
}
type SoundIRQResult struct {
	Clock           Clock
	Entries         []Clock
	Drivers         []DriverResult
	SchedulerFields []SchedulerField
	RequestFields   []QueueField
}
type soundIRQClock struct {
	s                         *service
	state                     *engine.SoundSchedulerState
	queue                     *[4]uint8
	result                    SoundIRQResult
	phase                     string
	sequence, requestSequence uint32
}

// The caller supplies an actually accepted outer interrupt/catchup entry clock.
// These functions do not infer world/VDP acceptance. Passed owners are detached;
// a failure may follow completed child drivers and external IO.
func AdvanceSoundInterrupt(clock Clock, state *engine.SoundSchedulerState, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus Bus, chip Chip, limits Limits) (SoundIRQResult, error) {
	return advanceSoundIRQ(clock, state, root, owners, input, library, bus, chip, limits, false)
}
func AdvanceSoundCatchup(clock Clock, state *engine.SoundSchedulerState, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus Bus, chip Chip, limits Limits) (SoundIRQResult, error) {
	return advanceSoundIRQ(clock, state, root, owners, input, library, bus, chip, limits, true)
}
func advanceSoundIRQ(clock Clock, state *engine.SoundSchedulerState, root *rootcontrol.State, owners rootcontrol.RuleOwners, input rootcontrol.RuleInput, library *resourcebridge.Library, bus Bus, chip Chip, limits Limits, catchup bool) (SoundIRQResult, error) {
	if state == nil {
		return SoundIRQResult{}, ErrRunningProfile
	}
	if _, err := validateRequestOwners(root, owners, input, library, bus, chip, true); err != nil {
		return SoundIRQResult{}, err
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return SoundIRQResult{}, err
	}
	if limits.MaxOperations == 0 {
		s.limit = 65536
	}
	r := &soundIRQClock{s: s, state: state, queue: owners.Queue, result: SoundIRQResult{Entries: []Clock{}, Drivers: []DriverResult{}, SchedulerFields: []SchedulerField{}, RequestFields: []QueueField{}}}
	for {
		var drive bool
		if catchup {
			drive, err = engine.BeginSoundCatchupObserved(state, r.schedule)
		} else {
			drive, err = engine.BeginSoundInterruptObserved(engine.SoundSchedulerOwners{State: state, Queue: owners.Queue}, engine.SoundSchedulerHooks{Operation: r.schedule, Request: r.request})
		}
		if err != nil {
			return SoundIRQResult{}, err
		}
		if !drive {
			break
		}
		out, failure := advanceRequestOnService(s, root, owners, input, library, true)
		if failure != nil {
			return SoundIRQResult{}, failure
		}
		r.result.Drivers = append(r.result.Drivers, out)
		if catchup {
			err = engine.FinishSoundCatchupObserved(state, r.schedule)
		} else {
			err = engine.FinishSoundInterruptObserved(state, r.schedule)
		}
		if err != nil {
			return SoundIRQResult{}, err
		}
		if !catchup {
			break
		}
	}
	r.result.Clock = s.clock
	return r.result, nil
}

func (r *soundIRQClock) schedulerField(cpu uint64, op engine.SoundScheduleOperation, apply func() error) error {
	return r.s.cost(cpu, func() error {
		if err := apply(); err != nil {
			return err
		}
		var value uint16
		switch op.Kind {
		case engine.ScheduleDelayRead, engine.ScheduleDelayWrite:
			value = r.state.Delay
		case engine.ScheduleDelayedRead, engine.ScheduleDelayedWrite:
			value = uint16(r.state.DelayedCode)
		case engine.ScheduleBusyRead, engine.ScheduleBusyWrite:
			value = uint16(r.state.Busy)
		case engine.ScheduleBacklogRead, engine.ScheduleBacklogAdd, engine.ScheduleBacklogDecrement:
			value = uint16(r.state.Backlog)
		}
		r.result.SchedulerFields = append(r.result.SchedulerFields, SchedulerField{Kind: string(op.Kind), At: r.s.clock.Cycle, Value: value})
		return nil
	})
}
func (r *soundIRQClock) schedule(op engine.SoundScheduleOperation, apply func() error) error {
	if op.Phase != r.phase {
		if op.Sequence != 0 {
			return ErrRunningSequence
		}
		r.phase = op.Phase
		r.sequence = 0
	}
	if op.Sequence != r.sequence || apply == nil {
		return ErrRunningSequence
	}
	r.sequence++
	s := r.s
	switch op.Kind {
	case engine.ScheduleDelayRead, engine.ScheduleDelayedRead, engine.ScheduleBusyRead, engine.ScheduleBacklogRead:
		return r.schedulerField(12, op, apply)
	case engine.ScheduleDelayDecision, engine.ScheduleDelayedDecision, engine.ScheduleBusyDecision, engine.ScheduleBacklogDecision:
		if err := s.branch(op.Value == 0, false); err != nil {
			return err
		}
	case engine.ScheduleDelayWrite:
		if err := s.spend(4); err != nil {
			return err
		}
		if err := r.schedulerField(12, op, apply); err != nil {
			return err
		}
		return s.spend(10)
	case engine.ScheduleSubmit:
		r.requestSequence = 0
		if err := s.spend(20); err != nil {
			return err
		}
	case engine.ScheduleSubmitDecision:
		if err := s.branch(!op.Accepted, false); err != nil {
			return err
		}
	case engine.ScheduleDelayedWrite, engine.ScheduleBusyWrite, engine.ScheduleBacklogAdd, engine.ScheduleBacklogDecrement:
		return r.schedulerField(16, op, apply)
	case engine.ScheduleDrive:
		if err := s.spend(20); err != nil {
			return err
		}
		r.result.Entries = append(r.result.Entries, s.clock)
		// The real exported driver entry is a fixed PC-relative jump to its
		// body. The other adjacent jump is a distinct initialization entry.
		if err := s.spend(10); err != nil {
			return err
		}
	case engine.ScheduleReturn:
		if err := s.spend(16); err != nil {
			return err
		}
	case engine.ScheduleRepeat:
		if err := s.spend(10); err != nil {
			return err
		}
	default:
		return ErrRunningSequence
	}
	return apply()
}
func (r *soundIRQClock) request(op engine.SoundRequestOperation, apply func() error) error {
	if op.Sequence != r.requestSequence || apply == nil {
		return ErrRunningSequence
	}
	r.requestSequence++
	s := r.s
	switch op.Kind {
	case engine.RequestTestRead, engine.RequestCompareRead, engine.RequestStore:
		if op.Slot < 0 || op.Slot >= 4 {
			return ErrRunningSequence
		}
		return s.cost(12, func() error {
			if err := apply(); err != nil {
				return err
			}
			r.result.RequestFields = append(r.result.RequestFields, QueueField{Kind: string(op.Kind), At: s.clock.Cycle, Slot: op.Slot, Value: r.queue[op.Slot]})
			return nil
		})
	case engine.RequestTestDecision:
		if err := s.branch(op.Value < 128, false); err != nil {
			return err
		}
	case engine.RequestCompareDecision:
		if err := s.branch(!op.Accepted, false); err != nil {
			return err
		}
	case engine.RequestReject:
		if err := s.spend(4); err != nil {
			return err
		}
	case engine.RequestReturn:
		if err := s.spend(16); err != nil {
			return err
		}
	default:
		return ErrRunningSequence
	}
	return apply()
}
