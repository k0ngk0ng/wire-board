package engine

import "fmt"

// PrepareOriginalBall17 starts the spinning special shot. Its launch is an
// additive vertical impulse and a fixed horizontal speed, not a pass arc.
func PrepareOriginalBall17(original Ball, ctx OriginalShotBallContext) (OriginalShotBallStep, error) {
	initial := OriginalShotBallStep{Ball: original, SpecialShot: ctx.SpecialShot, SelectedHoop: ctx.SelectedHoop, Target: ctx.Target, ArcHeight: ctx.ArcHeight, Landing: ctx.Landing}
	if original.Action != 17 {
		return initial, fmt.Errorf("ball17 preparation action %d", original.Action)
	}
	result := initial
	b := &result.Ball
	if b.Entered {
		return result, nil
	}
	b.Entered = true
	b.Animation.Cursor, b.Animation.Elapsed = 0, 0
	b.Animation.Loops = 12
	b.Animation.NextAction, b.Animation.HasNextAction = 12, true
	result.SoundIDs = []uint8{30}
	result.SelectedHoop = selectOriginalBallHoop(*b)
	b.Body.Velocity.Height = signed16(b.Body.Velocity.Height + 2272)
	b.Body.Gravity = signed16(b.Body.Gravity + 288)
	b.Body.Velocity.X = 6 * Pixel
	if b.FacingLeft {
		b.Body.Velocity.X = -6 * Pixel
	}
	result.ReleaseHolder = true
	return result, nil
}

// CompleteOriginalBall17 preserves the source order: animation can select
// action12 before court motion, and landing stops only horizontal/depth motion.
func CompleteOriginalBall17(original Ball, court FlatCourt) (Ball, error) {
	if original.Action != 17 {
		return original, fmt.Errorf("ball17 completion action %d", original.Action)
	}
	b := original
	animation, e := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{
		{Pose: Pose{Frame: 14, Mirrored: true}, Ticks: 1},
		{Pose: Pose{Frame: 13}, Ticks: 1},
		{Pose: Pose{Frame: 14}, Ticks: 1},
		{Pose: Pose{Frame: 13, Mirrored: true}, Ticks: 1},
	}, End: RepeatRemaining}, b.Animation, b.FacingLeft)
	if e != nil {
		return original, e
	}
	b.Animation, b.Pose = animation.State, animation.Pose
	if animation.ChangeAction {
		b.Action, b.Entered = BallAction(animation.Action), false
	}
	if e = advanceBallOnCourt(&b, court); e != nil {
		return original, e
	}
	if !b.Airborne {
		b.Body.Velocity.X, b.Body.Velocity.Depth = 0, 0
	}
	return b, nil
}
