package scoreclock

// NoteCommitWork is a transient projection of the unique score voice. FM and
// PSG share the cleanup leaf; DAC ends either at rest's return or immediately
// before actual PCM publication. Source positions and encoded notes are absent.
type NoteCommitWork struct {
	Family            string
	Tied              bool
	ModulationEnabled bool
	DACSample         bool
}

type NoteCommitResult struct {
	Clock               Clock
	NeedsDACPublication bool
}

type ModulationWork struct {
	Enabled            bool
	Delay, Rate, Steps uint8
}

func validNoteCommit(work NoteCommitWork) bool {
	if !noteFamily(work.Family) {
		return false
	}
	if work.Family == "dac" {
		return !work.Tied && !work.ModulationEnabled
	}
	return !work.DACSample
}

// modulationSeed accounts for the three definition bytes, the halved step
// seed, and the cleared offset. It changes no owner and shares the caller's
// clock/refresh/operation budget.
func (s *service) modulationSeed() error {
	for field := 0; field < 3; field++ {
		if err := s.spend(16); err != nil {
			return err
		}
	}
	if err := s.spend(8); err != nil {
		return err
	}
	if err := s.spend(8); err != nil {
		return err
	}
	if err := s.spend(12); err != nil {
		return err
	}
	return s.spend(16)
}

func (s *service) noteCommit(work NoteCommitWork) error {
	if err := s.spend(16); err != nil {
		return err
	} // retain logical next-event position
	if work.Family == "dac" {
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.spend(12); err != nil {
			return err
		} // current semantic DAC selection
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.branch(!work.DACSample, false); err != nil {
			return err
		}
		if !work.DACSample {
			return s.spend(16)
		}
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.spend(12); err != nil {
			return err
		} // fixed descriptor alignment
		return s.spend(12) // immutable descriptor resolution, before bus request
	}
	if err := s.spend(20); err != nil {
		return err
	} // reload note wait
	if err := s.spend(12); err != nil {
		return err
	}
	if err := s.branch(work.Tied, false); err != nil {
		return err
	}
	if !work.Tied {
		if err := s.spend(20); err != nil {
			return err
		} // reload gate
		for field := 0; field < 3; field++ {
			if err := s.spend(16); err != nil {
				return err
			}
		} // volume/pitch envelope cursors and pitch scale
		if err := s.spend(16); err != nil {
			return err
		}
		if err := s.branch(!work.ModulationEnabled, false); err != nil {
			return err
		}
		if work.ModulationEnabled {
			if err := s.spend(16); err != nil {
				return err
			} // current immutable modulation definition
			if err := s.modulationSeed(); err != nil {
				return err
			}
		}
	}
	return s.spend(16)
}

// PriceNoteCommit includes the cleanup return, but no caller or later note
// work. An accepted DAC sample stops before saved-SR/bus/arbitration/metadata
// publication; NeedsDACPublication never claims those operations were priced.
func PriceNoteCommit(clock Clock, work NoteCommitWork, limits Limits) (NoteCommitResult, error) {
	if !validNoteCommit(work) {
		return NoteCommitResult{}, ErrNoteWorkUnavailable
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return NoteCommitResult{}, err
	}
	if err = s.noteCommit(work); err != nil {
		return NoteCommitResult{}, err
	}
	return NoteCommitResult{Clock: s.clock, NeedsDACPublication: work.Family == "dac" && work.DACSample}, nil
}

func (s *service) modulationWork(work ModulationWork) error {
	if err := s.spend(16); err != nil {
		return err
	}
	if err := s.branch(!work.Enabled, false); err != nil {
		return err
	}
	if !work.Enabled {
		return s.spend(16)
	}
	if err := s.spend(12); err != nil {
		return err
	}
	if err := s.branch(work.Delay == 0, false); err != nil {
		return err
	}
	if work.Delay != 0 {
		if err := s.spend(16); err != nil {
			return err
		}
		return s.spend(16)
	}
	if err := s.spend(16); err != nil {
		return err
	} // cyclic rate decrement: zero wraps, only one expires
	if err := s.branch(work.Rate == 1, false); err != nil {
		return err
	}
	if work.Rate != 1 {
		return s.spend(16)
	}
	if err := s.spend(16); err != nil {
		return err
	} // current immutable definition
	if err := s.spend(20); err != nil {
		return err
	} // rate reload
	if err := s.spend(12); err != nil {
		return err
	}
	if err := s.branch(work.Steps != 0, false); err != nil {
		return err
	}
	if work.Steps == 0 {
		if err := s.spend(20); err != nil {
			return err
		} // step reload
		if err := s.spend(16); err != nil {
			return err
		} // delta reversal; no offset change in this branch
		return s.spend(16)
	}
	if err := s.spend(16); err != nil {
		return err
	} // step decrement
	if err := s.spend(12); err != nil {
		return err
	} // signed delta
	if err := s.spend(4); err != nil {
		return err
	}
	if err := s.spend(12); err != nil {
		return err
	} // add wrapped offset
	if err := s.spend(12); err != nil {
		return err
	} // retain offset
	if err := s.spend(12); err != nil {
		return err
	} // temporary base-pitch combination
	return s.spend(16)
}

// PriceModulationWork includes the complete held-note modulation leaf and
// return. It neither advances the canonical score counters nor emits pitch;
// the existing score driver remains the sole owner of both semantics.
func PriceModulationWork(clock Clock, work ModulationWork, limits Limits) (Clock, error) {
	s, err := tickService(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.modulationWork(work); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}
