package engine

import "errors"

// Buttons are logical controller buttons. Browser key names and physical
// gamepad layouts are translated by the platform before reaching the game.
type Buttons uint8

const (
	ButtonRight Buttons = 1 << iota
	ButtonLeft
	ButtonDown
	ButtonUp
	ButtonStart
	ButtonSelect
	ButtonB
	ButtonA
)

const directions = ButtonUp | ButtonDown | ButtonLeft | ButtonRight
const actionButtons = ButtonA | ButtonB
const dashTapUpdates uint8 = 8
const chordUpdates uint8 = 2
const verticalTapUpdates uint8 = 10

type ControllerFrame struct {
	Held    Buttons `json:"held"`
	Pressed Buttons `json:"pressed"`
}

type InputMemory struct {
	TapWindow       uint8   `json:"tapWindow"`
	LastTap         Buttons `json:"lastTap"`
	AttackTapWindow uint8   `json:"attackTapWindow"`
	LastAttackTap   Buttons `json:"lastAttackTap"`
	ButtonDelay     uint8   `json:"buttonDelay"`
	PendingButtons  Buttons `json:"pendingButtons"`
}

type FreeInputContext struct {
	Action         Action
	PoseFacingLeft bool
	Airborne       bool
	Request        ActionRequest
}

type InputResult struct {
	Memory  InputMemory   `json:"memory"`
	Request ActionRequest `json:"request"`
	Heading Heading       `json:"heading"`
}

type ControlledPlayer struct {
	Player Player      `json:"player"`
	Input  InputMemory `json:"input"`
}

// StepControlledFreePlayer is the live-input counterpart of the isolated action
// phase. All controller history and movement state are carried forward. A failed
// or unsupported step leaves both unchanged.
func StepControlledFreePlayer(original ControlledPlayer, frame ControllerFrame, profile CharacterProfile, court FlatCourt) (ControlledPlayer, error) {
	result := original
	p := &result.Player
	resolved, err := ResolveFreeInput(FreeInputContext{Action: p.Action, PoseFacingLeft: p.Pose.Mirrored, Airborne: p.Airborne, Request: p.Request}, result.Input, frame)
	if err != nil {
		return original, err
	}
	result.Input = resolved.Memory
	p.Request, p.Heading = resolved.Request, resolved.Heading
	result.Player, err = StepFreePlayer(*p, profile, court)
	if err != nil {
		return original, err
	}
	return result, nil
}

func HeadingForButtons(buttons Buttons) Heading {
	switch buttons & directions {
	case ButtonUp:
		return HeadingUp
	case ButtonUp | ButtonRight:
		return HeadingUpRight
	case ButtonRight:
		return HeadingRight
	case ButtonDown | ButtonRight:
		return HeadingDownRight
	case ButtonDown:
		return HeadingDown
	case ButtonDown | ButtonLeft:
		return HeadingDownLeft
	case ButtonLeft:
		return HeadingLeft
	case ButtonUp | ButtonLeft:
		return HeadingUpLeft
	default:
		return HeadingNone
	}
}

func tickInputWindow(value *uint8) {
	if *value > 0 {
		*value--
	}
}
func requestInput(result *InputResult, action Action) {
	result.Request = ActionRequest{Action: action, Pending: true}
}

// finishDirectionalTap is the original tail used after neutral or running
// direction handling. Unrestored vertical double-tap actions are explicit.
func finishDirectionalTap(result *InputResult, frame ControllerFrame) error {
	m := &result.Memory
	tickInputWindow(&m.TapWindow)
	if m.TapWindow == 0 && m.LastTap != 0 {
		m.LastTap = 0
		return nil
	}
	vertical := frame.Pressed & (ButtonUp | ButtonDown)
	if vertical == 0 {
		return nil
	}
	if result.Heading != HeadingUp && result.Heading != HeadingDown {
		m.TapWindow = 0
		m.LastTap = 0
		return nil
	}
	if m.LastTap == vertical {
		return errors.New("vertical double-tap action has not been reconstructed")
	}
	m.LastTap = vertical
	m.TapWindow = verticalTapUpdates
	return nil
}

func heldDirectionRequest(result *InputResult, frame ControllerFrame) error {
	if frame.Held&directions != 0 {
		requestInput(result, ActionWalk)
		return nil
	}
	requestInput(result, ActionIdle)
	return finishDirectionalTap(result, frame)
}

func resolveDirections(ctx FreeInputContext, result *InputResult, frame ControllerFrame) error {
	m := &result.Memory
	if ctx.Action == ActionRun {
		if frame.Held&directions != 0 && result.Heading != HeadingNone && result.Heading != HeadingUp && result.Heading != HeadingDown {
			left := result.Heading >= HeadingDownLeft
			if left != ctx.PoseFacingLeft {
				requestInput(result, ActionBrake)
				m.TapWindow = 0
				m.LastTap = 0
				return nil
			}
		}
		return finishDirectionalTap(result, frame)
	}
	if ctx.Action == ActionAttackA || ctx.Action == ActionAttackB {
		if m.AttackTapWindow == 0 {
			m.LastAttackTap = 0
		}
		pressed := frame.Pressed & directions
		switch pressed {
		case ButtonUp:
			if m.LastAttackTap == ButtonUp {
				m.AttackTapWindow = 0
				m.LastAttackTap = 0
				return nil
			}
			m.LastAttackTap = ButtonUp
			m.AttackTapWindow = dashTapUpdates
			m.TapWindow = 0
		case ButtonDown:
			m.TapWindow = 0
			return heldDirectionRequest(result, frame)
		case 0:
		default:
			m.AttackTapWindow = 0
		}
		tickInputWindow(&m.AttackTapWindow)
	}
	if m.TapWindow == 0 {
		m.LastTap = 0
	}
	horizontal := frame.Pressed & (ButtonLeft | ButtonRight)
	if horizontal != 0 {
		if horizontal == m.LastTap {
			requestInput(result, ActionRun)
			m.TapWindow = 0
			m.LastTap = 0
			return nil
		}
		m.LastTap = horizontal
		m.TapWindow = dashTapUpdates
	}
	tickInputWindow(&m.TapWindow)
	return heldDirectionRequest(result, frame)
}

// ResolveFreeInput restores the identified no-possession/no-contact input
// path. Contextual grabs, carried players and airborne follow-up attacks still
// require their original handlers; they must not turn into generic attacks.
func ResolveFreeInput(ctx FreeInputContext, memory InputMemory, frame ControllerFrame) (InputResult, error) {
	original := InputResult{Memory: memory, Request: ctx.Request, Heading: HeadingForButtons(frame.Held)}
	result := original
	if !freeAction(ctx.Action) || ctx.Action == ActionFalling {
		return original, errors.New("input action context has not been reconstructed")
	}
	if err := resolveDirections(ctx, &result, frame); err != nil {
		return original, err
	}
	if result.Request.Pending && result.Request.Action == ActionBrake {
		return result, nil
	}
	m := &result.Memory
	m.PendingButtons |= frame.Pressed & actionButtons
	if m.PendingButtons == 0 {
		return result, nil
	}
	if m.ButtonDelay == 0 {
		m.ButtonDelay = chordUpdates
	}
	m.ButtonDelay--
	if m.ButtonDelay != 0 {
		return result, nil
	}
	if ctx.Airborne {
		return original, errors.New("airborne follow-up input has not been reconstructed")
	}
	switch m.PendingButtons {
	case ButtonA:
		if ctx.Action == ActionRun {
			return original, errors.New("running A attack has not been reconstructed")
		}
		requestInput(&result, ActionAttackA)
	case ButtonB:
		if ctx.Action == ActionRun {
			return original, errors.New("running B attack has not been reconstructed")
		}
		requestInput(&result, ActionAttackB)
	case ButtonA | ButtonB:
		if ctx.Action == ActionRun {
			requestInput(&result, ActionRunningJump)
		} else {
			requestInput(&result, ActionJump)
		}
	}
	m.PendingButtons = 0
	return result, nil
}
