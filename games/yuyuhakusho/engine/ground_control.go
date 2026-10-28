package engine

// StepGroundControl recovers the full ground-control decision at 0x15cb0,
// including any tail-called constructor. It does not run the shared action
// tail's later attack, animation-record or random-state stages.
func StepGroundControl(body FighterBodyState, pool EffectPool, occupied uint8, controls FighterControls) (FighterBodyState, EffectPool, uint8, [2]int, error) {
	if controls.Pressed&ButtonY != 0 || controls.Held&ButtonC != 0 && controls.Changed&ButtonUp != 0 {
		return BeginLaneSwitch(body, pool, occupied)
	}
	none := [2]int{-1, -1}
	if body.Pose == 0x0d || body.Pose == 0x0e {
		return body, pool, occupied, none, nil
	}
	original := body
	direction := uint8(controls.Held) & 0x0f
	apply := func(transition GroundTransition) error {
		state, err := EnterGroundAction(body.GroundActionState, transition, controls.Held)
		if err == nil {
			body.GroundActionState = state
		}
		return err
	}
	finish := func(transition GroundTransition) (FighterBodyState, EffectPool, uint8, [2]int, error) {
		if err := apply(transition); err != nil {
			return original, pool, occupied, none, err
		}
		return body, pool, occupied, none, nil
	}
	if body.Pose != 1 {
		if body.Pose == 0 && direction&2 != 0 {
			if err := apply(GroundRise); err != nil {
				return original, pool, occupied, none, err
			}
		}
		if body.Pose == 2 && direction&2 == 0 {
			if err := apply(GroundRise); err != nil {
				return original, pool, occupied, none, err
			}
		}
	}
	if body.Pose != 3 && (direction == 8 && body.Facing == 1 || direction == 4 && body.Facing == 0) {
		return finish(GroundTurn)
	}
	if body.Pose != 4 && (direction == 10 && body.Facing == 1 || direction == 6 && body.Facing == 0) {
		return finish(GroundCrouchTurn)
	}
	guard := controls.Held&ButtonC != 0
	if body.Pose != 9 && body.Pose != 10 && guard {
		return finish(GroundGuard)
	}
	if !guard && body.Pose != 1 && body.Pose != 3 && body.Pose != 4 {
		if body.Pose != 0 && direction == 0 {
			return finish(GroundStand)
		}
		if body.Pose != 2 && direction&2 != 0 {
			return finish(GroundCrouch)
		}
		if body.Pose != 5 && (direction == 4 || direction == 8) {
			return finish(GroundWalk)
		}
	}
	if direction&1 != 0 {
		body.GroundActionState = PrepareJump(body.GroundActionState, direction)
	}
	return body, pool, occupied, none, nil
}
