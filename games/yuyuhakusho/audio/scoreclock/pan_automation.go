package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

var ErrPanAutomationSequence = errors.New("invalid pan automation sequence")

type PanAutomationRequest struct {
	Channel                  uint8
	Definition               param.PanDefinition
	State                    param.PanState
	OnNote, NoteOff, Claimed bool
	Pan                      score.Pan
}

// PanAutomationTransfer prices a note or held-note automation call around the
// existing routing service. It stores neither counters nor a persistent pan.
type PanAutomationTransfer struct {
	s             *service
	pan           *PanRouting
	applied, done bool
	failure       error
}

func (s *service) panAutomationDispatch(noteOff bool) error {
	if err := s.spend(12); err != nil {
		return err
	}
	if err := s.branch(noteOff, false); err != nil {
		return err
	}
	if noteOff {
		return nil
	}
	return s.ordinaryCosts(4, 12, 8, 14)
}

func BeginPanAutomation(clock Clock, request PanAutomationRequest, catalog *score.Catalog, bus Bus, chip Chip, limits Limits) (*PanAutomationTransfer, error) {
	panRequest := PanRequest{Entry: "voice", Channel: request.Channel, Claimed: request.Claimed, Pan: request.Pan}
	if !validPanRequest(panRequest) {
		return nil, ErrPanAutomationSequence
	}
	// Only FM6 can redirect into the PCM owner's pan mailbox. Ordinary FM
	// voices need the existing bus interface without an invented PCM owner.
	panBus, _ := bus.(PanBus)
	if request.Channel == 6 && panBus == nil {
		return nil, ErrPanAutomationSequence
	}
	tick, err := catalog.PanAutomationStep(request.Definition, request.State, request.OnNote, request.NoteOff)
	if err != nil {
		return nil, err
	}
	s, err := pauseService(clock, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	t := &PanAutomationTransfer{s: s}
	if err = s.panAutomationDispatch(request.NoteOff); err != nil {
		return nil, err
	}
	mode := request.Definition.Mode
	if request.NoteOff || mode == param.PanDisabled || !request.OnNote && mode == param.PanAdvanceOnNote {
		return t, nil
	}
	reset := mode == param.PanRestartEveryDriverTick || request.OnNote && (mode == param.PanRestartOnNoteOnce || mode == param.PanRestartOnNoteLoop)
	if request.OnNote && mode == param.PanRestartEveryDriverTick {
		// The note table's last entry is the held-note header itself.
		if err = s.panAutomationDispatch(false); err != nil {
			return nil, err
		}
	} else if mode != param.PanRestartEveryDriverTick {
		if err = s.spend(10); err != nil {
			return nil, err
		}
	}
	// Local values select fixed cost branches only. AdvancePan above supplies
	// the semantic output; the score driver alone advances actual counters.
	elapsed, step := request.State.Elapsed, request.State.Step
	if reset {
		if err = s.ordinaryCosts(20, 16); err != nil {
			return nil, err
		}
		elapsed, step = request.Definition.Period, 0
	}
	if err = s.ordinaryCosts(12, 12); err != nil {
		return nil, err
	}
	if err = s.branch(elapsed != request.Definition.Period, false); err != nil {
		return nil, err
	}
	if elapsed == request.Definition.Period {
		if err = s.ordinaryCosts(12, 12); err != nil {
			return nil, err
		}
		withinLimit := int8(request.Definition.StepLimit-step) >= 0
		if err = s.branch(withinLimit, false); err != nil {
			return nil, err
		}
		if !withinLimit {
			if err = s.spend(16); err != nil {
				return nil, err
			}
			once := mode == param.PanRestartOnNoteOnce
			if err = s.branch(once, false); err != nil {
				return nil, err
			}
			if once {
				return t, nil
			}
			if err = s.spend(16); err != nil {
				return nil, err
			}
		}
		if err = s.ordinaryCosts(16, 16); err != nil {
			return nil, err
		}
	}
	if err = s.ordinaryCosts(4, 12, 4, 10, 18, 4, 12, 4, 14, 12, 8, 4, 18); err != nil {
		return nil, err
	}
	if tick.Pan == nil {
		return nil, ErrPanAutomationSequence
	}
	writePan := request.Pan
	writePan.Left = *tick.Pan == param.Left || *tick.Pan == param.Both
	writePan.Right = *tick.Pan == param.Right || *tick.Pan == param.Both
	panRequest.WritePan = &writePan
	t.pan, err = beginPanRouting(s, panRequest, panBus)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *PanAutomationTransfer) Clock() Clock { return t.s.clock }
func (t *PanAutomationTransfer) ExpectedEffect() (PanEffect, bool) {
	if t.pan == nil {
		return PanEffect{}, false
	}
	return t.pan.ExpectedEffect()
}
func (t *PanAutomationTransfer) fail(err error) error { t.failure = err; return err }

func (t *PanAutomationTransfer) Apply(effect PanEffect) error {
	if t == nil {
		return ErrPanAutomationSequence
	}
	if t.done || t.failure != nil || t.applied || t.pan == nil {
		return t.fail(ErrPanAutomationSequence)
	}
	if expected, ok := t.pan.ExpectedEffect(); !ok || expected != effect {
		return t.fail(ErrPanAutomationSequence)
	}
	if err := t.pan.Apply(effect); err != nil {
		return t.fail(err)
	}
	t.applied = true
	return nil
}

func (t *PanAutomationTransfer) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrPanAutomationSequence
	}
	if t.done || t.failure != nil {
		return Clock{}, t.fail(ErrPanAutomationSequence)
	}
	if t.pan != nil {
		if _, expected := t.pan.ExpectedEffect(); expected && !t.applied {
			return Clock{}, t.fail(ErrPanAutomationSequence)
		}
		if _, err := t.pan.Finish(); err != nil {
			return Clock{}, t.fail(err)
		}
		if err := t.s.spend(16); err != nil { // increment elapsed after routing
			return Clock{}, t.fail(err)
		}
	}
	if err := t.s.spend(16); err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return t.s.clock, nil
}
