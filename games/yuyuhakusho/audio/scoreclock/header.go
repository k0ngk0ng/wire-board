package scoreclock

import (
	"errors"
	"math"
)

// ErrHeaderUnsupported rejects non-idle control/fade work whose semantics have
// not been recovered. Such work must not silently receive an idle cost.
var ErrHeaderUnsupported = errors.New("unsupported non-idle header profile")

// ControlWork is a temporary view supplied by the unique control owner.
// Only the double-zero idle profile is supported; nonzero mode semantics are
// deliberately not inferred or stored by this cost service.
type ControlWork struct{ Mode, AuxMode uint8 }

// TempoWork is derived by the score owner for this invocation only. The mask
// preserves the original order of the ten music slots, and is present only on
// reload. This service owns no tempo, remaining or NoteWait counter.
type TempoWork struct {
	Enabled, Reload bool
	ActiveMusicMask uint16
}

func headerCall(clock Clock, limits Limits) (*service, error) {
	if limits.MaxOperations > 65536 || clock.Cycle > math.MaxUint64-896 {
		return nil, errors.New("invalid fixed header clock or operation limit")
	}
	limit := limits.MaxOperations
	if limit == 0 {
		limit = 4096
	}
	s := &service{clock: clock, limit: limit}
	// The original caller is PC-relative JSR, not absolute-long JSR.
	if err := s.spend(18); err != nil {
		return nil, err
	}
	return s, nil
}

// CallControlIdle includes the original caller, double-zero control-idle
// callee and one return. It does not restore non-idle duck/control behavior.
func CallControlIdle(clock Clock, work ControlWork, limits Limits) (Clock, error) {
	if work.Mode != 0 || work.AuxMode != 0 {
		return Clock{}, ErrHeaderUnsupported
	}
	s, err := headerCall(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	return controlIdleBody(s, work)
}

func controlIdleBody(s *service, work ControlWork) (Clock, error) {
	var err error
	if err = s.spend(16); err != nil {
		return Clock{}, err
	} // compare auxiliary mode
	if err = s.branch(false, true); err != nil {
		return Clock{}, err
	}
	if err = s.spend(12); err != nil {
		return Clock{}, err
	} // read main mode
	if err = s.branch(true, true); err != nil {
		return Clock{}, err
	}
	if err = s.spend(16); err != nil {
		return Clock{}, err
	} // compare main mode
	if err = s.branch(false, true); err != nil {
		return Clock{}, err
	}
	if err = s.spend(12); err != nil {
		return Clock{}, err
	} // read auxiliary mode
	if err = s.branch(true, true); err != nil {
		return Clock{}, err
	}
	if err = s.spend(16); err != nil {
		return Clock{}, err
	} // return
	return s.clock, nil
}

func validTempoWork(work TempoWork) bool {
	if work.ActiveMusicMask > 0x3ff {
		return false
	}
	if !work.Enabled {
		return !work.Reload && work.ActiveMusicMask == 0
	}
	return work.Reload || work.ActiveMusicMask == 0
}

// CallTempo includes the original caller, the fixed disabled/step/reload path
// and one return. The score owner supplies the actual branch and applies its
// counters once; subsequent observed counter/scan/leave stages must not charge
// this work again. No original instructions or caller costs are accepted.
func CallTempo(clock Clock, work TempoWork, limits Limits) (Clock, error) {
	if !validTempoWork(work) {
		return Clock{}, errors.New("inconsistent temporary tempo work")
	}
	s, err := headerCall(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	return tempoBody(s, work)
}

func tempoBody(s *service, work TempoWork) (Clock, error) {
	var err error
	if err = s.spend(12); err != nil {
		return Clock{}, err
	} // test enabled period
	if err = s.branch(!work.Enabled, false); err != nil {
		return Clock{}, err
	}
	if !work.Enabled {
		if err = s.spend(16); err != nil {
			return Clock{}, err
		}
		return s.clock, nil
	}
	if err = s.spend(16); err != nil {
		return Clock{}, err
	} // remaining decrement
	if err = s.branch(!work.Reload, false); err != nil {
		return Clock{}, err
	}
	if !work.Reload {
		if err = s.spend(16); err != nil {
			return Clock{}, err
		}
		return s.clock, nil
	}
	if err = s.spend(20); err != nil {
		return Clock{}, err
	} // period-to-remaining reload
	if err = s.spend(8); err != nil {
		return Clock{}, err
	} // select music slots
	if err = s.spend(4); err != nil {
		return Clock{}, err
	} // fixed slot stride
	if err = s.spend(4); err != nil {
		return Clock{}, err
	} // fixed ten-slot scan
	for slot := 0; slot < 10; slot++ {
		active := work.ActiveMusicMask&(1<<slot) != 0
		if err = s.spend(8); err != nil {
			return Clock{}, err
		} // test this slot
		if err = s.branch(!active, false); err != nil {
			return Clock{}, err
		}
		if active {
			if err = s.spend(16); err != nil {
				return Clock{}, err
			} // owner's NoteWait increment
		}
		if err = s.spend(8); err != nil {
			return Clock{}, err
		} // next slot
		if slot < 9 {
			if err = s.spend(10); err != nil {
				return Clock{}, err
			} // scan backedge
		} else {
			if err = s.spend(14); err != nil {
				return Clock{}, err
			} // final scan branch
		}
	}
	if err = s.spend(16); err != nil {
		return Clock{}, err
	} // return
	return s.clock, nil
}

// CallFadeIdle includes the original caller, zero-step fade-idle callee and
// one return. Fade state stays with the caller; non-idle profiles are refused.
func CallFadeIdle(clock Clock, fadeSteps uint8, limits Limits) (Clock, error) {
	if fadeSteps != 0 {
		return Clock{}, ErrHeaderUnsupported
	}
	s, err := headerCall(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	return fadeIdleBody(s)
}

func fadeIdleBody(s *service) (Clock, error) {
	var err error
	if err = s.spend(4); err != nil {
		return Clock{}, err
	} // temporary clear
	if err = s.spend(12); err != nil {
		return Clock{}, err
	} // read fade steps
	if err = s.branch(true, false); err != nil {
		return Clock{}, err
	}
	if err = s.spend(16); err != nil {
		return Clock{}, err
	} // return
	return s.clock, nil
}
