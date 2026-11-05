package param

import "fmt"

type Stereo string

const (
	Left  Stereo = "left"
	Right Stereo = "right"
	Both  Stereo = "both"
)

func (s Stereo) Bits() uint8 {
	switch s {
	case Left:
		return 128
	case Right:
		return 64
	case Both:
		return 192
	}
	return 0
}

type PanPattern struct {
	ID    string   `json:"id"`
	Steps []Stereo `json:"steps"`
}

type PanMode string

const (
	PanDisabled               PanMode = "disabled"
	PanAdvanceOnNote          PanMode = "advance-on-note"
	PanRestartOnNoteOnce      PanMode = "restart-on-note-once"
	PanRestartOnNoteLoop      PanMode = "restart-on-note-loop"
	PanRestartEveryDriverTick PanMode = "restart-every-driver-tick"
)

type PanDefinition struct {
	Mode        PanMode `json:"mode"`
	Pattern     string  `json:"pattern,omitempty"`
	InitialStep uint8   `json:"initial_step,omitempty"`
	StepLimit   uint8   `json:"step_limit,omitempty"`
	Period      uint8   `json:"period,omitempty"`
}

type PanState struct {
	Step    uint8 `json:"step"`
	Elapsed uint8 `json:"elapsed"`
}

type PanTick struct {
	State PanState `json:"state"`
	Pan   *Stereo  `json:"pan"`
}

func (d PanDefinition) Validate(patterns []PanPattern) error {
	switch d.Mode {
	case PanDisabled:
		return nil
	case PanAdvanceOnNote, PanRestartOnNoteOnce, PanRestartOnNoteLoop, PanRestartEveryDriverTick:
	default:
		return fmt.Errorf("unknown pan mode")
	}
	if d.InitialStep > 3 || d.StepLimit > 3 {
		return fmt.Errorf("pan step exceeds proved shared window")
	}
	found := false
	for _, pattern := range patterns {
		if pattern.ID == d.Pattern {
			if found || len(pattern.Steps) < 4 {
				return fmt.Errorf("duplicate or incomplete pan pattern")
			}
			found = true
			for _, stereo := range pattern.Steps {
				if stereo != Left && stereo != Right && stereo != Both {
					return fmt.Errorf("unknown pan stereo value")
				}
			}
		}
	}
	if !found {
		return fmt.Errorf("unknown pan pattern")
	}
	return nil
}

func InitialPanState(d PanDefinition) PanState {
	return PanState{Step: d.InitialStep, Elapsed: d.Period}
}

// AdvancePan emits only stereo. The caller retains its lower LFO bits and owns
// hardware claim gating. Disabled/note-only/off calls preserve state exactly.
func AdvancePan(d PanDefinition, patterns []PanPattern, state PanState, noteBoundary, keyOff bool) (PanTick, error) {
	unchanged := PanTick{State: state}
	if err := d.Validate(patterns); err != nil {
		return unchanged, err
	}
	if keyOff || d.Mode == PanDisabled || (d.Mode == PanAdvanceOnNote && !noteBoundary) {
		return unchanged, nil
	}
	next := state
	if d.Mode == PanRestartEveryDriverTick || (noteBoundary && (d.Mode == PanRestartOnNoteOnce || d.Mode == PanRestartOnNoteLoop)) {
		next = PanState{Elapsed: d.Period}
	}
	if next.Elapsed == d.Period {
		if int8(d.StepLimit-next.Step) < 0 {
			if d.Mode == PanRestartOnNoteOnce {
				return PanTick{State: next}, nil
			}
			next.Step = 0
		}
		next.Elapsed = 0
		next.Step++
	}
	var steps []Stereo
	for _, pattern := range patterns {
		if pattern.ID == d.Pattern {
			steps = pattern.Steps
			break
		}
	}
	if next.Step == 0 || int(next.Step) > len(steps) {
		return unchanged, fmt.Errorf("pan state reads beyond shared window")
	}
	stereo := steps[next.Step-1]
	next.Elapsed++
	return PanTick{State: next, Pan: &stereo}, nil
}
