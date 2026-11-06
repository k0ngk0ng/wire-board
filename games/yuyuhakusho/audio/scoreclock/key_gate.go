package scoreclock

import "errors"

var ErrKeySequence = errors.New("invalid key transfer sequence")
var ErrGateSequence = errors.New("invalid gate transfer sequence")

type KeyGateChip interface {
	Chip
	PSGChip
}
type KeyRequest struct {
	Channel                    uint8
	On, NoteOff, Claimed, Tied bool
}
type KeyEffect struct {
	Kind string
	YM   Request
	PSG  uint8
}
type KeyTransfer struct {
	s             *service
	chip          KeyGateChip
	request       KeyRequest
	effect        KeyEffect
	applied, done bool
	failure       error
}

func validKeyChannel(channel uint8) bool {
	switch channel {
	case 0, 1, 2, 4, 5, 6, 128, 160, 192, 224:
		return true
	}
	return false
}

func beginKey(s *service, request KeyRequest, chip KeyGateChip) (*KeyTransfer, error) {
	t := &KeyTransfer{s: s, chip: chip, request: request}
	if request.Channel >= 128 {
		if err := s.spend(12); err != nil {
			return nil, err
		}
		if err := s.branch(request.Claimed, false); err != nil {
			return nil, err
		}
		if !request.Claimed {
			t.effect = KeyEffect{Kind: "psg", PSG: request.Channel | 31}
		}
		return t, nil
	}
	first := request.Tied
	if request.On {
		first = request.NoteOff
	}
	if err := s.spend(12); err != nil {
		return nil, err
	}
	if err := s.branch(first, false); err != nil {
		return nil, err
	}
	if first {
		return t, nil
	}
	if err := s.spend(12); err != nil {
		return nil, err
	}
	if err := s.branch(request.Claimed, false); err != nil {
		return nil, err
	}
	if request.Claimed {
		return t, nil
	}
	value := request.Channel
	if request.On {
		value |= 240
	}
	t.effect = KeyEffect{Kind: "ym", YM: Request{Register: 0x28, Value: value}}
	return t, nil
}

func BeginKeyTransfer(clock Clock, request KeyRequest, bus Bus, chip KeyGateChip, limits Limits) (*KeyTransfer, error) {
	if !validKeyChannel(request.Channel) || request.On && request.Channel >= 128 {
		return nil, ErrKeySequence
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	return beginKey(s, request, chip)
}
func (t *KeyTransfer) ExpectedEffect() (KeyEffect, bool) { return t.effect, t.effect.Kind != "" }
func (t *KeyTransfer) Clock() Clock                      { return t.s.clock }
func (t *KeyTransfer) fail(err error) error              { t.failure = err; return err }
func (t *KeyTransfer) Apply(effect KeyEffect) error {
	if t == nil {
		return ErrKeySequence
	}
	if t.done || t.failure != nil || t.applied || t.effect.Kind == "" || effect != t.effect {
		return t.fail(ErrKeySequence)
	}
	s := t.s
	if effect.Kind == "psg" {
		if err := s.ordinaryCosts(12, 8); err != nil {
			return t.fail(err)
		}
		if err := s.cost(16, func() error { return t.chip.WritePSG(s.clock.Cycle, effect.PSG) }); err != nil {
			return t.fail(err)
		}
	} else {
		if err := s.ordinaryCosts(4, 12); err != nil {
			return t.fail(err)
		}
		if t.request.On {
			if err := s.spend(8); err != nil {
				return t.fail(err)
			}
		}
		if err := s.ordinaryCosts(10, 10); err != nil {
			return t.fail(err)
		}
		if err := s.serveYM(effect.YM); err != nil {
			return t.fail(err)
		}
	}
	t.applied = true
	return nil
}
func (t *KeyTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrKeySequence
	}
	if t.done || t.failure != nil || t.effect.Kind != "" && !t.applied {
		return Clock{}, t.fail(ErrKeySequence)
	}
	// The FM tail branches into canonical YM, whose own return is the key
	// return. Suppressed and direct PSG paths still owe their final RTS.
	if t.effect.Kind != "ym" {
		if err := t.s.spend(16); err != nil {
			return Clock{}, t.fail(err)
		}
	}
	t.done = true
	return t.s.clock, nil
}

type GateRequest struct {
	Channel, Remaining uint8
	Claimed, Tied      bool
}
type GateTransfer struct {
	s             *service
	key           *KeyTransfer
	expired, done bool
	failure       error
}

func BeginGateTransfer(clock Clock, request GateRequest, bus Bus, chip KeyGateChip, limits Limits) (*GateTransfer, error) {
	if !validKeyChannel(request.Channel) {
		return nil, ErrGateSequence
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	t := &GateTransfer{s: s, expired: request.Remaining == 1}
	if err = s.spend(12); err != nil {
		return nil, err
	}
	if err = s.branch(request.Remaining == 0, false); err != nil {
		return nil, err
	}
	if request.Remaining == 0 {
		return t, nil
	}
	if err = s.spend(16); err != nil {
		return nil, err
	}
	if err = s.branch(!t.expired, false); err != nil {
		return nil, err
	}
	if !t.expired {
		return t, nil
	}
	if err = s.ordinaryCosts(16, 12); err != nil {
		return nil, err
	}
	if err = s.branch(request.Channel >= 128, true); err != nil {
		return nil, err
	}
	if err = s.spend(18); err != nil {
		return nil, err
	}
	t.key, err = beginKey(s, KeyRequest{Channel: request.Channel, Claimed: request.Claimed, Tied: request.Tied, NoteOff: true}, chip)
	if err != nil {
		return nil, err
	}
	return t, nil
}
func (t *GateTransfer) Expired() bool { return t.expired }
func (t *GateTransfer) Clock() Clock  { return t.s.clock }
func (t *GateTransfer) ExpectedEffect() (KeyEffect, bool) {
	if t.key == nil {
		return KeyEffect{}, false
	}
	return t.key.ExpectedEffect()
}
func (t *GateTransfer) fail(err error) error { t.failure = err; return err }
func (t *GateTransfer) Apply(effect KeyEffect) error {
	if t == nil {
		return ErrGateSequence
	}
	if t.done || t.failure != nil || t.key == nil {
		return t.fail(ErrGateSequence)
	}
	if err := t.key.Apply(effect); err != nil {
		return t.fail(err)
	}
	return nil
}
func (t *GateTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrGateSequence
	}
	if t.done || t.failure != nil {
		return Clock{}, t.fail(ErrGateSequence)
	}
	if t.key != nil {
		if _, err := t.key.Finish(); err != nil {
			return Clock{}, t.fail(err)
		}
		// Expiry skips the held-note caller and returns directly. This is a
		// semantic outcome/cost, never an emulated persistent source stack.
		if err := t.s.spend(8); err != nil {
			return Clock{}, t.fail(err)
		}
	}
	if err := t.s.spend(16); err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return t.s.clock, nil
}
