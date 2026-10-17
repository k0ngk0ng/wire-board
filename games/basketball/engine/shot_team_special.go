package engine

import "fmt"

// OriginalTeamSpecialBallStep carries only original writes. 0511 and 0564
// remain Ball.BouncesRemaining and Ball.Deflection even when action15 uses
// them as a countdown and saved facing; no second persistent truth is added.
type OriginalTeamSpecialBallStep struct {
	OriginalShotBallStep
	ClearExcludedCatcher bool
	ChangeCameraShake    bool
	CameraShake          uint8
}

// PrepareOriginalTeamSpecialBall ends before the action's animation/EEFF.
// Commit ReleaseHolder and exclusion/flag writes before a bound court loads
// world state. Its SoundIDs are requests preceding motion.
func PrepareOriginalTeamSpecialBall(original Ball, ctx OriginalShotBallContext, launchContext ...PassLaunchContext) (OriginalTeamSpecialBallStep, error) {
	initial := OriginalTeamSpecialBallStep{OriginalShotBallStep: OriginalShotBallStep{Ball: original, SpecialShot: ctx.SpecialShot, SelectedHoop: ctx.SelectedHoop, Target: ctx.Target, ArcHeight: ctx.ArcHeight, Landing: ctx.Landing}}
	result := initial
	var e error
	result.Ball, e = ResolveBallRequest(original)
	if e != nil {
		return initial, e
	}
	b := &result.Ball
	if b.Action != 12 && b.Action != 14 && b.Action != 15 {
		return initial, fmt.Errorf("team-special ball action %d outside restored handlers", b.Action)
	}
	if b.Entered {
		return result, nil
	}
	b.Entered = true
	b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
	result.SelectedHoop = selectOriginalBallHoop(*b)
	if b.Action == 14 {
		// BACE: the source impulse EA4C adds to existing vertical motion.
		result.SoundIDs = []uint8{30}
		b.Body.Velocity.Height = signed16(b.Body.Velocity.Height + 1664)
		b.Body.Gravity = signed16(b.Body.Gravity + 192)
		b.Body.Velocity.X = 2 * Pixel
		if b.FacingLeft {
			b.Body.Velocity.X = -2 * Pixel
		}
	} else {
		plan, clear, e := originalShotLaunchPlan(*b, ctx)
		if e != nil {
			return initial, e
		}
		if clear {
			if ctx.Holder == 6 {
				result.ClearUnownedSpecialShot = true
			} else {
				result.ChangeSpecialShot = true
				result.SpecialShot = 0
			}
		}
		result.Target, result.ArcHeight = plan.Target, plan.ArcHeight
		body, e := LaunchPass(b.Body.Position, plan.Target, plan.ArcHeight, launchContext...)
		if e != nil {
			return initial, e
		}
		b.Body = body
		if b.Action == 12 {
			result.Landing = Position{X: plan.Target.X, Depth: plan.Target.Depth}
		} else {
			result.SoundIDs = []uint8{30}
			b.Deflection = 0
			if b.FacingLeft {
				b.Deflection = 128
			}
			b.FacingLeft = true
			b.BouncesRemaining = 13
			result.ClearExcludedCatcher = true
		}
	}
	result.ReleaseHolder = true
	return result, nil
}

func animateTeamSpecialBall(b *Ball, action BallAction) error {
	if action == 12 {
		return advanceBallSpin(b)
	}
	if action == 14 {
		spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 6}, Ticks: 1}, {Pose: Pose{Frame: 7}, Ticks: 1}}, End: RepeatForever}
		r, e := AdvanceAnimation(spec, b.Animation, b.FacingLeft)
		if e != nil {
			return e
		}
		b.Animation, b.Pose = r.State, r.Pose
		return nil
	}
	// BEC6 starts with one long-lived pose; cursor2 starts the drop stage.
	state := b.Animation
	tail := state.Cursor >= 2
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 0}, Ticks: 255}}, End: RepeatForever}
	if tail {
		state.Cursor -= 2
		spec = AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 8}, Ticks: 4}, {Pose: Pose{Frame: 9}, Ticks: 5}, {Pose: Pose{Frame: 10}, Ticks: 255}}, End: RepeatRemaining}
	}
	r, e := AdvanceAnimation(spec, state, b.FacingLeft)
	if e != nil {
		return e
	}
	if tail && (r.State.Cursor != 0 || r.State.Elapsed != 0) {
		r.State.Cursor += 2
	}
	b.Animation, b.Pose = r.State, r.Pose
	if r.ChangeAction {
		b.Action, b.Entered = BallAction(r.Action), false
	}
	return nil
}

// CompleteOriginalTeamSpecialBall emits only sounds following motion. It
// leaves hoop/player contact phases to their original world stages.
func CompleteOriginalTeamSpecialBall(prepared OriginalTeamSpecialBallStep, court FlatCourt) (OriginalTeamSpecialBallStep, error) {
	result := prepared
	result.SoundIDs = nil
	// Preparation commands have already been committed by the world. Do not
	// repeat them after court motion, which can produce its own state writes.
	result.ReleaseHolder, result.ClearExcludedCatcher = false, false
	result.ChangeSpecialShot, result.ClearUnownedSpecialShot = false, false
	b := &result.Ball
	action := b.Action
	if action != 12 && action != 14 && action != 15 {
		return prepared, fmt.Errorf("team-special completion action %d outside restored handlers", action)
	}
	if e := animateTeamSpecialBall(b, action); e != nil {
		return prepared, e
	}
	if e := advanceBallOnCourt(b, court); e != nil {
		return prepared, e
	}
	switch action {
	case 12:
		if !b.Airborne {
			b.Action, b.Entered = BallBounce, false
		}
	case 14:
		if b.Action != 14 {
			// BDD2 uses the current action after EEFF. A device replacement
			// with action4 selects pointer $00FF, not action14's fixed turn
			// offsets. The audio work byte and graphics-upload prefix needed
			// by that lookup are not represented in this rule context.
			return prepared, fmt.Errorf("special ball14 post-motion turn selector %d not reconstructed", b.Action)
		}
		if !b.Airborne {
			b.Body.Velocity.X = 8 * Pixel
			if b.FacingLeft {
				b.Body.Velocity.X = -8 * Pixel
			}
		}
		// BDD2 selects BDBE for action14: offsets -32 / +32 from hoop X.
		if specialBallPassedTurnLine(b, result.SelectedHoop, 32) {
			b.Action, b.Entered = 12, false
		}
	case 15:
		b.Pose.Mirrored = b.Pose.Mirrored || b.FacingLeft
		if b.Animation.Cursor < 2 {
			b.BouncesRemaining--
			if int8(b.BouncesRemaining) < 0 {
				b.Body.Velocity = Velocity{}
				b.Body.Gravity = Pixel / 2
				b.Animation.Cursor, b.Animation.Elapsed = 2, 0
			}
		}
		if !b.Airborne {
			result.ChangeCameraShake, result.CameraShake = true, 11
			b.FacingLeft = uint8(b.Deflection)&128 != 0
			result.SoundIDs = []uint8{10}
			b.Action, b.Entered = 12, false
		}
	}
	return result, nil
}
