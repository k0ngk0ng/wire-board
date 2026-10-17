package engine

import "fmt"

// PrepareOriginalBall16 restores BB78..BB8D. Commit its owner/target commands
// before court motion so device handlers observe the released ball.
func PrepareOriginalBall16(original Ball, ctx OriginalShotBallContext, launch PassLaunchContext) (OriginalShotBallStep, error) {
	initial := OriginalShotBallStep{Ball: original, SpecialShot: ctx.SpecialShot, SelectedHoop: ctx.SelectedHoop, Target: ctx.Target, ArcHeight: ctx.ArcHeight, Landing: ctx.Landing}
	result := initial
	b := &result.Ball
	if b.Action != 16 {
		return initial, fmt.Errorf("ball16 preparation action %d", b.Action)
	}
	if b.Entered {
		return result, nil
	}
	b.Entered = true
	b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 0, 0, 0
	result.SoundIDs = []uint8{30}
	result.SelectedHoop = selectOriginalBallHoop(*b)
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
	b.Body, e = LaunchPass(b.Body.Position, plan.Target, plan.ArcHeight, launch)
	if e != nil {
		return initial, e
	}
	// EC59 clears both bytes of vertical velocity and gravity; BB86 then
	// assigns only the velocity integer byte, yielding exactly four pixels.
	b.Body.Velocity.Height, b.Body.Gravity = 4*Pixel, 0
	result.ReleaseHolder = true
	return result, nil
}

// CompleteOriginalBall16 restores BB8E..BBB8. loadSelectedHoop reads the current
// world after court motion; entry commands are not replayed over device writes.
func CompleteOriginalBall16(prepared OriginalShotBallStep, court FlatCourt, loadSelectedHoop func() int8) (OriginalShotBallStep, error) {
	if prepared.Ball.Action != 16 || loadSelectedHoop == nil {
		return prepared, fmt.Errorf("invalid ball16 completion context")
	}
	result := prepared
	result.SoundIDs = nil
	result.ReleaseHolder, result.ChangeSpecialShot, result.ClearUnownedSpecialShot = false, false, false
	b := &result.Ball
	animation, e := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 11}, Ticks: 1}, {Pose: Pose{Frame: 12}, Ticks: 1}}, End: RepeatForever}, b.Animation, b.FacingLeft)
	if e != nil {
		return prepared, e
	}
	b.Animation, b.Pose = animation.State, animation.Pose
	if e = advanceBallOnCourt(b, court); e != nil {
		return prepared, e
	}
	if b.Body.Velocity.Height >= 0 {
		if b.Action != 16 {
			return prepared, fmt.Errorf("ball16 post-motion turn selector %d not reconstructed", b.Action)
		}
		if specialBallPassedTurnLine(b, loadSelectedHoop(), 16) {
			b.Action, b.Entered = 16, true
			if e = redirectOriginalSpecialDrop(b); e != nil {
				return prepared, e
			}
		}
	}
	clampOriginalSpecialDescent(b)
	if !b.Airborne {
		b.Action, b.Entered = BallBounce, false
	}
	return result, nil
}
