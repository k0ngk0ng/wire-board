package engine

import "errors"

type CarrierInputContext struct {
	Player  FreeInputContext
	HasBall bool
}

// ResolveCarrierInput restores ordinary held-ball controls and follow-through
// after release. The shared directional windows are the same as free movement;
// A/B semantics and possession grouping belong to basketball's action state.
func ResolveCarrierInput(ctx CarrierInputContext, memory InputMemory, frame ControllerFrame) (InputResult, error) {
	original := InputResult{Memory: memory, Request: ctx.Player.Request, Heading: HeadingForButtons(frame.Held)}
	result := original
	if ctx.Player.Action < ActionDribbleIdle || ctx.Player.Action > ActionAirShoot && ctx.Player.Action != ActionCatchBall && ctx.Player.Action != ActionReceiveBall && ctx.Player.Action != ActionCatchAir && ctx.Player.Action != ActionPickUp {
		return original, errors.New("carrier input context has not been reconstructed")
	}
	directionalContext := ctx.Player
	directionalContext.Action = ctx.Player.Action & 15
	if err := resolveDirections(directionalContext, &result, frame); err != nil {
		return original, err
	}
	if result.Request.Pending && result.Request.Action == ActionBrake {
		if ctx.HasBall {
			result.Request.Action = ActionDribbleBrake
		}
		return result, nil
	}
	m := &result.Memory
	m.PendingButtons |= frame.Pressed & actionButtons
	processed := false
	if m.PendingButtons != 0 {
		if m.ButtonDelay == 0 {
			m.ButtonDelay = chordUpdates
		}
		m.ButtonDelay--
		if m.ButtonDelay == 0 {
			processed = true
			var action Action
			switch m.PendingButtons {
			case ButtonA, ButtonB:
				if !ctx.HasBall {
					return original, errors.New("follow-through contextual attack has not been reconstructed")
				}
				action = ActionPass - 16
				if m.PendingButtons == ButtonB {
					action = ActionAimShot - 16
				}
				if ctx.Player.Airborne {
					action++
				}
			case ButtonA | ButtonB:
				if ctx.Player.Airborne {
					return original, errors.New("carrier airborne jump boost has not been reconstructed")
				}
				action = ActionJump
				if ctx.Player.Action == ActionDribbleRun {
					action = ActionRunningJump
				}
			}
			requestInput(&result, action)
			m.PendingButtons = 0
		}
	}
	if !processed && (ctx.Player.Action == ActionAimShot || ctx.Player.Action == ActionAimAirShot) && frame.Held&ButtonB == 0 {
		action := ActionShoot - 16
		if ctx.Player.Airborne {
			action++
		}
		requestInput(&result, action)
	}
	if result.Request.Pending && result.Request.Action < 16 && ctx.HasBall {
		result.Request.Action += 16
	}
	return result, nil
}
