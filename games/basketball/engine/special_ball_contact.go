package engine

import "fmt"

// applyOriginalSpecialBallContact is bank5 $85EE followed by the $84EC hit
// branch. The target's own facing selects the impulse: X and Y both refer to
// the candidate when the original shared knockback routine is called.
func applyOriginalSpecialBallContact(result *InteractionStep, ctx InteractionContext) (bool, error) {
	candidate := ctx.Candidates[6]
	s := &result.State
	if candidate < 0 || candidate >= 4 || s.Holder != 6 || s.Ball.Action < 13 || s.Ball.Action == 21 {
		return false, nil
	}
	actor := &s.Players[candidate]
	p := &actor.Player
	b := &s.Ball
	if b.Action == 17 {
		b.Animation.Loops = 0
	} else if p.Action >= 0x60 && (actor.AttackVariant == 3 || actor.AttackVariant == 71 || actor.AttackVariant == 62) && p.FacingLeft != (b.Body.Velocity.X < 0) && (b.Action == 13 || b.Action == 22 || b.Action == 23) {
		// $862A swaps the selected pair with byte wrapping; $8638 complements
		// only the velocity integer byte, preserving its fractional remainder.
		selected := uint8(s.SelectedHoop)
		s.SelectedHoop = int8(((selected - 1) ^ 1) + 1)
		integer := ^uint8(uint16(b.Body.Velocity.X) >> 8)
		b.Body.Velocity.X = Fixed(int8(integer))*Pixel + (b.Body.Velocity.X & 255)
		b.FacingLeft = !b.FacingLeft
		return true, nil
	}
	if int(ctx.CourtID) >= len(originalCombatDepthPush) {
		return true, fmt.Errorf("special-ball knockback court %d not reconstructed", ctx.CourtID)
	}
	p.Action, p.Entered = 0x49, false
	p.Body.Gravity = Pixel
	p.Body.Velocity = Velocity{X: 4 * Pixel, Height: 8 * Pixel}
	if p.FacingLeft {
		p.Body.Velocity.X = -4 * Pixel
	}
	if originalCombatDepthPush[ctx.CourtID] && uint8(p.Body.Position.Depth>>8) >= 72 {
		p.Body.Velocity.Depth = -Pixel
	}
	// This path never writes the endurance reserve, possession or last touch.
	result.Sounds = append(result.Sounds, CombatSound{Source: 6, Target: uint8(candidate), ID: 21})
	return true, nil
}

// StepOriginalBallContact is the original $84C3..$801D tail after player
// combat. Special responses precede ordinary catch eligibility checks.
func StepOriginalBallContact(original InteractionState, ctx InteractionContext) (InteractionStep, error) {
	initial := InteractionStep{State: original, Catch: CatchResolution{Holder: original.Holder, LastTouch: original.LastTouch, Actor: -1}}
	result := initial
	// $84C8 dispatches auxiliary candidates before ball ownership/action
	// eligibility. Keep its original ignored-contact and reaction branches.
	if candidate := ctx.Candidates[6]; candidate >= 4 {
		if err := applyOriginalBallPropContact(&result, candidate); err != nil {
			return initial, err
		}
		return result, nil
	}
	handled, err := applyOriginalSpecialBallContact(&result, ctx)
	if err != nil {
		return initial, err
	}
	if handled {
		return result, nil
	}
	catchContext := AutomaticCatchContext{Candidate: ctx.Candidates[6], Holder: result.State.Holder, LastTouch: result.State.LastTouch, PreviousHolder: ctx.PreviousHolder, ExcludedCatcher: ctx.ExcludedCatcher, BallAction: result.State.Ball.Action, AuxiliaryOwners: ctx.AuxiliaryOwners, HoopOwners: ctx.HoopOwners}
	for i, actor := range result.State.Players {
		player := actor.Player
		catchContext.Players[i] = CatchPlayer{Action: player.Action, Control: ctx.Controls[i], Flags: originalPlayerCatchFlags(player.Action), Attached: ctx.Attachments[i]&128 != 0, Airborne: player.Airborne}
	}
	resolution, err := ResolveAutomaticCatch(catchContext)
	if err != nil {
		return initial, err
	}
	result.Catch = resolution
	result.State.Holder, result.State.LastTouch = resolution.Holder, resolution.LastTouch
	if resolution.ChangePlayer {
		player := &result.State.Players[resolution.Actor].Player
		player.Action, player.Entered = resolution.PlayerAction, false
	}
	result.State.Ball = ApplyBallCommand(result.State.Ball, resolution.Ball)
	return result, nil
}
