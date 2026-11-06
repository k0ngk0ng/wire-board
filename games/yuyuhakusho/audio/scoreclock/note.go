package scoreclock

import (
	"errors"
	"math"
	"math/bits"
)

var ErrNoteWorkUnavailable = errors.New("note Work clock unavailable")

// PitchWork is a transient semantic projection, not an encoded source token.
// Index is the effective seven-bit chromatic note after transpose, and is
// required only for chromatic FM/PSG. Rest and DAC carry zero Index.
type PitchWork struct {
	Family string
	Form   string
	Index  uint8
}
type DurationProbeWork struct {
	Family  string
	Outcome string
}

func noteFamily(family string) bool { return family == "fm" || family == "psg" || family == "dac" }
func validPitchWork(work PitchWork) bool {
	if !noteFamily(work.Family) {
		return false
	}
	if work.Form == "rest" {
		return work.Index == 0
	}
	if work.Family == "dac" {
		return work.Form == "dac" && work.Index == 0
	}
	return work.Form == "chromatic" && work.Index < 128
}

func (s *service) noteRefreshSkip() error {
	if s.clock.Cycle >= s.clock.Refresh {
		if s.clock.Refresh > math.MaxUint64-896 {
			return errors.New("note internal refresh overflow")
		}
		s.clock.Refresh += 896
	}
	return nil
}

// noteOctaveDivision accounts only for the fixed twelve-note octave resolver.
// In this seven-bit domain, the reference divide's 15 internal rounds simplify
// to 136 minus twice popcount(floor(index/24)) CPU cycles, plus its fixed four.
// Its refresh skip occurs after that internal work, before the fixed four;
// lumping it into spend(total) would charge the next refresh incorrectly.
func (s *service) noteOctaveDivision(index uint8) error {
	return s.cost(4, func() error {
		cycles := uint64(136 - 2*bits.OnesCount8(index/24))
		if err := s.add(cycles * 7); err != nil {
			return err
		}
		return s.noteRefreshSkip()
	})
}

func (s *service) noteOctaveAlignment() error {
	// The fixed eleven-bit octave alignment has 22 internal CPU cycles and
	// six base cycles. Its internal refresh skip precedes those six cycles.
	return s.cost(6, func() error {
		if err := s.add(22 * 7); err != nil {
			return err
		}
		return s.noteRefreshSkip()
	})
}

func (s *service) pitchWork(work PitchWork) error {
	if !validPitchWork(work) {
		return ErrNoteWorkUnavailable
	}
	if work.Family == "dac" {
		return s.spend(12)
	} // retain the selected DAC note marker
	if err := s.spend(8); err != nil {
		return err
	} // rest/chromatic normalization
	rest := work.Form == "rest"
	if err := s.branch(rest, false); err != nil {
		return err
	}
	if rest {
		if err := s.spend(16); err != nil {
			return err
		} // note-off state
		return s.spend(16) // retain family-specific rest pitch
	}
	if err := s.spend(12); err != nil {
		return err
	} // live transpose
	if work.Family == "psg" {
		if err := s.spend(8); err != nil {
			return err
		} // seven-bit note
		if err := s.spend(8); err != nil {
			return err
		} // word table selection
		if err := s.spend(8); err != nil {
			return err
		} // immutable pitch table
		return s.spend(22) // resolve and retain divider
	}
	if err := s.spend(16); err != nil {
		return err
	} // seven-bit FM note
	if err := s.noteOctaveDivision(work.Index); err != nil {
		return err
	}
	if err := s.spend(4); err != nil {
		return err
	} // chromatic remainder
	if err := s.spend(8); err != nil {
		return err
	} // table word
	if err := s.spend(8); err != nil {
		return err
	} // immutable f-number table
	if err := s.spend(14); err != nil {
		return err
	} // resolve f-number
	if err := s.spend(4); err != nil {
		return err
	} // octave quotient
	if err := s.spend(8); err != nil {
		return err
	} // octave domain
	if err := s.spend(4); err != nil {
		return err
	} // fixed octave alignment
	if err := s.noteOctaveAlignment(); err != nil {
		return err
	} // align octave bits
	if err := s.spend(4); err != nil {
		return err
	} // combine octave/f-number
	return s.spend(12) // retain resolved FM pitch
}

// PricePitchWork stops immediately after the original pitch/marker write.
// It excludes caller setup, callee return, cleanup and final note commit.
// PSG timing support does not add unrecovered pitch table values to score.
func PricePitchWork(clock Clock, work PitchWork, limits Limits) (Clock, error) {
	if !validPitchWork(work) {
		return Clock{}, ErrNoteWorkUnavailable
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.pitchWork(work); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}

func (s *service) durationProbe(work DurationProbeWork) error {
	if !noteFamily(work.Family) || work.Outcome != "take" && work.Outcome != "reuse" {
		return ErrNoteWorkUnavailable
	}
	if err := s.spend(8); err != nil {
		return err
	} // actual lookahead read
	if work.Family == "psg" {
		if err := s.spend(4); err != nil {
			return err
		} // independent sign test
	}
	take := work.Outcome == "take"
	if err := s.branch(take, false); err != nil {
		return err
	}
	if take {
		return nil
	}
	if err := s.spend(8); err != nil {
		return err
	} // leave lookahead for the next parse
	if work.Family == "dac" {
		if err := s.spend(20); err != nil {
			return err
		} // reuse last duration
	}
	return s.spend(10) // continuation at the final-commit boundary
}

// PriceDurationProbe stops before the duration caller on take, or before the
// final commit on reuse. It includes the DAC-specific old-duration copy, but
// neither the duration callee/caller nor any live commit body.
func PriceDurationProbe(clock Clock, work DurationProbeWork, limits Limits) (Clock, error) {
	if !noteFamily(work.Family) || work.Outcome != "take" && work.Outcome != "reuse" {
		return Clock{}, ErrNoteWorkUnavailable
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.durationProbe(work); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}
