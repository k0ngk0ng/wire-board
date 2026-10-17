package engine

import "fmt"

// OriginalBall19Step carries the writes at bank6 BBB9. ContactLatch remains
// the world's shared 0571 register, not a separate persistent ball flag.
type OriginalBall19Step struct {
	OriginalShotBallStep
	ChangeContactLatch bool
	ContactLatch       uint8
}

// PrepareOriginalBall19 ends at BBD1, before animation and court motion.
// The caller commits entry writes before motion can touch shared world state.
func PrepareOriginalBall19(original Ball, ctx OriginalShotBallContext, launch PassLaunchContext) (OriginalBall19Step, error) {
	initial := OriginalBall19Step{OriginalShotBallStep: OriginalShotBallStep{Ball: original, SpecialShot: ctx.SpecialShot, SelectedHoop: ctx.SelectedHoop, Target: ctx.Target, ArcHeight: ctx.ArcHeight, Landing: ctx.Landing}}
	result := initial
	b := &result.Ball
	if b.Action != 19 {
		return initial, fmt.Errorf("ball19 preparation action %d", b.Action)
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
	result.ChangeContactLatch, result.ContactLatch = true, 128
	b.BouncesRemaining = 6
	result.ReleaseHolder = true
	return result, nil
}

// CompleteOriginalBall19 restores BBD1..BBFC. The latch is read after court
// physics, so a device's writes are visible and entry writes are not replayed.
func CompleteOriginalBall19(prepared OriginalBall19Step, court FlatCourt, loadLatch func() uint8) (OriginalBall19Step, error) {
	if prepared.Ball.Action != 19 || loadLatch == nil {
		return prepared, fmt.Errorf("invalid ball19 completion context")
	}
	result := prepared
	result.SoundIDs = nil
	result.ReleaseHolder, result.ChangeSpecialShot, result.ClearUnownedSpecialShot = false, false, false
	result.ChangeContactLatch = false
	b := &result.Ball
	frames := []AnimationFrame{{Pose: Pose{Frame: 17}, Ticks: 2}, {Pose: Pose{Frame: 18}, Ticks: 2}, {Pose: Pose{Frame: 19}, Ticks: 2}}
	animation, e := AdvanceAnimation(AnimationSpec{Frames: frames, End: RepeatForever}, b.Animation, b.FacingLeft)
	if e != nil {
		return prepared, e
	}
	b.Animation, b.Pose = animation.State, animation.Pose
	if e = advanceBallOnCourt(b, court); e != nil {
		return prepared, e
	}
	if int8(loadLatch()) >= 0 {
		if e = redirectOriginalSpecialDrop(b); e != nil {
			return prepared, e
		}
		// EC59 clears both bytes of vertical speed and gravity after BAAB.
		b.Body.Velocity.Height, b.Body.Gravity = 0, 0
		b.BouncesRemaining--
		if int8(b.BouncesRemaining) < 0 {
			b.Action, b.Entered = 10, false
		}
	}
	// BD14 clamps only the signed high byte; the fractional byte survives.
	if b.Body.Velocity.Height < -8*Pixel {
		b.Body.Velocity.Height = -7*Pixel + (b.Body.Velocity.Height & 255)
	}
	if !b.Airborne {
		b.Action, b.Entered = 6, false
	}
	return result, nil
}
