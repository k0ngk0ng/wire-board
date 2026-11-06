package scoreclock

import (
	"errors"
	"math"
)

type GateRoute uint8

const (
	TickNormal GateRoute = iota + 1
	TickPauseHold
	TickPauseStart
	TickResume
)

// PauseField is the root owner's live byte, not a score-side snapshot.
type PauseField interface {
	ReadPause(at uint64) (uint8, error)
	WritePause(at uint64, value uint8) error
}
type PriorityField interface {
	WritePriority(at uint64, value uint8) error
}
type GateResult struct {
	Clock           Clock
	Route           GateRoute
	DomainClearedAt uint64
}

func tickService(clock Clock, limits Limits) (*service, error) {
	if limits.MaxOperations > 65536 || clock.Cycle > math.MaxUint64-896 {
		return nil, errors.New("invalid fixed tick boundary input")
	}
	limit := limits.MaxOperations
	if limit == 0 {
		limit = 4096
	}
	return &service{clock: clock, limit: limit}, nil
}

// EnterTick stops at the next semantic phase: normal housekeeping, pause
// acquisition, or resume's release/restore prefix. PauseHold already includes
// its early return. It does not advance duck/fade/tempo/dispatch or score.
// All fields are accessed on caller-owned detached transactions.
func EnterTick(clock Clock, fields PauseField, limits Limits) (GateResult, error) {
	if fields == nil {
		return GateResult{}, errors.New("missing root pause owner")
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return GateResult{}, err
	}
	var domainAt uint64
	if err = s.cost(16, func() error { domainAt = s.clock.Cycle; return nil }); err != nil {
		return GateResult{}, err
	}
	read := func(cpu uint64) (uint8, error) {
		var value uint8
		err := s.cost(cpu, func() error {
			var failure error
			value, failure = fields.ReadPause(s.clock.Cycle)
			return failure
		})
		return value, err
	}
	pause, err := read(12)
	if err != nil {
		return GateResult{}, err
	}
	if err = s.branch(pause != 0, true); err != nil {
		return GateResult{}, err
	}
	if pause == 0 {
		return GateResult{Clock: s.clock, Route: TickNormal, DomainClearedAt: domainAt}, nil
	}
	pause, err = read(16)
	if err != nil {
		return GateResult{}, err
	}
	if err = s.branch(pause != 255, true); err != nil {
		return GateResult{}, err
	}
	if pause == 255 {
		if err = s.spend(16); err != nil {
			return GateResult{}, err
		}
		return GateResult{Clock: s.clock, Route: TickPauseHold, DomainClearedAt: domainAt}, nil
	}
	pause, err = read(12)
	if err != nil {
		return GateResult{}, err
	}
	resume := pause&128 != 0
	if err = s.branch(resume, false); err != nil {
		return GateResult{}, err
	}
	value, route := uint8(255), TickPauseStart
	if resume {
		value, route = 0, TickResume
	}
	if err = s.cost(16, func() error { return fields.WritePause(s.clock.Cycle, value) }); err != nil {
		return GateResult{}, err
	}
	return GateResult{Clock: s.clock, Route: route, DomainClearedAt: domainAt}, nil
}

// FinishTick accounts only for the fixed final return. It is not appended to
// the gate's completed hold return or to leaf services that already return.
func FinishTick(clock Clock, limits Limits) (Clock, error) {
	s, err := tickService(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.spend(16); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}

// ClearPrimaryPriority applies one real semantic store at its own position.
// It does not consume the queue, aggregate a tick bool, or own the priority.
func ClearPrimaryPriority(clock Clock, fields PriorityField, limits Limits) (Clock, error) {
	if fields == nil {
		return Clock{}, errors.New("missing root priority owner")
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.cost(16, func() error { return fields.WritePriority(s.clock.Cycle, 0) }); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}
