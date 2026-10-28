package engine

import "errors"

// StepGroundIdle recovers action 0 at 0x19cee, including the constructor selected
// when an animation completes. The shared action tail still follows this stage.
func StepGroundIdle(body FighterBodyState, pool EffectPool, parameters JumpParameters, held Buttons) (FighterBodyState, EffectPool, [2]int, error) {
	before := body
	none := [2]int{-1, -1}
	if body.ActionKind != 0 {
		return body, pool, none, errors.New("unsupported ground-idle action")
	}
	body.Y = body.Ground
	if body.Pose == 0x29 && body.Health < 0 || body.Clock.Completed != 1 {
		return body, pool, none, nil
	}
	body.Clock.Completed = 0
	transition := GroundStand
	switch body.Pose {
	case 1:
		if held&ButtonDown != 0 {
			transition = GroundCrouch
		}
	case 2, 4, 0x0e:
		transition = GroundCrouch
	case 0x0d:
		state, next, slots, err := BeginJump(body, pool, parameters)
		if err != nil {
			return before, pool, none, err
		}
		return state, next, slots, nil
	case 0x29:
		body.ActionKind, body.Pose, body.EnabledStages = 0, 0x2a, 0
		body.GroundActionState = ResetActionState(body.GroundActionState)
		return body, pool, none, nil
	case 0x2a:
		body.TransitionDelay = 8
		if body.Auxiliary7E < 0 {
			body.ActionKind, body.Pose, body.EnabledStages = 12, 0x2b, 0
			body.Auxiliary7E = body.Auxiliary80
			body.GroundActionState = ResetActionState(body.GroundActionState)
		} else {
			body.EnabledStages = 0x0b
		}
		return body, pool, none, nil
	}
	state, err := EnterGroundAction(body.GroundActionState, transition, held)
	if err != nil {
		return before, pool, none, err
	}
	body.GroundActionState = state
	return body, pool, none, nil
}
