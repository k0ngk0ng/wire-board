package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrControlWorkUnavailable = errors.New("control Work clock unavailable")

type ControlParameterWork struct {
	Action, Family                             string
	ImmediateFM, PanDisabled, Enabled, Claimed bool
	Noise                                      uint8
}

type ControlResult struct {
	Clock Clock
	Next  string
}

func validControlWork(w ControlParameterWork) bool {
	if !noteFamily(w.Family) || w.ImmediateFM && (w.Action != "level-add" || w.Family != "fm") || w.PanDisabled && w.Action != "pan-automation" || w.Enabled && w.Action != "modulation-enable" || w.Action != "psg-noise" && w.Noise != 0 {
		return false
	}
	switch w.Action {
	case "pan", "detune", "level-add", "tie", "gate", "tempo", "enqueue-replace-first", "transpose-add", "modulation", "modulation-enable":
		return true
	case "pan-automation", "fm-patch", "fm-patch-select":
		return w.Family == "fm"
	case "volume-envelope":
		return w.Family == "psg"
	case "psg-noise":
		return w.Family == "psg" && w.Noise&0xf8 == 0xe0
	}
	return false
}

// controlPrefix stops at a declared external effect/continuation, or after
// the actual handler return. It does not publish, run another leaf or add a
// caller/backedge. Field mutations remain solely score-owned.
func (s *service) controlPrefix(w ControlParameterWork) (string, error) {
	complete := func(err error) (string, error) { return "complete", err }
	switch w.Action {
	case "pan":
		if err := s.ordinaryCosts(8, 12); err != nil {
			return "", err
		}
		psg := w.Family == "psg"
		if err := s.branch(psg, false); err != nil {
			return "", err
		}
		if psg {
			return complete(s.spend(16))
		}
		if err := s.ordinaryCosts(12, 8, 4, 12, 10); err != nil {
			return "", err
		}
		return "pan", nil
	case "detune", "fm-patch-select", "volume-envelope":
		return complete(s.ordinaryCosts(16, 16))
	case "pan-automation":
		if err := s.spend(16); err != nil {
			return "", err
		}
		if err := s.branch(w.PanDisabled, false); err != nil {
			return "", err
		}
		if w.PanDisabled {
			if err := s.ordinaryCosts(12, 10); err != nil {
				return "", err
			}
			return "pan", nil
		}
		return complete(s.ordinaryCosts(16, 16, 16, 16, 16, 16))
	case "level-add":
		if err := s.ordinaryCosts(8, 16); err != nil {
			return "", err
		}
		if w.ImmediateFM {
			if err := s.spend(10); err != nil {
				return "", err
			}
			return "level", nil
		}
		return complete(s.spend(16))
	case "tie":
		return complete(s.ordinaryCosts(16, 16))
	case "gate", "tempo":
		return complete(s.ordinaryCosts(16, 16, 16))
	case "enqueue-replace-first":
		return "queue", nil // stop before the shared Root queue store
	case "fm-patch":
		if err := s.ordinaryCosts(4, 8, 12); err != nil {
			return "", err
		}
		return "instrument", nil
	case "modulation":
		if err := s.ordinaryCosts(20, 16); err != nil {
			return "", err
		}
		if err := s.modulationSeed(); err != nil {
			return "", err
		}
		return complete(s.spend(16))
	case "modulation-enable":
		return complete(s.ordinaryCosts(20, 16))
	case "transpose-add":
		return complete(s.ordinaryCosts(8, 16, 16))
	case "psg-noise":
		if err := s.ordinaryCosts(16, 16, 12); err != nil {
			return "", err
		}
		if err := s.branch(w.Claimed, false); err != nil {
			return "", err
		}
		if w.Claimed {
			return complete(s.spend(16))
		}
		return "psg-noise", nil
	}
	return "", ErrControlWorkUnavailable
}

func PriceControlWork(clock Clock, work ControlParameterWork, limits Limits) (ControlResult, error) {
	if !validControlWork(work) {
		return ControlResult{}, ErrControlWorkUnavailable
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return ControlResult{}, err
	}
	next, err := s.controlPrefix(work)
	if err != nil {
		return ControlResult{}, err
	}
	return ControlResult{Clock: s.clock, Next: next}, nil
}

func PriceObservedControl(clock Clock, op score.Operation, limits Limits) (ControlResult, error) {
	if op.Version != 1 || op.Kind != score.OpParameter || op.Routine != score.RoutineControl || op.VoiceSlot < 0 || op.VoiceSlot >= 18 {
		return ControlResult{}, ErrControlWorkUnavailable
	}
	family := ""
	switch op.Channel {
	case "dac":
		family = "dac"
	case "fm1", "fm2", "fm3", "fm4", "fm5", "fm6":
		family = "fm"
	case "psg1", "psg2", "psg3", "psg-noise":
		family = "psg"
	}
	return PriceControlWork(clock, ControlParameterWork{Action: op.EventKind, Family: family, ImmediateFM: op.Control.ImmediateFM, PanDisabled: op.Control.PanDisabled, Enabled: op.Control.Enabled, Noise: op.Control.Noise, Claimed: op.Flags.Claimed}, limits)
}

// ServeNoiseControl completes the exact noise handler on its unique PSG
// owner. Unlike the ordinary prepared-byte leaf, the source store includes
// its operand read and costs 24 CPU cycles with no YM/ZRAM access delay.
func ServeNoiseControl(clock Clock, work ControlParameterWork, chip PSGChip, limits Limits) (Clock, error) {
	if work.Action != "psg-noise" || !validControlWork(work) {
		return Clock{}, ErrControlWorkUnavailable
	}
	s, err := newPSGService(clock, chip, limits)
	if err != nil {
		return Clock{}, err
	}
	next, err := s.controlPrefix(work)
	if err != nil {
		return Clock{}, err
	}
	if next == "psg-noise" {
		if err = s.cost(24, func() error { return chip.WritePSG(s.clock.Cycle, work.Noise) }); err != nil {
			return Clock{}, err
		}
		if err = s.spend(16); err != nil {
			return Clock{}, err
		}
	}
	return s.clock, nil
}

// ServeQueueControl wraps the caller's actual canonical effect at the original
// shared queue-store point. The Root callback must not apply that effect again.
func ServeQueueControl(clock Clock, op score.Operation, effect func(at uint64) error, limits Limits) (Clock, error) {
	if effect == nil || op.Version != 1 || op.Kind != score.OpQueueReplace || op.WriteCount != 1 || op.Writes[0].Order != op.IntentOffset {
		return Clock{}, ErrControlWorkUnavailable
	}
	w := op.Writes[0]
	if w.Kind != "queue" || w.Action != "replace-first" || w.Pan != nil || w.Bank != 0 || w.Register != 0 || w.Value != 0 || w.DescriptorID != "" {
		return Clock{}, ErrControlWorkUnavailable
	}
	s, err := tickService(clock, limits)
	if err != nil {
		return Clock{}, err
	}
	if err = s.cost(16, func() error { return effect(s.clock.Cycle) }); err != nil {
		return Clock{}, err
	}
	if err = s.spend(16); err != nil {
		return Clock{}, err
	}
	return s.clock, nil
}
