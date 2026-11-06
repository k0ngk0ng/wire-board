package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrParserWorkUnavailable = errors.New("parser Work clock unavailable")

func supportedParserWork(work score.ParserWork) bool {
	shape := score.ParserWork{Kind: work.Kind}
	switch work.Kind {
	case score.WorkTokenRead:
		if work.Form != "control" && work.Form != "pitch" && work.Form != "duration" {
			return false
		}
		shape.Form = work.Form
	case score.WorkControlDispatch:
		switch work.Action {
		case "pan", "detune", "pan-automation", "level-add", "tie", "gate", "tempo", "enqueue-replace-first", "fm-patch", "fm-patch-select", "modulation", "track-end", "psg-noise", "volume-envelope", "transpose-add", "modulation-enable", "jump", "loop", "call", "return":
		default:
			return false
		}
		shape.Action = work.Action
	case score.WorkLoop:
		shape.First, shape.Taken = work.First, work.Taken
	case score.WorkJump, score.WorkCall, score.WorkReturn:
	case score.WorkDurationScale:
		if work.Additions > 1 {
			return false
		}
		shape.Additions = work.Additions
	default:
		return false
	}
	return work == shape
}

// parserJump prices the original finite offset-read/resolve leaf and its
// return. It reads no offset, source pointer or executable control data.
func (s *service) parserJump() error {
	if err := s.spend(8); err != nil {
		return err
	} // high offset byte
	if err := s.spend(22); err != nil {
		return err
	} // fixed byte-to-word alignment
	if err := s.spend(8); err != nil {
		return err
	} // low offset byte
	if err := s.spend(8); err != nil {
		return err
	} // source-relative resolution
	if err := s.spend(8); err != nil {
		return err
	} // parser read-position convention
	return s.spend(16)
}

func (s *service) parserWork(work score.ParserWork) error {
	if !supportedParserWork(work) {
		return ErrParserWorkUnavailable
	}
	switch work.Kind {
	case score.WorkTokenRead:
		if err := s.spend(4); err != nil {
			return err
		} // token accumulator setup
		if err := s.spend(8); err != nil {
			return err
		} // token read
		if err := s.spend(8); err != nil {
			return err
		} // control classification
		return s.branch(work.Form != "control", false)
	case score.WorkControlDispatch:
		if err := s.spend(18); err != nil {
			return err
		} // original PC-relative caller
		if err := s.spend(8); err != nil {
			return err
		} // semantic action normalization
		if err := s.spend(10); err != nil {
			return err
		} // fixed action-table selection
		if err := s.spend(14); err != nil {
			return err
		} // canonical handler selection
		return s.spend(10) // enter selected handler; its body is separately owned
	case score.WorkJump:
		return s.parserJump()
	case score.WorkCall:
		if err := s.spend(4); err != nil {
			return err
		} // temporary call-site setup
		if err := s.spend(12); err != nil {
			return err
		} // current finite call depth
		if err := s.spend(4); err != nil {
			return err
		} // reserve one return
		if err := s.spend(18); err != nil {
			return err
		} // preserve source return location
		if err := s.spend(12); err != nil {
			return err
		} // commit depth
		if err := s.spend(10); err != nil {
			return err
		} // shared finite offset leaf
		return s.parserJump()
	case score.WorkReturn:
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.spend(12); err != nil {
			return err
		} // read finite return position
		if err := s.spend(18); err != nil {
			return err
		} // resolve retained source return
		if err := s.spend(8); err != nil {
			return err
		} // consume original offset field
		if err := s.spend(4); err != nil {
			return err
		} // release finite return
		if err := s.spend(12); err != nil {
			return err
		} // commit depth
		return s.spend(16)
	case score.WorkLoop:
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		} // loop selector
		if err := s.spend(8); err != nil {
			return err
		} // repetition definition
		if err := s.spend(14); err != nil {
			return err
		} // first/repeat classification
		if err := s.branch(!work.First, false); err != nil {
			return err
		}
		if work.First {
			if err := s.spend(14); err != nil {
				return err
			} // initialize declared loop
		}
		if err := s.spend(18); err != nil {
			return err
		} // consume one repetition
		if err := s.branch(work.Taken, false); err != nil {
			return err
		}
		if work.Taken {
			return s.parserJump()
		}
		if err := s.spend(8); err != nil {
			return err
		} // completed offset field
		return s.spend(16)
	case score.WorkDurationScale:
		if err := s.spend(4); err != nil {
			return err
		} // duration seed
		if err := s.spend(12); err != nil {
			return err
		} // declared multiplier
		for addition := uint8(0); addition <= work.Additions; addition++ {
			if err := s.spend(4); err != nil {
				return err
			}
			last := addition == work.Additions
			if err := s.branch(last, false); err != nil {
				return err
			}
			if !last {
				if err := s.spend(4); err != nil {
					return err
				} // one bounded addition
				if err := s.spend(10); err != nil {
					return err
				}
			}
		}
		if err := s.spend(12); err != nil {
			return err
		} // retain duration definition
		if err := s.spend(12); err != nil {
			return err
		} // retain current wait
		return s.spend(16)
	}
	return ErrParserWorkUnavailable
}

// PriceParserWork accounts for one explicitly supported structural family.
// TokenRead ends at its actual control-call/cleanup boundary. ControlDispatch
// includes its caller and dispatcher but stops before the selected live body.
// Flow and duration-scale families include their callee return, not caller
// setup/backedge. Unknown live body/resource work remains explicitly unpriced.
// Work never changes score state or drives a source interpreter.
func PriceParserWork(clock Clock, work score.ParserWork, limits Limits) (Clock, error) {
	if !supportedParserWork(work) {
		return Clock{}, ErrParserWorkUnavailable
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.parserWork(work); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}

// FinishParserControl prices the fixed parser continuation after a completed
// control handler has already returned. It adds no second handler return and
// is never appended to an incomplete control or live parameter body.
func FinishParserControl(clock Clock, limits Limits) (Clock, error) {
	s, err := tickService(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.spend(10); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}
