// Package engine contains independently maintained Yu Yu Hakusho game rules.
// It does not execute a ROM or emulate a CPU.
package engine

import "errors"

// Buttons are game logical controls. Platform profiles translate keyboard,
// touch and physical devices before the engine consumes this bitset.
type Buttons uint16

const (
	ButtonUp Buttons = 1 << iota
	ButtonDown
	ButtonLeft
	ButtonRight
	ButtonB
	ButtonC
	ButtonA
	ButtonStart
	ButtonZ
	ButtonY
	ButtonX
	ButtonMode
)

const allButtons = (1 << 12) - 1

type ControllerDevice uint8

const (
	MD3 ControllerDevice = 3
	MD6 ControllerDevice = 6
)

// ControllerFrame retains both edges; held inputs must not become a fresh
// press on each render or network snapshot.
type ControllerFrame struct {
	Current  Buttons `json:"current"`
	Previous Buttons `json:"previous"`
	Changed  Buttons `json:"changed"`
	Pressed  Buttons `json:"pressed"`
	Released Buttons `json:"released"`
}

// ValidateControllerButtons is the external-input boundary. Only the twelve
// physical controls may be submitted by a platform player.
func ValidateControllerButtons(buttons Buttons) error {
	if buttons&^Buttons(allButtons) != 0 {
		return errors.New("unsupported MD button bits")
	}
	return nil
}

// SampleController recovers the original controller normalization semantics
// at 0x2d5e/0x2dce. Three-button mode clears extension history, matching the
// device transition at 0x2d20. Facing, directional priority, move recognition
// and buffering belong to subsequent fighter rules.
//
// The original six-button routine also carries four non-button storage bits from
// its caller. Preserve them in recovered states instead of masking discrepancies;
// external platform input must pass ValidateControllerButtons before sampling.
func SampleController(previous, held Buttons, device ControllerDevice, swapAC bool) (ControllerFrame, error) {
	if device != MD3 && device != MD6 {
		return ControllerFrame{}, errors.New("unsupported MD controller")
	}
	if device == MD3 {
		previous &= 0xff
		held &= 0xff
	}
	if swapAC {
		a, c := held&ButtonA != 0, held&ButtonC != 0
		held &^= ButtonA | ButtonC
		if a {
			held |= ButtonC
		}
		if c {
			held |= ButtonA
		}
	}
	changed := previous ^ held
	return ControllerFrame{
		Current: held, Previous: previous, Changed: changed,
		Pressed: held & changed, Released: previous & changed,
	}, nil
}
