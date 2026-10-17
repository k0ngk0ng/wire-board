package engine

import "fmt"

// PrepareOriginalBall22 restores BD23..BD4F. This kick-like special shot uses
// a fixed impulse rather than the common targeted launch. Existing gravity,
// depth velocity and shared target coordinates are preserved.
func PrepareOriginalBall22(original Ball, ctx OriginalShotBallContext) (OriginalShotBallStep, error) {
	initial := OriginalShotBallStep{Ball: original, SpecialShot: ctx.SpecialShot, SelectedHoop: ctx.SelectedHoop, Target: ctx.Target, ArcHeight: ctx.ArcHeight, Landing: ctx.Landing}
	result := initial
	b := &result.Ball
	if b.Action != 22 {
		return initial, fmt.Errorf("ball22 preparation action %d", b.Action)
	}
	if b.Entered {
		return result, nil
	}
	b.Entered = true
	b.Animation.Cursor, b.Animation.Elapsed, b.Animation.Loops = 4, 0, 0
	b.Body.Velocity.Height = -1472
	b.Body.Velocity.X = 8 * Pixel
	if b.FacingLeft {
		b.Body.Velocity.X = -8 * Pixel
	}
	result.SelectedHoop = selectOriginalBallHoop(*b)
	result.ReleaseHolder = true
	result.SoundIDs = []uint8{30}
	return result, nil
}

func animateOriginalBall22(b *Ball) error {
	state := b.Animation
	tail := state.Cursor >= 4
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 32}, Ticks: 1}, {Pose: Pose{Frame: 30}, Ticks: 1}, {Pose: Pose{Frame: 31}, Ticks: 1}}, End: RepeatUntilCount, RepeatLimit: 255}
	if tail {
		state.Cursor -= 4
		spec = AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 32}, Ticks: 255}}, End: RepeatForever}
	}
	out, e := AdvanceAnimation(spec, state, b.FacingLeft)
	if e != nil {
		return e
	}
	if tail && (out.State.Cursor != 0 || out.State.Elapsed != 0) {
		out.State.Cursor += 4
	}
	b.Animation, b.Pose = out.State, out.Pose
	if out.ChangeAction {
		b.Action, b.Entered = BallAction(out.Action), false
	}
	return nil
}

// CompleteOriginalBall22 commits no preparation commands twice. The hoop is
// read after motion because court devices may update the shared selection.
func CompleteOriginalBall22(prepared OriginalShotBallStep, court FlatCourt, loadHoop func() int8) (OriginalShotBallStep, error) {
	if prepared.Ball.Action != 22 || loadHoop == nil {
		return prepared, fmt.Errorf("invalid ball22 completion context")
	}
	result := prepared
	result.SoundIDs = nil
	result.ReleaseHolder, result.ChangeSpecialShot, result.ClearUnownedSpecialShot = false, false, false
	b := &result.Ball
	if e := animateOriginalBall22(b); e != nil {
		return prepared, e
	}
	if e := advanceBallOnCourt(b, court); e != nil {
		return prepared, e
	}
	if b.Animation.Cursor >= 4 {
		if !b.Airborne {
			// Original writes a literal pose, without adding facing/mirroring.
			b.Pose = Pose{Frame: 31}
			b.Body.Velocity.Height = 6*Pixel + (b.Body.Velocity.Height & 255)
		}
		if b.Action != 22 {
			return prepared, fmt.Errorf("special ball22 post-motion turn selector %d not reconstructed", b.Action)
		}
		if specialBallPassedTurnLine(b, loadHoop(), 16) {
			b.Action, b.Entered = 12, false
			if b.Body.Position.Height >= 88*Pixel {
				b.Action, b.Entered = 22, true
				b.Animation.Cursor, b.Animation.Elapsed = 0, 0
				if e := redirectOriginalSpecialDrop(b); e != nil {
					return prepared, e
				}
			} else {
				b.Body.Velocity.Height = 6*Pixel + (b.Body.Velocity.Height & 255)
			}
		}
	}
	if b.Animation.Cursor < 4 && !b.Airborne {
		b.Action, b.Entered = 6, false
	}
	return result, nil
}
