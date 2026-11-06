package scoreclock

import "errors"

var ErrFrequencySequence = errors.New("invalid frequency transfer sequence")

// FrequencyRequest supplies an already calculated pitch. FM3 mode is outside
// the normal score closure and is rejected rather than silently left unpriced.
type FrequencyRequest struct {
	Channel uint8
	Word    uint16
	FM3Mode bool
}

// FrequencyTransfer separates the two actual score writes. Both use the same
// temporary clock and budget; the chip retains all latch and synthesis state.
type FrequencyTransfer struct {
	s       *service
	chip    KeyGateChip
	channel uint8
	effects [2]KeyEffect
	next    int
	done    bool
	failure error
}

func BeginFrequencyTransfer(clock Clock, request FrequencyRequest, bus Bus, chip KeyGateChip, limits Limits) (*FrequencyTransfer, error) {
	if !validKeyChannel(request.Channel) || request.FM3Mode {
		return nil, ErrFrequencySequence
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	return beginFrequencyTransfer(s, request, chip)
}

func beginFrequencyTransfer(s *service, request FrequencyRequest, chip KeyGateChip) (*FrequencyTransfer, error) {
	var err error
	t := &FrequencyTransfer{s: s, chip: chip, channel: request.Channel}
	if request.Channel < 128 {
		// The audited normal FM3 field is zero. Its taken branch is part of
		// this caller, before preparation of the first high-byte write.
		if err = s.spend(12); err != nil {
			return nil, err
		}
		if err = s.branch(true, false); err != nil {
			return nil, err
		}
		t.effects = [2]KeyEffect{
			{Kind: "ym", YM: Request{Bank: request.Channel >> 2, Register: 0xa4 + (request.Channel & 3), Value: uint8(request.Word>>8) & 63}},
			{Kind: "ym", YM: Request{Bank: request.Channel >> 2, Register: 0xa0 + (request.Channel & 3), Value: uint8(request.Word)}},
		}
		return t, nil
	}
	if err = s.ordinaryCosts(12, 8); err != nil {
		return nil, err
	}
	if err = s.branch(request.Channel != 224, false); err != nil {
		return nil, err
	}
	channel := request.Channel
	if channel == 224 {
		if err = s.spend(8); err != nil {
			return nil, err
		}
		channel = 192
	}
	if err = s.ordinaryCosts(4, 8, 4, 14, 8); err != nil {
		return nil, err
	}
	t.effects = [2]KeyEffect{
		{Kind: "psg", PSG: channel | uint8(request.Word&15)},
		{Kind: "psg", PSG: uint8(request.Word>>4) & 63},
	}
	return t, nil
}

func (t *FrequencyTransfer) Clock() Clock { return t.s.clock }

// ExpectedEffects returns detached values in their actual high/low or PSG
// latch/data order. Changing the returned slice cannot alter this transfer.
func (t *FrequencyTransfer) ExpectedEffects() []KeyEffect {
	return append([]KeyEffect(nil), t.effects[:]...)
}

func (t *FrequencyTransfer) fail(err error) error { t.failure = err; return err }

func (t *FrequencyTransfer) WriteNext(effect KeyEffect) error {
	if t == nil {
		return ErrFrequencySequence
	}
	if t.done || t.failure != nil || t.next == len(t.effects) || effect != t.effects[t.next] {
		return t.fail(ErrFrequencySequence)
	}
	s := t.s
	if effect.Kind == "psg" {
		if err := psgAbsoluteStore(s, t.chip, effect.PSG); err != nil {
			return t.fail(err)
		}
	} else {
		if t.next == 0 {
			if err := s.ordinaryCosts(4, 22, 8, 18); err != nil {
				return t.fail(err)
			}
		} else if err := s.ordinaryCosts(4, 8, 18); err != nil {
			return t.fail(err)
		}
		if err := s.ordinaryCosts(12, 12); err != nil {
			return t.fail(err)
		}
		if err := s.branch(t.channel&4 != 0, false); err != nil {
			return t.fail(err)
		}
		if err := s.spend(4); err != nil {
			return t.fail(err)
		}
		if err := s.serveYM(effect.YM); err != nil {
			return t.fail(err)
		}
	}
	t.next++
	return nil
}

func (t *FrequencyTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrFrequencySequence
	}
	if t.done || t.failure != nil || t.next != len(t.effects) {
		return Clock{}, t.fail(ErrFrequencySequence)
	}
	if err := t.s.spend(16); err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return t.s.clock, nil
}
