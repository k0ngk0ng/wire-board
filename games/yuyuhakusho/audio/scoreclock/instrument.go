package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

var ErrInstrumentSequence = errors.New("invalid instrument transfer sequence")

type InstrumentRequest struct {
	Mode, Domain    string
	Channel         uint8
	Claimed         bool
	Work            score.InstrumentWork
	Patch           param.FMPatch
	BaseLevel       uint8
	LoadedAlgorithm uint8
}

// InstrumentTransfer is one temporary caller transaction. It owns only a work
// cursor/clock/budget and derived expected effects, never score/chip state or
// a persistent source stack/program. Discard it and all detached external
// owners on any failure; phases cannot continue after a rejected operation.
type InstrumentTransfer struct {
	s             *service
	request       InstrumentRequest
	writes        []Request
	cursor        int
	operator      int
	levelPending  bool
	negativeLevel bool
	done          bool
	failure       error
}

var instrumentSlots = [4]uint8{0, 2, 1, 3}
var instrumentCarriers = [8]uint8{8, 8, 8, 8, 10, 14, 14, 15}

func validInstrumentRequest(work InstrumentRequest) error {
	if work.Mode != "load" && work.Mode != "level" {
		return ErrInstrumentSequence
	}
	if work.Domain != "music" && work.Domain != "primary" && work.Domain != "secondary" {
		return ErrInstrumentSequence
	}
	switch work.Channel {
	case 0, 1, 2, 4, 5, 6:
	default:
		return ErrInstrumentSequence
	}
	if work.LoadedAlgorithm > 7 || work.Work.ID == "" || work.Work.PatchID != work.Patch.ID {
		return ErrInstrumentSequence
	}
	return work.Patch.Validate()
}

// instrumentSeek uses the actual temporary domain and immutable seek workload.
// No original bank, instrument index or pointer is retained or dereferenced.
func (s *service) instrumentSeek(domain string, skipped uint8, level bool) error {
	if err := s.spend(16); err != nil {
		return err
	}
	if err := s.spend(12); err != nil {
		return err
	}
	if err := s.branch(domain == "music", false); err != nil {
		return err
	}
	if domain != "music" {
		if err := s.spend(16); err != nil {
			return err
		}
		if level {
			if err := s.spend(12); err != nil {
				return err
			}
		}
		if err := s.branch(domain == "secondary", false); err != nil {
			return err
		}
		if domain == "primary" {
			if err := s.spend(16); err != nil {
				return err
			}
		}
	}
	if err := s.spend(4); err != nil {
		return err
	}
	if err := s.branch(skipped == 0, false); err != nil {
		return err
	}
	if skipped != 0 {
		if err := s.spend(8); err != nil {
			return err
		}
		for record := 0; record < int(skipped); record++ {
			if err := s.spend(8); err != nil {
				return err
			}
			if err := s.pauseLoopEnd(record == int(skipped)-1); err != nil {
				return err
			}
		}
	}
	return nil
}

func BeginInstrumentTransfer(clock Clock, work InstrumentRequest, bus Bus, chip Chip, limits Limits) (*InstrumentTransfer, error) {
	if err := validInstrumentRequest(work); err != nil {
		return nil, err
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	t := &InstrumentTransfer{s: s, request: work}
	if err = s.spend(12); err != nil {
		return nil, err
	} // live claimed guard
	if err = s.branch(work.Claimed, work.Mode == "load"); err != nil {
		return nil, err
	}
	if work.Claimed {
		return t, nil
	}
	if work.Mode == "level" {
		if err = s.spend(4); err != nil {
			return nil, err
		}
		if err = s.spend(12); err != nil {
			return nil, err
		}
	}
	if err = s.instrumentSeek(work.Domain, work.Work.RecordsSkipped, work.Mode == "level"); err != nil {
		return nil, err
	}
	if work.Mode == "load" {
		if err = s.spend(8); err != nil {
			return nil, err
		} // selected algorithm read
		if err = s.spend(12); err != nil {
			return nil, err
		} // logical loaded-algorithm point
		writes, err := param.PatchWrites(work.Patch, work.BaseLevel)
		if err != nil {
			return nil, err
		}
		for _, write := range writes {
			t.writes = append(t.writes, t.route(write.Register, write.Value))
		}
		return t, nil
	}
	if err = s.spend(12); err != nil {
		return nil, err
	} // selected TL resource group
	if err = s.spend(8); err != nil {
		return nil, err
	}
	if err = s.spend(12); err != nil {
		return nil, err
	} // actual loaded algorithm
	if err = s.spend(8); err != nil {
		return nil, err
	}
	if err = s.spend(14); err != nil {
		return nil, err
	}
	if err = s.spend(12); err != nil {
		return nil, err
	} // live base attenuation
	t.negativeLevel = work.BaseLevel >= 128
	if err = s.branch(t.negativeLevel, false); err != nil {
		return nil, err
	}
	if t.negativeLevel {
		return t, nil
	}
	if err = s.spend(4); err != nil {
		return nil, err
	}
	patch := work.Patch // temporary selected TL/loaded algorithm view
	patch.Algorithm = work.LoadedAlgorithm
	mask := instrumentCarriers[work.LoadedAlgorithm]
	for source, slot := range instrumentSlots {
		patch.Operators[slot].Carrier = mask&(1<<source) != 0
	}
	writes, err := param.FMLevelUpdates(patch, work.BaseLevel, false)
	if err != nil {
		return nil, err
	}
	for _, write := range writes {
		t.writes = append(t.writes, t.route(0x40+write.Slot*4, write.TotalLevel))
	}
	return t, nil
}

func (t *InstrumentTransfer) route(register, value uint8) Request {
	return Request{Bank: t.request.Channel >> 2, Register: register + (t.request.Channel & 3), Value: value}
}
func (t *InstrumentTransfer) ExpectedWrites() []Request { return append([]Request(nil), t.writes...) }
func (t *InstrumentTransfer) Clock() Clock              { return t.s.clock }
func (t *InstrumentTransfer) fail(err error) error      { t.failure = err; return err }

func (t *InstrumentTransfer) voiceWrite(write Request) error {
	if err := t.s.spend(18); err != nil {
		return err
	} // original caller
	if err := t.s.spend(12); err != nil {
		return err
	}
	if err := t.s.spend(12); err != nil {
		return err
	}
	if err := t.s.branch(t.request.Channel&4 != 0, false); err != nil {
		return err
	}
	if err := t.s.spend(4); err != nil {
		return err
	}
	return t.s.serveYM(write)
}

func (t *InstrumentTransfer) loadNext(write Request) error {
	s := t.s
	switch {
	case t.cursor == 0:
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		}
	case t.cursor == 1:
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.spend(4); err != nil {
			return err
		}
	case t.cursor <= 20:
		if err := s.pauseLoopEnd(false); err != nil {
			return err
		}
	case t.cursor == 21:
		if err := s.pauseLoopEnd(true); err != nil {
			return err
		}
		if err := s.spend(4); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.spend(14); err != nil {
			return err
		}
		if err := s.spend(12); err != nil {
			return err
		}
	default:
		if err := s.pauseLoopEnd(false); err != nil {
			return err
		}
	}
	if t.cursor != 0 {
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		}
	}
	if t.cursor >= 21 {
		if err := s.spend(8); err != nil {
			return err
		}
		carrier := instrumentCarriers[t.request.Patch.Algorithm]&(1<<uint(t.cursor-21)) != 0
		if err := s.branch(!carrier, false); err != nil {
			return err
		}
		if carrier {
			if err := s.spend(4); err != nil {
				return err
			}
		}
	}
	return t.voiceWrite(write)
}

// levelNext consumes non-writing operators at their real stages. It returns
// at one actual expected write, leaving that iteration's DBF for the next
// phase. Finish consumes the remaining non-writing tail and final return.
func (t *InstrumentTransfer) levelNext(write *Request) error {
	s := t.s
	if t.levelPending {
		if err := s.pauseLoopEnd(t.operator == 4); err != nil {
			return err
		}
		t.levelPending = false
	}
	for t.operator < 4 {
		op := t.operator
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		}
		if err := s.spend(8); err != nil {
			return err
		}
		carrier := instrumentCarriers[t.request.LoadedAlgorithm]&(1<<uint(op)) != 0
		if err := s.branch(!carrier, false); err != nil {
			return err
		}
		carry := false
		if carrier {
			if err := s.spend(4); err != nil {
				return err
			}
			source := t.request.Patch.Operators[instrumentSlots[op]]
			level := uint16(source.TotalLevel) + uint16(t.request.BaseLevel)
			if source.VolumeCarryBias {
				level += 128
			}
			carry = level > 255
			if err := s.branch(carry, false); err != nil {
				return err
			}
		}
		t.operator++
		if carrier && !carry {
			if write == nil {
				return ErrInstrumentSequence
			}
			if err := t.voiceWrite(*write); err != nil {
				return err
			}
			t.levelPending = true
			return nil
		}
		if err := s.pauseLoopEnd(t.operator == 4); err != nil {
			return err
		}
	}
	if write != nil {
		return ErrInstrumentSequence
	}
	return nil
}

func (t *InstrumentTransfer) WriteNext(expected Request) error {
	if t == nil {
		return ErrInstrumentSequence
	}
	if t.done || t.failure != nil || t.cursor >= len(t.writes) || expected != t.writes[t.cursor] {
		return t.fail(ErrInstrumentSequence)
	}
	var err error
	if t.request.Mode == "load" {
		err = t.loadNext(expected)
	} else {
		err = t.levelNext(&expected)
	}
	if err != nil {
		return t.fail(err)
	}
	t.cursor++
	return nil
}

func (t *InstrumentTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrInstrumentSequence
	}
	if t.done || t.failure != nil || t.cursor != len(t.writes) {
		return Clock{}, t.fail(ErrInstrumentSequence)
	}
	var err error
	if t.request.Claimed || t.negativeLevel {
		err = t.s.spend(16)
	} else if t.request.Mode == "load" {
		err = t.s.pauseLoopEnd(true) // stop before pan-routing tail, no second return
	} else {
		err = t.levelNext(nil)
		if err == nil {
			err = t.s.spend(16)
		}
	}
	if err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return t.s.clock, nil
}

func ServeInstrument(clock Clock, request InstrumentRequest, bus Bus, chip Chip, limits Limits) (Clock, error) {
	t, err := BeginInstrumentTransfer(clock, request, bus, chip, limits)
	if err != nil {
		return Clock{}, err
	}
	for _, write := range t.ExpectedWrites() {
		if err = t.WriteNext(write); err != nil {
			return Clock{}, err
		}
	}
	return t.Finish()
}
