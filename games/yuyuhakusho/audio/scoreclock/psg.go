package scoreclock

import (
	"errors"
	"math"
)

// PSGChip is the external, unique PSG synthesis owner. *chips.Interactive
// implements it directly; this service keeps no latch or register shadow.
type PSGChip interface {
	WritePSG(cycle uint64, value uint8) error
}

// PSGKind selects one fixed leaf from an already prepared canonical write.
type PSGKind uint8

const (
	PSGSingle PSGKind = iota + 1
	PSGPitchPair
	PSGMuteAll
)

// PSGRequest carries physical bytes, never caller-supplied costs or programs.
// Single uses Value only. PitchPair uses a tone latch in Value and a six-bit
// continuation in Data. MuteAll requires both payload bytes to be zero.
type PSGRequest struct {
	Kind        PSGKind
	Value, Data uint8
}

func newPSGService(clock Clock, chip PSGChip, limits Limits) (*service, error) {
	if chip == nil || limits.MaxOperations > 65536 || clock.Cycle > math.MaxUint64-896 {
		return nil, errors.New("invalid PSG clock, owner or operation limit")
	}
	limit := limits.MaxOperations
	if limit == 0 {
		limit = 4096
	}
	return &service{clock: clock, limit: limit}, nil
}

func psgAbsoluteStore(s *service, chip PSGChip, value uint8) error {
	// PSG receives the current main cursor directly. The YM/ZRAM seven-master
	// access delay does not apply to this independent VDP PSG byte path.
	return s.cost(16, func() error { return chip.WritePSG(s.clock.Cycle, value) })
}

// StorePSG performs only one canonical absolute data-register PSG byte store.
// It includes the store's refresh boundary and fixed cost, with no return,
// caller preparation or call cost. Use a detached owner and commit on success;
// external errors are not silently rolled back and frames are not flushed.
func StorePSG(clock Clock, value uint8, chip PSGChip, limits Limits) (Clock, error) {
	s, err := newPSGService(clock, chip, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = psgAbsoluteStore(s, chip, value); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}

func validPSGRequest(request PSGRequest) bool {
	switch request.Kind {
	case PSGSingle:
		return request.Data == 0
	case PSGPitchPair:
		if request.Data > 0x3f {
			return false
		}
		switch request.Value & 0xf0 {
		case 0x80, 0xa0, 0xc0:
			return true
		}
	case PSGMuteAll:
		return request.Value == 0 && request.Data == 0
	}
	return false
}

// ServePSG performs one fixed single, pitch-pair or mute-all leaf, including
// exactly one return. It starts after caller preparation and any call cost.
// Callers retain the same detached-owner and explicit-frame obligations as
// StorePSG. An error may follow prior writes; discard that transaction.
func ServePSG(clock Clock, request PSGRequest, chip PSGChip, limits Limits) (Clock, error) {
	if !validPSGRequest(request) {
		return Clock{}, errors.New("invalid canonical PSG leaf request")
	}
	s, err := newPSGService(clock, chip, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.servePSG(request, chip); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}

// servePSG uses the same shared operation budget as its fixed caller.
func (s *service) servePSG(request PSGRequest, chip PSGChip) error {
	var err error
	switch request.Kind {
	case PSGSingle:
		err = psgAbsoluteStore(s, chip, request.Value)
	case PSGPitchPair:
		if err = psgAbsoluteStore(s, chip, request.Value); err == nil {
			err = psgAbsoluteStore(s, chip, request.Data)
		}
	case PSGMuteAll:
		scan, failure := beginMutePSG(s, chip)
		if failure != nil {
			return failure
		}
		for _, value := range mutePSGValues {
			if failure = scan.next(value); failure != nil {
				return failure
			}
		}
		return nil
	}
	if err != nil {
		return err
	}
	if err = s.spend(16); err != nil {
		return err
	} // return from this leaf
	return nil
}
