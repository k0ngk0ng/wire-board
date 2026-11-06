package engine

import "errors"

type SoundRequestKind string

const (
	RequestTestRead        SoundRequestKind = "request-test-read"
	RequestTestDecision    SoundRequestKind = "request-test-decision"
	RequestCompareRead     SoundRequestKind = "request-compare-read"
	RequestCompareDecision SoundRequestKind = "request-compare-decision"
	RequestStore           SoundRequestKind = "request-store"
	RequestReject          SoundRequestKind = "request-reject"
	RequestReturn          SoundRequestKind = "request-return"
)

type SoundRequestOperation struct {
	Sequence    uint32
	Kind        SoundRequestKind
	Slot        int8
	Code, Value uint8
	Accepted    bool
}
type SoundRequestHook func(SoundRequestOperation, func() error) error

type SoundScheduleKind string

const (
	ScheduleDelayRead        SoundScheduleKind = "delay-read"
	ScheduleDelayDecision    SoundScheduleKind = "delay-decision"
	ScheduleDelayWrite       SoundScheduleKind = "delay-write"
	ScheduleDelayedRead      SoundScheduleKind = "delayed-read"
	ScheduleDelayedDecision  SoundScheduleKind = "delayed-decision"
	ScheduleSubmit           SoundScheduleKind = "submit"
	ScheduleSubmitDecision   SoundScheduleKind = "submit-decision"
	ScheduleDelayedWrite     SoundScheduleKind = "delayed-write"
	ScheduleBusyRead         SoundScheduleKind = "busy-read"
	ScheduleBusyDecision     SoundScheduleKind = "busy-decision"
	ScheduleBacklogAdd       SoundScheduleKind = "backlog-add"
	ScheduleBusyWrite        SoundScheduleKind = "busy-write"
	ScheduleBacklogRead      SoundScheduleKind = "backlog-read"
	ScheduleBacklogDecision  SoundScheduleKind = "backlog-decision"
	ScheduleBacklogDecrement SoundScheduleKind = "backlog-decrement"
	ScheduleDrive            SoundScheduleKind = "drive"
	ScheduleRepeat           SoundScheduleKind = "repeat"
	ScheduleReturn           SoundScheduleKind = "return"
)

type SoundScheduleOperation struct {
	Sequence uint32
	Phase    string
	Kind     SoundScheduleKind
	Value    uint16
	Accepted bool
}
type SoundScheduleHook func(SoundScheduleOperation, func() error) error
type SoundSchedulerOwners struct {
	State *SoundSchedulerState
	Queue *[4]uint8
}
type SoundSchedulerHooks struct {
	Operation SoundScheduleHook
	Request   SoundRequestHook
}

var ErrSoundScheduleSequence = errors.New("invalid sound scheduling effect sequence")

func soundLease(hook func(func() error) error, effect func() error) error {
	if effect == nil {
		effect = func() error { return nil }
	}
	if hook == nil {
		return effect()
	}
	active, called, violated := true, false, false
	apply := func() error {
		if !active || called {
			violated = true
			return ErrSoundScheduleSequence
		}
		called = true
		return effect()
	}
	err := hook(apply)
	active = false
	if err != nil {
		return err
	}
	if !called || violated {
		return ErrSoundScheduleSequence
	}
	return nil
}

// EnqueueSoundObserved is the source A..D request rule on its actual owner.
// A negative slot is read again for CMP; an interrupt between these operations
// must not be hidden behind a sampled queue or an atomic predicate shortcut.
func EnqueueSoundObserved(queue *[4]uint8, code uint8, hook SoundRequestHook) (bool, error) {
	if queue == nil {
		return false, ErrSoundScheduleSequence
	}
	sequence := uint32(0)
	emit := func(op SoundRequestOperation, effect func() error) error {
		op.Sequence = sequence
		sequence++
		if hook == nil {
			return soundLease(nil, effect)
		}
		return soundLease(func(apply func() error) error { return hook(op, apply) }, effect)
	}
	for slot := int8(0); slot < 4; slot++ {
		var value uint8
		if err := emit(SoundRequestOperation{Kind: RequestTestRead, Slot: slot, Code: code}, func() error { value = queue[slot]; return nil }); err != nil {
			return false, err
		}
		if err := emit(SoundRequestOperation{Kind: RequestTestDecision, Slot: slot, Code: code, Value: value}, nil); err != nil {
			return false, err
		}
		accept := value < 128
		if !accept {
			if err := emit(SoundRequestOperation{Kind: RequestCompareRead, Slot: slot, Code: code}, func() error { value = queue[slot]; return nil }); err != nil {
				return false, err
			}
			accept = value == code
			if err := emit(SoundRequestOperation{Kind: RequestCompareDecision, Slot: slot, Code: code, Value: value, Accepted: accept}, nil); err != nil {
				return false, err
			}
		}
		if accept {
			if err := emit(SoundRequestOperation{Kind: RequestStore, Slot: slot, Code: code}, func() error { queue[slot] = code; return nil }); err != nil {
				return false, err
			}
			if err := emit(SoundRequestOperation{Kind: RequestReturn, Slot: slot, Code: code, Accepted: code != 0}, nil); err != nil {
				return false, err
			}
			return code != 0, nil
		}
	}
	if err := emit(SoundRequestOperation{Kind: RequestReject, Slot: -1, Code: code}, nil); err != nil {
		return false, err
	}
	return false, emit(SoundRequestOperation{Kind: RequestReturn, Slot: -1, Code: code}, nil)
}

type soundScheduleEmitter struct {
	sequence uint32
	phase    string
	hook     SoundScheduleHook
}

func (e *soundScheduleEmitter) emit(kind SoundScheduleKind, value uint16, accepted bool, effect func() error) error {
	op := SoundScheduleOperation{Sequence: e.sequence, Phase: e.phase, Kind: kind, Value: value, Accepted: accepted}
	e.sequence++
	if e.hook == nil {
		return soundLease(nil, effect)
	}
	return soundLease(func(apply func() error) error { return e.hook(op, apply) }, effect)
}

func BeginSoundInterruptObserved(owners SoundSchedulerOwners, hooks SoundSchedulerHooks) (bool, error) {
	if owners.State == nil || owners.Queue == nil {
		return false, ErrSoundScheduleSequence
	}
	state := owners.State
	e := soundScheduleEmitter{phase: "interrupt-begin", hook: hooks.Operation}
	var delay uint16
	if err := e.emit(ScheduleDelayRead, 0, false, func() error { delay = state.Delay; return nil }); err != nil {
		return false, err
	}
	if err := e.emit(ScheduleDelayDecision, delay, false, nil); err != nil {
		return false, err
	}
	if delay != 0 {
		if err := e.emit(ScheduleDelayWrite, delay-1, false, func() error { state.Delay = delay - 1; return nil }); err != nil {
			return false, err
		}
	} else {
		var code uint8
		if err := e.emit(ScheduleDelayedRead, 0, false, func() error { code = state.DelayedCode; return nil }); err != nil {
			return false, err
		}
		if err := e.emit(ScheduleDelayedDecision, uint16(code), false, nil); err != nil {
			return false, err
		}
		if code != 0 {
			if err := e.emit(ScheduleSubmit, uint16(code), false, nil); err != nil {
				return false, err
			}
			accepted, err := EnqueueSoundObserved(owners.Queue, code, hooks.Request)
			if err != nil {
				return false, err
			}
			if err = e.emit(ScheduleSubmitDecision, uint16(code), accepted, nil); err != nil {
				return false, err
			}
			if accepted {
				if err = e.emit(ScheduleDelayedWrite, 0, false, func() error { state.DelayedCode = 0; return nil }); err != nil {
					return false, err
				}
			}
		}
	}
	var busy uint8
	if err := e.emit(ScheduleBusyRead, 0, false, func() error { busy = state.Busy; return nil }); err != nil {
		return false, err
	}
	if err := e.emit(ScheduleBusyDecision, uint16(busy), false, nil); err != nil {
		return false, err
	}
	if busy != 0 {
		if err := e.emit(ScheduleBacklogAdd, 0, false, func() error { state.Backlog++; return nil }); err != nil {
			return false, err
		}
		return false, e.emit(ScheduleReturn, 0, false, nil)
	}
	if err := e.emit(ScheduleBusyWrite, 1, false, func() error { state.Busy = 1; return nil }); err != nil {
		return false, err
	}
	return true, e.emit(ScheduleDrive, 0, false, nil)
}
func FinishSoundInterruptObserved(state *SoundSchedulerState, hook SoundScheduleHook) error {
	if state == nil {
		return ErrSoundScheduleSequence
	}
	e := soundScheduleEmitter{phase: "interrupt-finish", hook: hook}
	if err := e.emit(ScheduleBusyWrite, 0, false, func() error { state.Busy = 0; return nil }); err != nil {
		return err
	}
	return e.emit(ScheduleReturn, 0, false, nil)
}
func BeginSoundCatchupObserved(state *SoundSchedulerState, hook SoundScheduleHook) (bool, error) {
	if state == nil {
		return false, ErrSoundScheduleSequence
	}
	e := soundScheduleEmitter{phase: "catchup-begin", hook: hook}
	var backlog uint8
	if err := e.emit(ScheduleBacklogRead, 0, false, func() error { backlog = state.Backlog; return nil }); err != nil {
		return false, err
	}
	if err := e.emit(ScheduleBacklogDecision, uint16(backlog), false, nil); err != nil {
		return false, err
	}
	if backlog == 0 {
		return false, e.emit(ScheduleReturn, 0, false, nil)
	}
	if err := e.emit(ScheduleBusyWrite, 1, false, func() error { state.Busy = 1; return nil }); err != nil {
		return false, err
	}
	return true, e.emit(ScheduleDrive, 0, false, nil)
}
func FinishSoundCatchupObserved(state *SoundSchedulerState, hook SoundScheduleHook) error {
	if state == nil {
		return ErrSoundScheduleSequence
	}
	e := soundScheduleEmitter{phase: "catchup-finish", hook: hook}
	if err := e.emit(ScheduleBusyWrite, 0, false, func() error { state.Busy = 0; return nil }); err != nil {
		return err
	}
	if err := e.emit(ScheduleBacklogDecrement, 0, false, func() error { state.Backlog--; return nil }); err != nil {
		return err
	}
	return e.emit(ScheduleRepeat, 0, false, nil)
}
