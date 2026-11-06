package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrPanSequence = errors.New("invalid pan routing sequence")

type PanBus interface {
	Bus
	PanRequest(at uint64) (uint8, error)
	WriteRestorePan(at uint64, value uint8) error
}

// PanRequest describes a legal FM voice row. The instrument entry begins at
// the load tail, after its upstream claimed guard has already succeeded.
type PanRequest struct {
	Entry   string
	Channel uint8
	Claimed bool
	Pan     score.Pan
	// WritePan is the ordinary voice call's YM argument. Nil uses Pan.
	// DAC redirection always uses Pan; instrument tails load Pan themselves.
	WritePan *score.Pan
}

type PanEffect struct {
	Kind       string
	Write      Request
	RestorePan uint8
}

// PanRouting is temporary work on detached owners. Begin stops before the
// corresponding score intent's effect; Apply and Finish share the same clock
// and budget. Nothing here is a persistent score/PCM state or event history.
type PanRouting struct {
	s       *service
	bus     PanBus
	request PanRequest
	effect  PanEffect
	applied bool
	done    bool
	failure error
}

func validPanRequest(request PanRequest) bool {
	if request.Entry != "voice" && request.Entry != "instrument" || request.Pan.AMS > 3 || request.Pan.FMS > 7 || request.Entry == "instrument" && request.Claimed {
		return false
	}
	if request.WritePan != nil && (request.WritePan.AMS > 3 || request.WritePan.FMS > 7 || request.Entry == "instrument" && *request.WritePan != request.Pan) {
		return false
	}
	switch request.Channel {
	case 0, 1, 2, 4, 5, 6:
		return true
	}
	return false
}

func panByte(p score.Pan) uint8 {
	value := p.AMS<<4 | p.FMS
	if p.Left {
		value |= 128
	}
	if p.Right {
		value |= 64
	}
	return value
}

// panAcquire deliberately does not call acquire: pan waits only for BUSACK,
// then reads the request marker; it never tests the player's busy mailbox.
func (s *service) panAcquire() error {
	if err := s.request(20, true); err != nil {
		return err
	}
	for {
		granted := false
		if err := s.cost(24, func() error {
			var err error
			granted, err = s.bus.BusGranted(s.clock.Cycle)
			if err != nil {
				return err
			}
			after, err := s.bus.RequestBus(s.clock.Cycle, true)
			if err != nil {
				return err
			}
			if after < s.clock.Cycle {
				return errors.New("pan bus returned a backwards main cursor")
			}
			s.clock.Cycle = after
			return nil
		}); err != nil {
			return err
		}
		if err := s.branch(!granted, false); err != nil {
			return err
		}
		if granted {
			return nil
		}
	}
}

func (t *PanRouting) releasePanBus(redirected bool) error {
	if err := t.s.request(20, false); err != nil {
		return err
	}
	if err := t.s.spend(16); err != nil {
		return err
	}
	if err := t.s.spend(4); err != nil {
		return err
	}
	return t.s.branch(redirected, false)
}

func BeginPanRouting(clock Clock, request PanRequest, bus PanBus, chip Chip, limits Limits) (*PanRouting, error) {
	if !validPanRequest(request) {
		return nil, ErrPanSequence
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	storedValue := panByte(request.Pan)
	writeValue := storedValue
	if request.WritePan != nil {
		writeValue = panByte(*request.WritePan)
	}
	request.WritePan = nil // the derived value above is detached from its caller
	t := &PanRouting{s: s, bus: bus, request: request}
	if request.Entry == "voice" {
		if err = s.spend(12); err != nil {
			return nil, err
		}
		if err = s.branch(request.Claimed, false); err != nil {
			return nil, err
		}
		if request.Claimed {
			return t, nil
		}
	}
	t.effect = PanEffect{Kind: "ym", Write: Request{Bank: request.Channel >> 2, Register: 0xb4 + (request.Channel & 3), Value: writeValue}}
	if err = s.spend(16); err != nil {
		return nil, err
	}
	if err = s.branch(request.Channel != 6, true); err != nil {
		return nil, err
	}
	if request.Channel != 6 {
		return t, nil
	}
	// Legal voice rows exclude the special non-voice header pointer.
	for _, cost := range []uint64{14, 12, 14, 20} {
		if err = s.spend(cost); err != nil {
			return nil, err
		}
	}
	if err = s.panAcquire(); err != nil {
		return nil, err
	}
	var marker uint8
	if err = s.cost(16, func() error {
		if err := s.add(7); err != nil {
			return err
		}
		var err error
		marker, err = bus.PanRequest(s.clock.Cycle)
		return err
	}); err != nil {
		return nil, err
	}
	if err = s.branch(marker == 0, true); err != nil {
		return nil, err
	}
	if marker != 0 {
		t.effect = PanEffect{Kind: "dac-pan", RestorePan: storedValue}
		return t, nil
	}
	if err = t.releasePanBus(false); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *PanRouting) Clock() Clock { return t.s.clock }
func (t *PanRouting) ExpectedEffect() (PanEffect, bool) {
	return t.effect, t.effect.Kind != ""
}
func (t *PanRouting) fail(err error) error { t.failure = err; return err }

func (t *PanRouting) Apply(expected PanEffect) error {
	if t == nil {
		return ErrPanSequence
	}
	if t.done || t.failure != nil || t.applied || t.effect.Kind == "" || expected != t.effect {
		return t.fail(ErrPanSequence)
	}
	var err error
	if expected.Kind == "dac-pan" {
		err = t.s.cost(24, func() error {
			if err := t.s.add(7); err != nil {
				return err
			}
			return t.bus.WriteRestorePan(t.s.clock.Cycle, expected.RestorePan)
		})
	} else {
		err = t.applyYM(expected.Write)
	}
	if err != nil {
		return t.fail(err)
	}
	t.applied = true
	return nil
}

func (t *PanRouting) applyYM(write Request) error {
	if t.request.Entry == "instrument" {
		if err := t.s.spend(12); err != nil {
			return err
		}
	}
	// Fixed register selection/caller followed by the same voice routing and
	// canonical YM service used by the rest of the score consumer.
	for _, cost := range []uint64{8, 18, 12, 12} {
		if err := t.s.spend(cost); err != nil {
			return err
		}
	}
	if err := t.s.branch(t.request.Channel&4 != 0, false); err != nil {
		return err
	}
	if err := t.s.spend(4); err != nil {
		return err
	}
	return t.s.serveYM(write)
}

func (t *PanRouting) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrPanSequence
	}
	if t.done || t.failure != nil || t.effect.Kind != "" && !t.applied {
		return Clock{}, t.fail(ErrPanSequence)
	}
	if t.effect.Kind == "dac-pan" {
		if err := t.releasePanBus(true); err != nil {
			return Clock{}, t.fail(err)
		}
	}
	if err := t.s.spend(16); err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return t.s.clock, nil
}

func ServePan(clock Clock, request PanRequest, bus PanBus, chip Chip, limits Limits) (Clock, error) {
	t, err := BeginPanRouting(clock, request, bus, chip, limits)
	if err != nil {
		return Clock{}, err
	}
	if effect, ok := t.ExpectedEffect(); ok {
		if err = t.Apply(effect); err != nil {
			return Clock{}, err
		}
	}
	return t.Finish()
}
